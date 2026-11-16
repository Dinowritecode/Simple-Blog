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
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// visitorCookie 点赞去重用的访客 Cookie 名（匿名、无需登录）。
const visitorCookie = "blog_visitor"

// visitorID 取当前访客标识：Cookie 里有就用，没有就生成一个新的并写回。
// 这样「同一浏览器」的点赞能被稳定识别，实现每篇文章只能赞一次。
func visitorID(c *gin.Context) string {
	id, err := c.Cookie(visitorCookie)
	if err == nil && id != "" {
		return id
	}
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		// 极端情况下随机失败：退回纳秒时间戳方案（几乎不可能发生）
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	id = hex.EncodeToString(buf)
	c.SetCookie(visitorCookie, id, 365*24*3600, "/", "", false, true)
	return id
}

// hasLiked 当前访客是否已赞该文章。
func hasLiked(postID int64, visitor string) bool {
	var n int
	err := q.QueryRow(`SELECT COUNT(*) FROM post_likes WHERE post_id = ? AND visitor = ?`, postID, visitor).Scan(&n)
	return err == nil && n > 0
}

// postListFields 列表查询复用的列名（不含 content 正文，减小传输量）。
// comments 用子查询实时统计（含回复），这样列表里就能直接显示评论数。
const postListFields = `id, slug, title, summary, cover, category, tags, status, likes, views, (SELECT COUNT(*) FROM comments c WHERE c.post_id = posts.id) AS comments, created_at, updated_at`

// postDetailFields 详情查询的完整列名（含正文与公开短码）。
// PostgreSQL 不支持 LastInsertId，插入后统一用 RETURNING id 再查一次，
// 所以详情查询也直接复用这组列名。
const postDetailFields = `id, slug, title, summary, cover, category, tags, content, status, likes, views, (SELECT COUNT(*) FROM comments c WHERE c.post_id = posts.id) AS comments, created_at, updated_at`

// scanPost 扫描一行完整 Post（含 slug）。
// content 列用 sql.NullString 承接：为 NULL 表示正文已迁到文件（见 postcontent.go）。
func scanPost(row interface{ Scan(...any) error }, p *Post) error {
	var content sql.NullString
	if err := row.Scan(&p.ID, &p.Slug, &p.Title, &p.Summary, &p.Cover, &p.Category, &p.Tags,
		&content, &p.Status, &p.Likes, &p.Views, &p.Comments, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return err
	}
	p.Content = scanNullContent(content)
	return nil
}

// loadDetail 按 where 子句查一篇文章的完整信息，并把正文从「数据库或文件」补齐。
func loadDetail(where string, args ...any) (*Post, error) {
	var p Post
	if err := scanPost(q.QueryRow(
		`SELECT `+postDetailFields+` FROM posts WHERE `+where, args...), &p); err != nil {
		return nil, err
	}
	// 正文已迁到文件时，这里补读文件内容
	p.Content, _ = loadPostContent(&p)
	return &p, nil
}

// validSlug 校验公开短码格式（字母数字，1~32 位，防止注入/异常字符）。
func validSlug(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for _, ch := range s {
		ok := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
		if !ok {
			return false
		}
	}
	return true
}

// findPostBySlug 按公开短码查文章（任何状态），返回 (id, status)。
func findPostBySlug(slug string) (int64, string, error) {
	var id int64
	var status string
	err := q.QueryRow(`SELECT id, status FROM posts WHERE slug = ?`, slug).Scan(&id, &status)
	return id, status, err
}

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
// 多个列表接口共用，避免重复代码。列顺序必须与 postListFields 完全一致：
//   id, slug, title, summary, cover, category, tags, status, likes, views, comments, created_at, updated_at
func scanListRows(rows *sql.Rows) ([]PostListItem, error) {
	items := []PostListItem{}
	for rows.Next() {
		var p PostListItem
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Summary, &p.Cover, &p.Category, &p.Tags,
			&p.Status, &p.Likes, &p.Views, &p.Comments, &p.CreatedAt, &p.UpdatedAt); err != nil {
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
// 查询参数：page（页码，默认1）、pageSize（每页条数，默认10），
// 以及三个可叠加的筛选条件（都为空时 = 全部已发布文章）：
//   category —— 精确匹配分类
//   tag      —— 精确匹配标签
//   q        —— 关键字：标题 / 摘要 / 分类 / 标签 / 公开短码 / 正文全文
//               （与 /api/search 共用 buildPostFilter，两边命中口径完全一致）
// 排序：带关键字时「标题命中」优先，其余按创建时间倒序（最新在前）。
func handleListPosts(c *gin.Context) {
	page, pageSize := parsePage(c)
	category := trimSpace(c.Query("category"))
	tag := trimSpace(c.Query("tag"))
	keyword := cutRunes(trimSpace(c.Query("q")), searchKeywordMaxRunes)

	where, args := buildPostFilter(category, tag, keyword)

	// ① 先查总数（用于前端算总页数）
	var total int64
	if err := q.QueryRow(`SELECT COUNT(*) FROM posts WHERE `+where, args...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// ② 查当前页数据。
	// 参数顺序必须跟 SQL 里出现的顺序一致：WHERE 的 → ORDER BY 的 → LIMIT/OFFSET。
	queryArgs := append([]any{}, args...)
	order := `created_at DESC, id DESC`
	if keyword != "" {
		order = `(title ILIKE ?) DESC, ` + order
		queryArgs = append(queryArgs, likeContains(keyword))
	}
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)

	rows, err := q.Query(
		`SELECT `+postListFields+` FROM posts WHERE `+where+` ORDER BY `+order+` LIMIT ? OFFSET ?`,
		queryArgs...,
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

// handleGetPost 公开：文章详情。GET /api/posts/:slug
// 注意：URL 用「随机短码 slug」（创建时生成，非递增 id、非标题拼音）；
// 草稿（draft）对外不可见 —— 条件里带 status='published'，访问草稿返回 404。
// 返回里带 liked 字段：当前访客（Cookie）是否已赞过。
func handleGetPost(c *gin.Context) {
	slug := c.Param("slug")
	if !validSlug(slug) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	visitor := visitorID(c) // 顺带确保访客有 Cookie（详情页要显示点赞状态）
	p, err := loadDetail(`slug = ? AND status = 'published'`, slug)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	p.Liked = hasLiked(p.ID, visitor)

	// 浏览量 +1（尽力而为：失败不影响正文返回）。
	// 只在公开详情接口里加，后台编辑用的 /api/admin/posts/:id 不会计数。
	if _, err := q.Exec(`UPDATE posts SET views = views + 1 WHERE id = ?`, p.ID); err == nil {
		p.Views++
	}

	c.JSON(http.StatusOK, p)
}

// handleLikePost 公开：点赞 / 取消点赞。POST /api/posts/:slug/like
// 规则（按需求）：无需登录；同一浏览器（Cookie）对每篇文章只能点一次，可再次点击取消。
// 实现：post_likes 表存去重记录，posts.likes 列做计数器（原子 ±1），事务保证一致。
func handleLikePost(c *gin.Context) {
	slug := c.Param("slug")
	if !validSlug(slug) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	id, status, err := findPostBySlug(slug)
	if err != nil || status != "published" {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	visitor := visitorID(c)

	// 开启事务：查记录 → 插入/删除 → 更新计数，避免并发下数据错乱
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "点赞失败"})
		return
	}
	defer tx.Rollback() // 提交成功后调用无害
	// 事务内也要做占位符翻译（PostgreSQL 用 $1/$2）
	txq := pgTx{tx}

	liked := false
	var n int
	if err := txq.QueryRow(`SELECT COUNT(*) FROM post_likes WHERE post_id = ? AND visitor = ?`, id, visitor).Scan(&n); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "点赞失败"})
		return
	}
	if n > 0 {
		// 已赞 → 取消：删记录、计数 -1（不低于 0）
		if _, err := txq.Exec(`DELETE FROM post_likes WHERE post_id = ? AND visitor = ?`, id, visitor); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
			return
		}
		if _, err := txq.Exec(`UPDATE posts SET likes = GREATEST(likes - 1, 0) WHERE id = ?`, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
			return
		}
	} else {
		// 未赞 → 点赞：插记录、计数 +1
		if _, err := txq.Exec(`INSERT INTO post_likes (post_id, visitor) VALUES (?, ?)`, id, visitor); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "点赞失败"})
			return
		}
		if _, err := txq.Exec(`UPDATE posts SET likes = likes + 1 WHERE id = ?`, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "点赞失败"})
			return
		}
		liked = true
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "点赞失败"})
		return
	}

	var likes int
	if err := q.QueryRow(`SELECT likes FROM posts WHERE id = ?`, id).Scan(&likes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "点赞失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": liked, "likes": likes})
}

// ------------------------------------------------------------
// 管理接口（后台用，需登录）
// ------------------------------------------------------------

// handleAdminListPosts 管理端：全部文章（含草稿）。GET /api/admin/posts
// 与公开列表的区别：不加 status 过滤，按更新时间倒序（最近改的排前面）。
func handleAdminListPosts(c *gin.Context) {
	page, pageSize := parsePage(c)

	var total int64
	if err := q.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	rows, err := q.Query(
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
// 与公开详情的唯一区别：不加 status 过滤（后台仍用数字 ID 操作），编辑草稿时用。
func handleAdminGetPost(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	p, err := loadDetail(`id = ?`, id)
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
	// 分类 / 标签规范化：去空白、标签统一用「英文逗号 + 空格」分隔；
	// 再按「字符数」截断到列宽上限
	in.Category = cutRunes(trimSpace(in.Category), 64)
	in.Tags = cutRunes(normalizeTags(in.Tags), 255)
	return in, true
}

// cutRunes 按字符数截断（中文一个字算一个字符）。
func cutRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// splitTags 把标签串按中英文逗号拆开，去掉空白与空项。
func splitTags(raw string) []string {
	raw = strings.ReplaceAll(raw, "，", ",")
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = trimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// normalizeTags 规范化标签串：拆分 → 去重 → 用「, 」重新拼接。
func normalizeTags(raw string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range splitTags(raw) {
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return strings.Join(out, ", ")
}

// handleCreatePost 管理端：新建文章。POST /api/admin/posts
// 请求体：{title, summary, cover, category, tags, content, status}
// 返回：创建成功后的完整文章（含自增 ID、随机短码 slug、时间）。
// 公开 URL 用随机短码：创建时自动生成，与标题、递增 id 都无关。
func handleCreatePost(c *gin.Context) {
	in, ok := bindPostInput(c)
	if !ok {
		return
	}
	// 生成唯一短码（撞库则换一个，最多重试 5 次）
	var id int64
	var slug string
	inserted := false
	for attempt := 0; attempt < 5 && !inserted; attempt++ {
		slug = genSlug()
		// PostgreSQL 用 RETURNING 取回自增主键（驱动不支持 LastInsertId）
		newID, err := insertReturningID(
			`INSERT INTO posts (slug, title, summary, cover, category, tags, content, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			slug, in.Title, in.Summary, in.Cover, in.Category, in.Tags, in.Content, in.Status,
		)
		if err != nil {
			if isUniqueViolation(err) {
				continue // slug 撞了唯一约束，重新生成
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
			return
		}
		id = newID
		inserted = true
	}
	if !inserted {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败，请重试"})
		return
	}
	// 查一次拿到创建后的完整行（含随机短码、数据库生成的时间）
	p, err := loadDetail(`id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	// 正文存到文件系统（若已启用），并把数据库的 content 列置空。
	// 放在插入之后：文件路径要按自增 id + created_at 计算，必须先有行。
	if postContent.Enabled() {
		dbValue := savePostContent(p, in.Content)
		if _, err := q.Exec(`UPDATE posts SET content = ? WHERE id = ?`, dbValue, p.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "正文保存失败"})
			return
		}
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

	// 分类 / 标签同样做规范化和截断（更新接口没走 bindPostInput）
	in.Category = cutRunes(trimSpace(in.Category), 64)
	in.Tags = cutRunes(normalizeTags(in.Tags), 255)

	// 是否需要动正文：动正文前必须先取到这一行的元数据（文件路径要用 created_at 算）
	contentDirty := in.Content != ""
	var existing *Post
	if contentDirty {
		var err error
		existing, err = loadDetail(`id = ?`, id)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
			return
		}
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
	if in.Category != "" {
		sets = append(sets, "category = ?")
		args = append(args, in.Category)
	}
	if in.Tags != "" {
		sets = append(sets, "tags = ?")
		args = append(args, in.Tags)
	}
	if contentDirty {
		// 用请求里的新值覆盖元数据，保证文件首部与数据库一致
		if in.Title != "" {
			existing.Title = in.Title
		}
		if in.Category != "" {
			existing.Category = in.Category
		}
		if in.Tags != "" {
			existing.Tags = in.Tags
		}
		if in.Status != "" {
			existing.Status = in.Status
		}
		dbValue := savePostContent(existing, in.Content)
		sets = append(sets, "content = ?")
		args = append(args, dbValue)
	}
	if in.Status != "" {
		sets = append(sets, "status = ?")
		args = append(args, in.Status)
	}
	if len(sets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有需要更新的内容"})
		return
	}
	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id)

	query := `UPDATE posts SET ` + strings.Join(sets, ", ") + ` WHERE id = ?`
	if _, err := q.Exec(query, args...); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}
	// 正文进文件后，content 列恒为 NULL，只改正文时 RowsAffected 会是 0，
	// 那是「没改动」而不是「文章不存在」，所以存在性单独判断。
	if !contentDirty {
		var n int
		if err := q.QueryRow(`SELECT COUNT(*) FROM posts WHERE id = ?`, id).Scan(&n); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
			return
		}
		if n == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id})
}

// handleDeletePost 管理端：删除文章。DELETE /api/admin/posts/:id
// 物理删除（不留回收站），前端已加二次确认。
// 级联：同时删除该文章的评论、点赞记录、以及相关站内通知。
func handleDeletePost(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	// 先取一份元数据：删完数据库行就不知道 created_at 了，而正文文件路径要用它算
	p, _ := loadDetail(`id = ?`, id)

	res, err := q.Exec(`DELETE FROM posts WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	// 正文文件一并删除（尽力而为）
	if p != nil {
		removePostContent(p)
	}
	// 级联清理（尽力而为，失败不影响主流程）
	_, _ = q.Exec(`DELETE FROM comments WHERE post_id = ?`, id)
	_, _ = q.Exec(`DELETE FROM post_likes WHERE post_id = ?`, id)
	_, _ = q.Exec(`DELETE FROM notifications WHERE post_id = ?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ------------------------------------------------------------
// 分类 / 全站统计（首页与列表页用）
// ------------------------------------------------------------

// handleListCategories 公开：已发布文章的分类及数量。GET /api/categories
func handleListCategories(c *gin.Context) {
	type catItem struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}
	items := []catItem{}
	// 按文章数倒序，数量相同再按名称排，保证顺序稳定
	rows, err := q.Query(
		`SELECT category, COUNT(*) AS n FROM posts
		 WHERE status = 'published' AND category <> ''
		 GROUP BY category
		 ORDER BY n DESC, category ASC`)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"items": items})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var it catItem
		if err := rows.Scan(&it.Name, &it.Count); err != nil {
			c.JSON(http.StatusOK, gin.H{"items": items})
			return
		}
		items = append(items, it)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// handleGetStats 公开：全站统计。GET /api/stats
// 口径：只统计「已发布」文章；分类按 category 去重；标签按逗号拆分后去重。
func handleGetStats(c *gin.Context) {
	var st Stats

	// ① 文章数
	if err := q.QueryRow(`SELECT COUNT(*) FROM posts WHERE status = 'published'`).Scan(&st.Posts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "统计失败"})
		return
	}
	// ② 分类数（空分类不计）
	if err := q.QueryRow(
		`SELECT COUNT(DISTINCT category) FROM posts WHERE status = 'published' AND category <> ''`,
	).Scan(&st.Categories); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "统计失败"})
		return
	}
	// ③ 标签数：数据库里是逗号分隔字符串，取出来在内存里拆分去重
	rows, err := q.Query(`SELECT tags FROM posts WHERE status = 'published' AND tags <> ''`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "统计失败"})
		return
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		for _, t := range splitTags(raw) {
			seen[t] = true
		}
	}
	st.Tags = int64(len(seen))
	c.JSON(http.StatusOK, st)
}
