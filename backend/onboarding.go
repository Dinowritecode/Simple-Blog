package main

// ============================================================
// onboarding.go —— 后台「首次进入时的初始化引导」
//
// 只有一件事：站长第一次进后台时，先让 TA 上传头像 + 填一个邮箱（不做验证），
// 点完成之后写一个标记，之后再也不显示。
//
// 为什么放后端而不是前端 localStorage：
//   · 换浏览器 / 换设备不该再走一遍引导；
//   · 标记属于站点状态，跟 settings 其它项一起存在库里最自然。
//
// 接口：
//   GET  /api/admin/onboarding   {done, email, hasAvatar}
//   POST /api/admin/onboarding   {email}  → 校验邮箱、写入 users.email、置标记
// ============================================================

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

const settingAdminOnboarded = "admin_onboarded"

// 只做「看起来像邮箱」的宽松校验，不发验证邮件
var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// handleAdminOnboardingState 查询是否已经完成初始化。GET /api/admin/onboarding
func handleAdminOnboardingState(c *gin.Context) {
	u := authUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"done":      getSetting(settingAdminOnboarded) == "1",
		"email":     u.Email,
		"hasAvatar": strings.TrimSpace(u.Avatar) != "",
	})
}

// handleAdminOnboardingComplete 完成初始化：保存邮箱并置标记。POST /api/admin/onboarding
// 请求体：{"email": "me@example.com"}
func handleAdminOnboardingComplete(c *gin.Context) {
	u := authUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写邮箱"})
		return
	}
	if len([]rune(email)) > 120 || !emailPattern.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱格式不正确"})
		return
	}

	// 同一邮箱不能属于别的账号（站长自己重填同一个没问题）
	var taken int
	if err := q.QueryRow(`SELECT COUNT(*) FROM users WHERE email = ? AND id <> ?`, email, u.ID).Scan(&taken); err == nil && taken > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "这个邮箱已经被其它账号占用"})
		return
	}

	if _, err := q.Exec(`UPDATE users SET email = ? WHERE id = ?`, email, u.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存邮箱失败"})
		return
	}
	u.Email = email

	if err := setSetting(settingAdminOnboarded, "1"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存状态失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "user": u})
}
