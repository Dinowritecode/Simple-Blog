package main

// ============================================================
// moment_comments.go —— 说说的评论
//
// 规则与文章评论完全一致（前端也是同一个组件）：
//   · 只有登录用户能评论 / 回复，内容纯文本、≤2000 字；
//   · 只保留两级：parent_id 恒为顶级评论 ID，回复「回复」也挂到同一顶级下，
//     用 reply_to_user_id 记住被 @ 的人；
//   · 查询列与文章评论共用 commentJoinFields，扫描 / 转模型 / 组树也共用，
//     所以两边的样式和层级行为天然一致。
//
// 与文章评论的差异：不发通知。原因是通知点击后会跳到文章页（/blog/<slug>?c=），
// 说说没有 slug，链过去会是死链；等说说的详情路由做出来再接。
// ============================================================

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// handleListMomentComments GET /api/moments/:id/comments
func handleListMomentComments(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的 ID"})
		return
	}
	var exists int
	if err := q.QueryRow(`SELECT COUNT(*) FROM moments WHERE id = ?`, id).Scan(&exists); err != nil || exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "这条说说不存在"})
		return
	}

	// 先倒序取最近的 300 条，再反转成正序（与文章评论同一套做法）
	rows, err := q.Query(
		`SELECT `+momentCommentFields+`
		 FROM moment_comments c
		 LEFT JOIN users au ON au.id = c.user_id
		 LEFT JOIN users ru ON ru.id = c.reply_to_user_id
		 WHERE c.moment_id = ?
		 ORDER BY c.id DESC LIMIT 300`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()

	raw := []*commentRow{}
	for rows.Next() {
		r, err := scanComment(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
			return
		}
		raw = append(raw, r)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	for i, j := 0, len(raw)-1; i < j; i, j = i+1, j-1 {
		raw[i], raw[j] = raw[j], raw[i]
	}

	var total int64
	_ = q.QueryRow(`SELECT COUNT(*) FROM moment_comments WHERE moment_id = ?`, id).Scan(&total)
	c.JSON(http.StatusOK, gin.H{"items": assembleCommentTree(raw), "total": total})
}

// handleCreateMomentComment POST /api/moments/:id/comments（需登录）
func handleCreateMomentComment(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的 ID"})
		return
	}
	var exists int
	if err := q.QueryRow(`SELECT COUNT(*) FROM moments WHERE id = ?`, id).Scan(&exists); err != nil || exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "这条说说不存在"})
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

	// 解析回复关系：回复「回复」时仍挂到同一顶级下
	var parentID, replyToID int64
	if req.ParentID > 0 {
		pc, err := queryMomentCommentByID(req.ParentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "要回复的评论不存在"})
			return
		}
		if pc.PostID != id { // moment 评论里 PostID 存的是 moment_id
			c.JSON(http.StatusBadRequest, gin.H{"error": "要回复的评论不属于这条说说"})
			return
		}
		if pc.ParentID > 0 {
			parentID = pc.ParentID
		} else {
			parentID = pc.ID
		}
		replyToID = pc.UserID
	}

	newID, err := insertReturningID(
		`INSERT INTO moment_comments (moment_id, user_id, parent_id, reply_to_user_id, content)
		 VALUES (?, ?, ?, ?, ?) RETURNING id`,
		id, me.ID, nullInt64(parentID), nullInt64(replyToID), content,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": newID, "parentId": parentID})
}

// momentCommentFields 与 commentJoinFields 同构，唯一区别是列名：
// c.moment_id 用别名 post_id 输出，这样能直接复用 scanComment / toComment。
const momentCommentFields = `
	c.id, c.moment_id AS post_id, c.user_id, c.parent_id, c.reply_to_user_id, c.content, c.created_at,
	COALESCE(NULLIF(au.nickname,''), SPLIT_PART(au.email,'@',1), '') AS author_nick,
	COALESCE(au.avatar,'') AS author_avatar, COALESCE(au.role,'user') AS author_role,
	COALESCE(au.user_id,'') AS author_uid, COALESCE(au.level,1) AS author_level,
	COALESCE(NULLIF(ru.nickname,''), SPLIT_PART(ru.email,'@',1), '') AS reply_nick,
	COALESCE(ru.avatar,'') AS reply_avatar, COALESCE(ru.role,'user') AS reply_role,
	COALESCE(ru.user_id,'') AS reply_uid, COALESCE(ru.level,1) AS reply_level`

func queryMomentCommentByID(id int64) (*commentRow, error) {
	row := q.QueryRow(
		`SELECT `+momentCommentFields+`
		 FROM moment_comments c
		 LEFT JOIN users au ON au.id = c.user_id
		 LEFT JOIN users ru ON ru.id = c.reply_to_user_id
		 WHERE c.id = ?`, id)
	return scanComment(row)
}
