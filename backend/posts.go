package main

// ============================================================
// posts.go —— 文章模块
// 职责：文章的增删改查（CRUD）、分页列表、点赞。
//
// 接口分为两组：
//   公开接口（/api/posts...）：任何人都能调用，只能看到「已发布」的文章
//   管理接口（/api/admin/posts...）：需要登录（JWT），能看到包括草稿在内的全部文章
//
// 安全要点：所有 SQL 都用 ? 占位符传参（预编译），杜绝 SQL 注入。
// ============================================================

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// postListFields 列表查询复用的列名（不含 content 正文，减小传输量）
const postListFields = `id, title, summary, cover, status, likes, created_at, updated_at`

// parsePage 从查询参数解析分页，带默认值与上限保护。
// 例：?page=2&pageSize=5 → page=2, pageSize=5
// 非法输入（负数、超限）会被纠正为默认值，防止恶意参数。
func parsePage(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10 // 单页最多 50 条，防止一次拉太多
	}
	return
}

// parseID 解析路径参数里的文章 ID（如 /api/posts/3 中的 3）。
// 返回 (id, true) 表示合法；(_, false) 表示非法（已直接返回 400 响应）。
func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的文章 ID"})
		return 0, false
	}
	return id, true
}

// trimSpace 去掉字符串首尾空白（用于标题等输入清洗）
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

// scanListRows 把查询结果集的每一行扫描成 PostListItem。
// 多个列表接口共用，避免重复代码。
func scanListRows(rows *sql.Rows) ([]PostListItem, error) {
	items := []PostListItem{}
	for rows.Next() {
		var p PostListItem
		if err := rows.Scan(&p.ID, &p.Title, &p.Summary, &p.Cover, &p.Status, &p.Likes, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// ------------------------------------------------------------
// 公开接口（前台浏览用）
// ------------------------------------------------------------

// handleListPosts 公开：已发布文章分页列表。GET /api/posts
// 查询参数：page（页码，默认1）、pageSize（每页条数，默认10）
// 只返回 status='published' 的文章，按创建时间倒序（最新在前）。
func handleListPosts(c *gin.Context) {
	page, pageSize := parsePage(c)

	// ① 先查总数（用于前端算总页数）
	var total int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM posts WHERE status = 'published'`).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	// ② 查当前页数据（LIMIT 条数，OFFSET 跳过的行数）
	rows, err := db.Query(
		`SELECT `+postListFields+` FROM posts WHERE status = 'published' ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		pageSize, (page-1)*pageSize,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()
	items, err := scanListRows(rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, PageResult{Items: items, Total: total, Page: page, PageSize: pageSize})
}

// handleGetPost 公开：文章详情。GET /api/posts/:id
// 注意：草稿（draft）对外不可见 —— 条件里带 status='published'，
// 所以访问草稿会返回 404（对外表现为「不存在」，不泄露草稿内容）。
func handleGetPost(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var p Post
	err := db.QueryRow(
		`SELECT id, title, summary, cover, content, status, likes, created_at, updated_at
		 FROM posts WHERE id = ? AND status = 'published'`, id,
	).Scan(&p.ID, &p.Title, &p.Summary, &p.Cover, &p.Content, &p.Status, &p.Likes, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// handleLikePost 公开：点赞。POST /api/posts/:id/like
// 实现：UPDATE posts SET likes = likes + 1（数据库原子自增，并发安全）。
// 前端负责「同一浏览器只能点一次」（localStorage 记录），后端不做防刷。
func handleLikePost(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	res, err := db.Exec(`UPDATE posts SET likes = likes + 1 WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "点赞失败"})
		return
	}
	// RowsAffected 为 0 说明 id 不存在（WHERE 没匹配到行）
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	// 返回最新点赞数，前端直接更新显示
	var likes int
	if err := db.QueryRow(`SELECT likes FROM posts WHERE id = ?`, id).Scan(&likes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "点赞失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"likes": likes})
}

// ------------------------------------------------------------
// 管理接口（后台用，需登录）
// ------------------------------------------------------------

// handleAdminListPosts 管理端：全部文章（含草稿）。GET /api/admin/posts
// 与公开列表的区别：不加 status 过滤，按更新时间倒序（最近改的排前面）。
func handleAdminListPosts(c *gin.Context) {
	page, pageSize := parsePage(c)

	var total int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	rows, err := db.Query(
		`SELECT `+postListFields+` FROM posts ORDER BY updated_at DESC LIMIT ? OFFSET ?`,
		pageSize, (page-1)*pageSize,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()
	items, err := scanListRows(rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, PageResult{Items: items, Total: total, Page: page, PageSize: pageSize})
}

// handleAdminGetPost 管理端：文章详情（含草稿）。GET /api/admin/posts/:id
// 与公开详情的唯一区别：不加 status 过滤，编辑草稿时用。
func handleAdminGetPost(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var p Post
	err := db.QueryRow(
		`SELECT id, title, summary, cover, content, status, likes, created_at, updated_at
		 FROM posts WHERE id = ?`, id,
	).Scan(&p.ID, &p.Title, &p.Summary, &p.Cover, &p.Content, &p.Status, &p.Likes, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// bindPostInput 解析并校验「创建文章」的请求体。
// 规则：JSON 必须合法；标题去空白后不能为空；状态只能是 draft/published（非法则归为 draft）。
func bindPostInput(c *gin.Context) (PostInput, bool) {
	var in PostInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return in, false
	}
	in.Title = trimSpace(in.Title)
	if in.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题不能为空"})
		return in, false
	}
	if in.Status != "draft" && in.Status != "published" {
		in.Status = "draft"
	}
	return in, true
}

// handleCreatePost 管理端：新建文章。POST /api/admin/posts
// 请求体：{title, summary, cover, content, status}
// 返回：创建成功后的完整文章（含自增 ID、时间）。
func handleCreatePost(c *gin.Context) {
	in, ok := bindPostInput(c)
	if !ok {
		return
	}
	res, err := db.Exec(
		`INSERT INTO posts (title, summary, cover, content, status) VALUES (?, ?, ?, ?, ?)`,
		in.Title, in.Summary, in.Cover, in.Content, in.Status,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	// LastInsertId 拿到自增主键，再查一次返回完整数据
	id, _ := res.LastInsertId()
	var p Post
	if err := db.QueryRow(
		`SELECT id, title, summary, cover, content, status, likes, created_at, updated_at FROM posts WHERE id = ?`, id,
	).Scan(&p.ID, &p.Title, &p.Summary, &p.Cover, &p.Content, &p.Status, &p.Likes, &p.CreatedAt, &p.UpdatedAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// handleUpdatePost 管理端：更新文章。PUT /api/admin/posts/:id
// 特色：支持「部分字段更新」—— 请求体里哪个字段非空就更新哪个，
// 传空字符串表示「保持原值」。这样后台「一键发布/下架」只需传 {"status": "published"}。
// 无论改了什么，updated_at 都会刷新为当前时间（NOW()）。
func handleUpdatePost(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in PostInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	// 状态字段如果传了，必须是合法值
	if in.Status != "" && in.Status != "draft" && in.Status != "published" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的状态值"})
		return
	}

	// 动态拼 SET 子句：只更新非空字段（列名是代码写死的，不存在注入风险）
	sets := []string{}
	args := []any{}
	if in.Title != "" {
		sets = append(sets, "title = ?")
		args = append(args, in.Title)
	}
	if in.Summary != "" {
		sets = append(sets, "summary = ?")
		args = append(args, in.Summary)
	}
	if in.Cover != "" {
		sets = append(sets, "cover = ?")
		args = append(args, in.Cover)
	}
	if in.Content != "" {
		sets = append(sets, "content = ?")
		args = append(args, in.Content)
	}
	if in.Status != "" {
		sets = append(sets, "status = ?")
		args = append(args, in.Status)
	}
	if len(sets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有需要更新的内容"})
		return
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, id)

	query := `UPDATE posts SET ` + strings.Join(sets, ", ") + ` WHERE id = ?`
	res, err := db.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id})
}

// handleDeletePost 管理端：删除文章。DELETE /api/admin/posts/:id
// 物理删除（不留回收站），前端已加二次确认。
func handleDeletePost(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	res, err := db.Exec(`DELETE FROM posts WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
