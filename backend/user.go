package main

// ============================================================
// user.go —— 用户个人中心模块
// 职责：登录用户的自助操作 ——
//   - GET  /api/user/me                    当前用户信息
//   - PUT  /api/user/profile                修改昵称
//   - PUT  /api/user/password               修改密码（需原密码）
//   - POST /api/user/avatar                 上传头像（≤5MB，裁剪后图片）
//   - GET  /api/user/notifications          我的回复通知（分页 + 未读数）
//   - POST /api/user/notifications/read     标记通知已读
//
// 全部挂在 authMiddleware 下（登录即可）。
// 头像大小限制 5MB（比正文图片上传的 10MB 更严，见 upload.go）。
// ============================================================

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// maxAvatarSize 头像大小上限：5MB
const maxAvatarSize = 5 << 20 // 5 * 1024 * 1024 字节

// handleMe 当前用户信息：GET /api/user/me
func handleMe(c *gin.Context) {
	u := authUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	c.JSON(http.StatusOK, u)
}

// handleUpdateProfile 修改昵称 / 主页封面：PUT /api/user/profile
// 请求体: {"nickname": "...", "cover": "/uploads/xxx.jpg"} —— 两个都是可选的，
// 只传一个就只改一个；cover 传空串表示恢复默认（用站点首页背景）。
func handleUpdateProfile(c *gin.Context) {
	u := authUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	var req struct {
		Nickname *string `json:"nickname"`
		Cover    *string `json:"cover"`
		Email    *string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if req.Nickname == nil && req.Cover == nil && req.Email == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有要修改的字段"})
		return
	}

	if req.Nickname != nil {
		nick := strings.TrimSpace(*req.Nickname)
		if nick == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "昵称不能为空"})
			return
		}
		if len([]rune(nick)) > 30 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "昵称最长 30 个字符"})
			return
		}
		if _, err := q.Exec(`UPDATE users SET nickname = ? WHERE id = ?`, nick, u.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
			return
		}
		u.Nickname = nick
	}

	if req.Cover != nil {
		cover := strings.TrimSpace(*req.Cover)
		// 只接受站内上传路径或 http(s) 地址，防止塞进奇怪的协议
		if cover != "" && !strings.HasPrefix(cover, "/uploads/") &&
			!strings.HasPrefix(cover, "http://") && !strings.HasPrefix(cover, "https://") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "封面地址不合法"})
			return
		}
		if len(cover) > 255 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "封面地址过长"})
			return
		}
		if _, err := q.Exec(`UPDATE users SET cover = ? WHERE id = ?`, cover, u.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
			return
		}
		u.Cover = cover
	}

	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		if email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱不能为空"})
			return
		}
		if len([]rune(email)) > 120 || !emailPattern.MatchString(email) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱格式不正确"})
			return
		}
		var taken int
		if err := q.QueryRow(`SELECT COUNT(*) FROM users WHERE email = ? AND id <> ?`, email, u.ID).Scan(&taken); err == nil && taken > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "这个邮箱已经被其它账号占用"})
			return
		}
		if _, err := q.Exec(`UPDATE users SET email = ? WHERE id = ?`, email, u.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
			return
		}
		u.Email = email
	}

	c.JSON(http.StatusOK, u)
}

// handleChangePassword 修改密码：PUT /api/user/password
// 请求体: {"oldPassword": "...", "newPassword": "..."}
// 必须校验原密码；新密码至少 6 位。
func handleChangePassword(c *gin.Context) {
	u := authUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if len(req.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码至少需要 6 位"})
		return
	}
	// 校验原密码
	var hash string
	if err := q.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, u.ID).Scan(&hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务端错误"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.OldPassword)) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原密码不正确"})
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务端错误"})
		return
	}
	if _, err := q.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(newHash), u.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "修改失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleUploadAvatar 上传头像：POST /api/user/avatar（multipart，字段名 avatar）
// 限制：≤5MB，仅 jpg/jpeg/png/gif/webp（前端裁剪后通常为 png/jpeg/webp）。
// 保存到 uploads/avatars/，并更新 users.avatar。
func handleUploadAvatar(c *gin.Context) {
	u := authUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	file, err := c.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未接收到文件"})
		return
	}
	if file.Size > maxAvatarSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "头像不能超过 5MB"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExt[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 jpg / jpeg / png / gif / webp 图片"})
		return
	}
	// 文件名：u<用户ID>_<随机串>，方便清理与识别
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务端错误"})
		return
	}
	name := "u" + strconv.FormatInt(u.ID, 10) + "_" + hex.EncodeToString(buf) + ext
	dir := filepath.Join(uploadDir, "avatars")
	dst := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	url := "/uploads/avatars/" + name

	// 更新库并删除旧头像文件（只删本项目 avatars 目录下的）
	old := u.Avatar
	if _, err := q.Exec(`UPDATE users SET avatar = ? WHERE id = ?`, url, u.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	removeOldAvatar(old)
	u.Avatar = url
	c.JSON(http.StatusOK, gin.H{"avatar": url, "user": u})
}

// removeOldAvatar 删除旧头像文件（仅在 uploads/avatars 目录内，防路径穿越）。
func removeOldAvatar(url string) {
	if !strings.HasPrefix(url, "/uploads/avatars/") {
		return
	}
	// 只删文件名部分，保证不会删到目录外
	name := strings.TrimPrefix(url, "/uploads/avatars/")
	if name == "" || strings.ContainsAny(name, `/\`) {
		return
	}
	_ = os.Remove(filepath.Join(uploadDir, "avatars", name))
}

// scanNotification 扫描一行通知（含 actor 信息，LEFT JOIN 后可能为 NULL）。
func scanNotification(row interface{ Scan(...any) error }) (*Notification, error) {
	var n Notification
	var actorID, commentID sql.NullInt64
	var nickname, avatar, role, userID, postSlug sql.NullString
	var level sql.NullInt64
	if err := row.Scan(&n.ID, &actorID, &n.PostID, &commentID, &n.Content, &n.IsRead, &n.CreatedAt,
		&nickname, &avatar, &role, &userID, &level, &postSlug); err != nil {
		return nil, err
	}
	if commentID.Valid {
		n.CommentID = commentID.Int64
	}
	n.PostSlug = postSlug.String
	if actorID.Valid {
		lv := int(level.Int64)
		if lv < 1 {
			lv = 1
		}
		n.Actor = &UserBrief{ID: actorID.Int64, UserID: userID.String, Nickname: nickname.String,
			Avatar: avatar.String, Level: lv, Role: role.String}
	}
	return &n, nil
}

// handleNotifications 我的通知列表：GET /api/user/notifications?page=&pageSize=
// 同时返回未读数 unreadCount（铃铛角标用）。
func handleNotifications(c *gin.Context) {
	u := authUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	page, pageSize := parsePage(c)

	var unread int64
	if err := q.QueryRow(
		`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = FALSE`, u.ID,
	).Scan(&unread); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	var total int64
	if err := q.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ?`, u.ID).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	rows, err := q.Query(
		`SELECT n.id, n.actor_id, n.post_id, n.comment_id, n.content, n.is_read, n.created_at,
		        u.nickname, u.avatar, u.role, u.user_id, COALESCE(u.level,1),
		        COALESCE(p.slug, '') AS post_slug
		 FROM notifications n
		 LEFT JOIN users u ON u.id = n.actor_id
		 LEFT JOIN posts p ON p.id = n.post_id
		 WHERE n.user_id = ?
		 ORDER BY n.id DESC
		 LIMIT ? OFFSET ?`,
		u.ID, pageSize, (page-1)*pageSize,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()

	items := []Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
			return
		}
		items = append(items, *n)
	}
	// 迭代结束后检查是否有中途错误，避免把「读到一半出错」当成完整结果返回
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"total":       total,
		"page":        page,
		"pageSize":    pageSize,
		"unreadCount": unread,
	})
}

// handleMarkNotificationsRead 标记已读：POST /api/user/notifications/read
// 请求体: {"ids": [1,2,3]} 标记指定；空数组/不传则全部标记已读。
func handleMarkNotificationsRead(c *gin.Context) {
	u := authUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	var req struct {
		IDs []int64 `json:"ids"`
	}
	_ = c.ShouldBindJSON(&req) // 空 body 也允许（视为全部已读）

	if len(req.IDs) == 0 {
		if _, err := q.Exec(`UPDATE notifications SET is_read = TRUE WHERE user_id = ?`, u.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
			return
		}
	} else {
		// 拼 ? 占位符；只更新属于当前用户的通知，防越权
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(req.IDs)), ",")
		args := make([]any, 0, len(req.IDs)+1)
		args = append(args, u.ID)
		for _, id := range req.IDs {
			args = append(args, id)
		}
		if _, err := q.Exec(
			`UPDATE notifications SET is_read = TRUE WHERE user_id = ? AND id IN (`+placeholders+`)`, args...,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
