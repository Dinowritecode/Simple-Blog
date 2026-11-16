package main

// ============================================================
// admin_users.go —— 后台用户管理模块（adminMiddleware）
// 职责：
//   GET    /api/admin/users         用户列表（含评论数，分页）
//   DELETE /api/admin/users/:id     删除用户（级联清理其评论与通知）
//
// 安全：不允许删除管理员账号；不允许删除自己。
// ============================================================

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleAdminListUsers 用户列表：GET /api/admin/users
// 查询参数：page / pageSize；可选 ?keyword= 按邮箱/昵称模糊搜索。
func handleAdminListUsers(c *gin.Context) {
	page, pageSize := parsePage(c)

	where := ""
	args := []any{}
	if kw := c.Query("keyword"); kw != "" {
		where = " WHERE u.email LIKE ? OR u.nickname LIKE ?"
		like := "%" + kw + "%"
		args = append(args, like, like)
	}

	var total int64
	if err := q.QueryRow(`SELECT COUNT(*) FROM users u`+where, args...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	rows, err := q.Query(
		`SELECT u.id, u.email, u.username, u.nickname, u.avatar, u.role, u.created_at,
		        (SELECT COUNT(*) FROM comments c WHERE c.user_id = u.id) AS comment_count
		 FROM users u`+where+` ORDER BY u.id ASC LIMIT ? OFFSET ?`,
		append(args, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()

	items := []UserAdminItem{}
	for rows.Next() {
		var it UserAdminItem
		var username sql.NullString
		if err := rows.Scan(&it.ID, &it.Email, &username, &it.Nickname, &it.Avatar, &it.Role, &it.CreatedAt,
			&it.CommentCount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
			return
		}
		it.Username = username.String
		items = append(items, it)
	}
	// 迭代结束后检查是否有中途错误（避免把不完整结果当正常返回）
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, PageResult{Items: items, Total: total, Page: page, PageSize: pageSize})
}

// handleAdminDeleteUser 删除用户：DELETE /api/admin/users/:id
// 级联：删除该用户的评论、作为接收者/触发者的通知。
// 注意：回复过别人的评论不会因对方删号受影响（渲染时已做容错）。
func handleAdminDeleteUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	me := authUser(c)
	if me != nil && me.ID == id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除自己的账号"})
		return
	}

	var role string
	err := q.QueryRow(`SELECT role FROM users WHERE id = ?`, id).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}
	if role == "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "不能删除管理员账号"})
		return
	}

	// 级联清理（尽力而为）
	_, _ = q.Exec(`DELETE FROM notifications WHERE user_id = ? OR actor_id = ?`, id, id)
	_, _ = q.Exec(`DELETE FROM comments WHERE user_id = ?`, id)

	if _, err := q.Exec(`DELETE FROM users WHERE id = ?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
