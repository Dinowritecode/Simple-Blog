package main

// ============================================================
// comments.go —— 评论与通知模块
// 职责：
//   公开   GET  /api/posts/:id/comments          文章评论列表（两级树）
//   登录   POST /api/posts/:id/comments          发表评论 / 回复（回复自动通知对方）
//   管理   GET  /api/admin/comments              后台评论管理（分页）
//   管理   DELETE /api/admin/comments/:id        删除评论（级联删其回复与通知）
//
// 规则：
//   - 仅登录用户可评论 / 回复；内容为纯文本，≤2000 字。
//   - 评论只保留两级：parent_id 恒为顶级评论 ID；回复「回复」时也挂在顶级下，
//     用 reply_to_user_id 记住被 @ 的人（通知只发给被回复者本人）。
// ============================================================

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// maxCommentLen 评论内容最长字符数
const maxCommentLen = 2000

// commentJoinFields 评论查询复用的 SELECT 片段（作者与 @ 对象 LEFT JOIN users）。
// 用户不存在/未设置时相关列可能全 NULL，故 COALESCE 兜底空串，保证可 Scan 进 string。
// 同时取出作者的 9 位桃信 ID 与等级：前端在评论里显示「私聊」入口与等级爱心。
const commentJoinFields = `
	c.id, c.post_id, c.user_id, c.parent_id, c.reply_to_user_id, c.content, c.created_at,
	COALESCE(NULLIF(au.nickname,''), SPLIT_PART(au.email,'@',1), '') AS author_nick,
	COALESCE(au.avatar,'') AS author_avatar, COALESCE(au.role,'user') AS author_role,
	COALESCE(au.user_id,'') AS author_uid, COALESCE(au.level,1) AS author_level,
	COALESCE(NULLIF(ru.nickname,''), SPLIT_PART(ru.email,'@',1), '') AS reply_nick,
	COALESCE(ru.avatar,'') AS reply_avatar, COALESCE(ru.role,'user') AS reply_role,
	COALESCE(ru.user_id,'') AS reply_uid, COALESCE(ru.level,1) AS reply_level`

// commentRow 一条评论的原始查询行（含 join 出来的展示信息）。
type commentRow struct {
	ID, PostID, UserID, ParentID, ReplyToID int64
	Content, CreatedAt                      string
	AuthorNick, AuthorAvatar, AuthorRole    string
	AuthorUID                               string
	AuthorLevel                             int
	ReplyNick, ReplyAvatar, ReplyRole       string
	ReplyUID                                string
	ReplyLevel                              int
}

func scanComment(row interface{ Scan(...any) error }) (*commentRow, error) {
	var r commentRow
	// parent_id / reply_to_user_id 允许 NULL（顶级评论），须用 NullInt64 容错
	var parentID, replyToID sql.NullInt64
	err := row.Scan(&r.ID, &r.PostID, &r.UserID, &parentID, &replyToID, &r.Content, &r.CreatedAt,
		&r.AuthorNick, &r.AuthorAvatar, &r.AuthorRole, &r.AuthorUID, &r.AuthorLevel,
		&r.ReplyNick, &r.ReplyAvatar, &r.ReplyRole, &r.ReplyUID, &r.ReplyLevel)
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		r.ParentID = parentID.Int64
	}
	if replyToID.Valid {
		r.ReplyToID = replyToID.Int64
	}
	return &r, nil
}

// toComment 转成对外 JSON 模型（回复对象按需填充）。
func (r *commentRow) toComment() Comment {
	c := Comment{
		ID:        r.ID,
		PostID:    r.PostID,
		ParentID:  r.ParentID, // 0 = 顶级
		Content:   r.Content,
		CreatedAt: r.CreatedAt,
		Author: UserBrief{
			ID:       r.UserID,
			UserID:   r.AuthorUID,
			Nickname: r.AuthorNick,
			Avatar:   r.AuthorAvatar,
			Level:    r.AuthorLevel,
			Role:     r.AuthorRole,
		},
		Replies: []Comment{},
	}
	if r.ParentID > 0 && r.ReplyToID > 0 {
		c.ReplyTo = &UserBrief{
			ID:       r.ReplyToID,
			UserID:   r.ReplyUID,
			Nickname: r.ReplyNick,
			Avatar:   r.ReplyAvatar,
			Level:    r.ReplyLevel,
			Role:     r.ReplyRole,
		}
	}
	return c
}

// queryCommentByID 按 ID 查一条评论（含 join 信息）。
func queryCommentByID(id int64) (*commentRow, error) {
	row := q.QueryRow(
		`SELECT `+commentJoinFields+`
		 FROM comments c
		 LEFT JOIN users au ON au.id = c.user_id
		 LEFT JOIN users ru ON ru.id = c.reply_to_user_id
		 WHERE c.id = ?`, id,
	)
	return scanComment(row)
}

// handleListComments 文章评论列表：GET /api/posts/:slug/comments
// 返回顶级评论数组（各自带 replies），时间正序；最多返回最近 300 条。
func handleListComments(c *gin.Context) {
	slug := c.Param("slug")
	if !validSlug(slug) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	id, _, err := findPostBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	rows, err := q.Query(
		`SELECT `+commentJoinFields+`
		 FROM comments c
		 LEFT JOIN users au ON au.id = c.user_id
		 LEFT JOIN users ru ON ru.id = c.reply_to_user_id
		 WHERE c.post_id = ?
		 ORDER BY c.id DESC LIMIT 300`, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()

	// 读出后反转成正序（先倒序 LIMIT 取最新的 300 条）
	raw := []*commentRow{}
	for rows.Next() {
		r, err := scanComment(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
			return
		}
		raw = append(raw, r)
	}
	// 迭代结束后检查是否有中途错误（避免把不完整结果当正常返回）
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	for i, j := 0, len(raw)-1; i < j; i, j = i+1, j-1 {
		raw[i], raw[j] = raw[j], raw[i]
	}

	// 组装两级树：parent_id=0 的是顶级；其余挂在顶级下（父被删则提升为顶级）。
	top := assembleCommentTree(raw)

	var total int64
	_ = q.QueryRow(`SELECT COUNT(*) FROM comments WHERE post_id = ?`, id).Scan(&total)
	c.JSON(http.StatusOK, gin.H{"items": top, "total": total})
}

// assembleCommentTree 把按 id 升序排好的评论行组装成两级树。
// 文章评论和说说评论共用这一份逻辑，保证两边的层级行为完全一致。
// 注意：必须先用「指针」收集，全部挂完后再一次性取值 —— 否则副本会丢失后到的回复。
func assembleCommentTree(raw []*commentRow) []Comment {
	byID := map[int64]*Comment{}
	roots := []int64{}
	for _, r := range raw {
		cm := r.toComment()
		if r.ParentID == 0 {
			byID[cm.ID] = &cm
			roots = append(roots, cm.ID)
			continue
		}
		if p, exists := byID[r.ParentID]; exists {
			p.Replies = append(p.Replies, cm)
		} else {
			// 父评论已被删除：这条回复按顶级展示
			cm.ParentID = 0
			byID[cm.ID] = &cm
			roots = append(roots, cm.ID)
		}
	}
	top := make([]Comment, 0, len(roots))
	for _, id := range roots {
		top = append(top, *byID[id])
	}
	return top
}

// handleCreateComment 发表评论 / 回复：POST /api/posts/:slug/comments（需登录）
// 请求体: {"content": "...", "parentId": 123}；parentId 省略 = 顶级评论。
func handleCreateComment(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	slug := c.Param("slug")
	if !validSlug(slug) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	postID, status, err := findPostBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	// 只能评论已发布的文章
	if status != "published" {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	var req struct {
		Content  string `json:"content"`
		ParentID int64  `json:"parentId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论内容不能为空"})
		return
	}
	if len([]rune(content)) > maxCommentLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论最多 2000 字"})
		return
	}

	// 解析回复关系：被回复评论的作者即通知对象；回复「回复」仍挂到同一顶级下
	var parentID, replyToID int64
	if req.ParentID > 0 {
		pc, err := queryCommentByID(req.ParentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "要回复的评论不存在"})
			return
		}
		if pc.PostID != postID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "要回复的评论不属于这篇文章"})
			return
		}
		if pc.ParentID > 0 {
			parentID = pc.ParentID // 回复「回复」→ 挂到同一顶级下
		} else {
			parentID = pc.ID
		}
		replyToID = pc.UserID
	}

	newID, err := insertReturningID(
		`INSERT INTO comments (post_id, user_id, parent_id, reply_to_user_id, content)
		 VALUES (?, ?, ?, ?, ?) RETURNING id`,
		postID, me.ID, nullInt64(parentID), nullInt64(replyToID), content,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发表失败"})
		return
	}

	// 回复通知：只通知被回复的人（自己回复自己不通知）
	if replyToID > 0 && replyToID != me.ID {
		notifyReply(me, replyToID, postID, newID, content)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": newID, "parentId": parentID})
}

// notifyReply 给被回复者插入通知。content 带回复人昵称快照，
// 即使回复人账号之后被删，通知文案也完整可读。
// 写入后立刻通过 SSE 推一条 notification 事件 —— 对方在线时红点实时亮起，无需等轮询。
func notifyReply(actor *User, toUserID, postID, commentID int64, replyContent string) {
	nick := actor.Nickname
	if nick == "" {
		nick = actor.Email
	}
	snippet := replyContent
	if r := []rune(snippet); len(r) > 40 {
		snippet = string(r[:40]) + "…"
	}
	content := fmt.Sprintf("%s 回复了你的评论：%s", nick, snippet)
	newID, err := insertReturningID(
		`INSERT INTO notifications (user_id, actor_id, post_id, comment_id, content)
		 VALUES (?, ?, ?, ?, ?) RETURNING id`,
		toUserID, actor.ID, postID, commentID, content,
	)
	if err != nil {
		return
	}
	broadcastNotification(toUserID, gin.H{
		"id":        newID,
		"content":   content,
		"commentId": commentID,
		"postId":    postID,
		"createdAt": nowString(),
	})
}

// nullInt64 把 0 转成 SQL NULL（父评论/回复目标可空）。
func nullInt64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// ------------------------------------------------------------
// 后台管理（adminMiddleware）
// ------------------------------------------------------------

// handleAdminListComments 后台评论列表：GET /api/admin/comments
// 查询参数：page / pageSize；可选 ?postId= 只看某篇文章。
func handleAdminListComments(c *gin.Context) {
	page, pageSize := parsePage(c)
	where := ""
	args := []any{}
	if pid := c.Query("postId"); pid != "" {
		where = " WHERE c.post_id = ?"
		args = append(args, pid)
	}

	var total int64
	if err := q.QueryRow(`SELECT COUNT(*) FROM comments c`+where, args...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	rows, err := q.Query(
		`SELECT c.id, c.post_id, c.user_id, c.parent_id, c.reply_to_user_id, c.content, c.created_at,
		        COALESCE(NULLIF(au.nickname,''), SPLIT_PART(au.email,'@',1), '') AS author_nick,
		        COALESCE(au.avatar,'') AS author_avatar, COALESCE(au.role,'user') AS author_role,
		        COALESCE(au.user_id,'') AS author_uid, COALESCE(au.level,1) AS author_level,
		        COALESCE(NULLIF(ru.nickname,''), SPLIT_PART(ru.email,'@',1), '') AS reply_nick,
		        COALESCE(ru.avatar,'') AS reply_avatar, COALESCE(ru.role,'user') AS reply_role,
		        COALESCE(ru.user_id,'') AS reply_uid, COALESCE(ru.level,1) AS reply_level,
		        COALESCE(p.title, '') AS post_title, COALESCE(p.slug, '') AS post_slug
		 FROM comments c
		 LEFT JOIN users au ON au.id = c.user_id
		 LEFT JOIN users ru ON ru.id = c.reply_to_user_id
		 LEFT JOIN posts p ON p.id = c.post_id
		`+where+` ORDER BY c.id DESC LIMIT ? OFFSET ?`,
		append(args, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()

	items := []CommentAdminItem{}
	for rows.Next() {
		var it CommentAdminItem
		var authorNick, authorAvatar, authorRole, authorUID string
		var authorLevel int
		var replyNick, replyAvatar, replyRole, replyUID string
		var replyLevel int
		var authorID, parentID, replyToID sql.NullInt64
		if err := rows.Scan(&it.ID, &it.PostID, &authorID, &parentID, &replyToID, &it.Content, &it.CreatedAt,
			&authorNick, &authorAvatar, &authorRole, &authorUID, &authorLevel,
			&replyNick, &replyAvatar, &replyRole, &replyUID, &replyLevel,
			&it.PostTitle, &it.PostSlug); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
			return
		}
		if parentID.Valid {
			it.ParentID = parentID.Int64
		}
		it.Author = UserBrief{ID: authorID.Int64, UserID: authorUID, Nickname: authorNick,
			Avatar: authorAvatar, Level: authorLevel, Role: authorRole}
		if replyToID.Valid {
			it.ReplyTo = &UserBrief{ID: replyToID.Int64, UserID: replyUID, Nickname: replyNick,
				Avatar: replyAvatar, Level: replyLevel, Role: replyRole}
		}
		items = append(items, it)
	}
	// 迭代结束后检查是否有中途错误（避免把不完整结果当正常返回）
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, PageResult{Items: items, Total: total, Page: page, PageSize: pageSize})
}

// handleAdminDeleteComment 删除评论：DELETE /api/admin/comments/:id
// 若是顶级评论，连同其下所有回复一起删；并清理关联通知。
func handleAdminDeleteComment(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var parentID int64
	var pNull sql.NullInt64
	err := q.QueryRow(`SELECT parent_id FROM comments WHERE id = ?`, id).Scan(&pNull)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "评论不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}
	if pNull.Valid {
		parentID = pNull.Int64
	}

	// 顶级评论：连回复一起删；回复：只删自己
	if parentID == 0 {
		if _, err := q.Exec(`DELETE FROM comments WHERE id = ? OR parent_id = ?`, id, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
			return
		}
	} else {
		if _, err := q.Exec(`DELETE FROM comments WHERE id = ?`, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
			return
		}
	}
	// 尽力清理相关通知（comment_id 指向被删评论的）
	_, _ = q.Exec(`DELETE FROM notifications WHERE comment_id = ?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
