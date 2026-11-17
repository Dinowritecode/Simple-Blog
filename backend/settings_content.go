package main

// ============================================================
// settings_content.go —— 「内容类」设置项（首页文案 + 登录页背景）
//
// 为什么单独一个文件：
//   settings.go 里是一堆「一项一个接口」的老风格（home_bg / post_bg / notice …），
//   这次要加 5 个内容项，再复制 5 遍 handler 太啰嗦，所以统一成一个接口：
//     PUT /api/admin/settings/content  { nickname, desc, roles, avatar, loginBg }
//   传哪个就改哪个（未传的字段保持原值）。
//
// 存储约定：
//   hero_nickname  首页昵称，用 | 分隔「主题色前半段」和「正文」，如 凛冬|RinEyce
//   hero_desc      首屏介绍，支持 **文字** 表示高亮
//   hero_roles     打字动画的候选，一行一条
//   hero_avatar    首屏头像 URL（空 = 用内置头像）
//   login_bg       登录页左半屏背景（空 = 用主题色底）
// 全部留空时前端回落到内置默认值，所以老库不用迁移。
// ============================================================

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const (
	keyHeroNickname = "hero_nickname"
	keyHeroDesc     = "hero_desc"
	keyHeroRoles    = "hero_roles"
	keyHeroAvatar   = "hero_avatar"
	keyLoginBg      = "login_bg"
)

// 长度上限（按字符数，不是字节）
const (
	maxNicknameRunes = 40
	maxDescRunes     = 300
	maxRoleRunes     = 40
	maxRoleCount     = 10
)

// handleAdminUpdateContent 管理员：保存首页文案 / 头像 / 登录页背景。
// PUT /api/admin/settings/content
// 请求体（全部可选，只传要改的）：{"nickname":"凛冬|RinEyce","desc":"…","roles":"一行一条","avatar":"/uploads/a.jpg","loginBg":"/uploads/b.jpg"}
func handleAdminUpdateContent(c *gin.Context) {
	var req struct {
		Nickname *string `json:"nickname"`
		Desc     *string `json:"desc"`
		Roles    *string `json:"roles"`
		Avatar   *string `json:"avatar"`
		LoginBg  *string `json:"loginBg"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	// 昵称
	if req.Nickname != nil {
		v := strings.TrimSpace(*req.Nickname)
		if utf8.RuneCountInString(v) > maxNicknameRunes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "昵称最长 40 个字符"})
			return
		}
		// 允许的形式：前半段|后半段；不允许出现换行
		v = strings.ReplaceAll(v, "\n", "")
		if err := setSetting(keyHeroNickname, v); err != nil {
			serverError(c, err)
			return
		}
	}

	// 介绍
	if req.Desc != nil {
		v := strings.TrimSpace(*req.Desc)
		if utf8.RuneCountInString(v) > maxDescRunes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "介绍最长 300 个字符"})
			return
		}
		if err := setSetting(keyHeroDesc, v); err != nil {
			serverError(c, err)
			return
		}
	}

	// 打字动画候选：一行一条，去空行、限条数与单行长度
	if req.Roles != nil {
		lines := strings.Split(strings.ReplaceAll(*req.Roles, "\r", ""), "\n")
		out := make([]string, 0, len(lines))
		for _, ln := range lines {
			ln = strings.TrimSpace(ln)
			if ln == "" {
				continue
			}
			if utf8.RuneCountInString(ln) > maxRoleRunes {
				c.JSON(http.StatusBadRequest, gin.H{"error": "每行文字最长 40 个字符"})
				return
			}
			out = append(out, ln)
		}
		if len(out) > maxRoleCount {
			c.JSON(http.StatusBadRequest, gin.H{"error": "最多 10 行"})
			return
		}
		if err := setSetting(keyHeroRoles, strings.Join(out, "\n")); err != nil {
			serverError(c, err)
			return
		}
	}

	// 头像 / 登录页背景：都是图片地址，走同一套 URL 白名单校验
	for _, item := range []struct {
		key string
		val *string
	}{
		{keyHeroAvatar, req.Avatar},
		{keyLoginBg, req.LoginBg},
	} {
		if item.val == nil {
			continue
		}
		v := strings.TrimSpace(*item.val)
		if !safeBgURL(v) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "图片地址必须以 / 或 http(s):// 开头"})
			return
		}
		if err := setSetting(item.key, v); err != nil {
			serverError(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"nickname": getSetting(keyHeroNickname),
		"desc":     getSetting(keyHeroDesc),
		"roles":    getSetting(keyHeroRoles),
		"avatar":   getSetting(keyHeroAvatar),
		"loginBg":  getSetting(keyLoginBg),
	})
}

// serverError 统一的 500 响应（内容类设置只可能因为写库失败而报错）。
func serverError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败：" + err.Error()})
}
