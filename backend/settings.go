package main

// ============================================================
// settings.go —— 站点页面设置模块
// 目前支持：首页背景图（home_bg）—— 管理员可填图片 URL 或上传本地图片。
//
//   GET  /api/settings            （公开）读取公开站点设置（首页背景等）
//   PUT  /api/admin/settings/home （管理员）保存首页背景图 URL / 清空
//
// 存储：settings 键值表（key = home_bg）。前端拿到 URL 后作为 CSS 背景使用。
// ============================================================

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// settingsPublicKeys 对公众开放的设置键（避免把内部设置泄露出去）。
var settingsPublicKeys = map[string]bool{
	"home_bg": true,
	"post_bg": true,
	"notice":  true,
	// 首页文案与登录页背景（后台「设置 → 首页文案」里改）
	"hero_nickname": true,
	"hero_desc":     true,
	"hero_roles":    true,
	"hero_avatar":   true,
	"login_bg":      true,
}

// ---------- 背景图：首页 / 文章页各一张 ----------
// 两个键完全独立：后台可以只改其中一个，另一个保持不动。
// 老库升级上来时 post_bg 还不存在，由 migratePostBg 从 home_bg 复制一份兜底，
// 保证「升级完文章页背景不会突然消失」；之后想清空文章页背景写空串即可。
const (
	settingHomeBg = "home_bg"
	settingPostBg = "post_bg"
)

// ---------- 功能开关：Momo Talk ----------
// 后台可关闭整个 Momo Talk（私信 / 好友）功能；关闭后前端不渲染任何入口
// （导航栏按钮、评论区的「私信」按钮、小窗），后端接口仍然存在但前端不再调用。
const settingMomoTalk = "enable_momo_talk"

// getMomoTalkEnabled Momo Talk 是否启用（默认启用；设置值只认 "false" 为关闭）。
func getMomoTalkEnabled() bool {
	return strings.TrimSpace(getSetting(settingMomoTalk)) != "false"
}

// ---------- 全站字体 ----------
// 前端 public/font 下放了一份 Gen Jyuu Gothic（思源柔黑）。
// 这里只保存「选了哪一款」，真正的 @font-face 声明在 style.css 里。
const settingSiteFont = "site_font"

// 可选的字体标识（与前端 style.css 里的 @font-face / 字体栈一一对应）
const (
	fontSystem = "system" // 系统默认字体栈
	fontRouHei = "rouhei" // Gen Jyuu Gothic（思源柔黑）
	fontRound  = "round"  // 圆体（系统字体里挑圆润的）
)

// validSiteFont 校验字体标识。
func validSiteFont(v string) bool {
	switch v {
	case fontSystem, fontRouHei, fontRound:
		return true
	}
	return false
}

// getSiteFont 读取当前全站字体；没设置或非法时回退「系统默认」。
func getSiteFont() string {
	v := strings.TrimSpace(getSetting(settingSiteFont))
	if !validSiteFont(v) {
		return fontSystem
	}
	return v
}

// noticeMaxRunes 公告字数上限（按字符数算，一个中文/英文/符号都算一个）。
const noticeMaxRunes = 300

// 文章列表分页：每页条数在后台可改，这里是默认值与上限（与 parsePage 的 50 上限一致）。
const (
	defaultPageSize = 20
	maxPageSize     = 50
)

// getPageSize 读取「每页文章数」设置；缺失或非法时回退默认值。
func getPageSize() int {
	n, err := strconv.Atoi(getSetting("page_size"))
	if err != nil || n < 1 || n > maxPageSize {
		return defaultPageSize
	}
	return n
}

// ---------- 外观定制 ----------
// 主题色 + 每个模块独立的「底色浓度」和「模糊半径」。
const (
	defaultAccent     = "#00AAEE" // 主题色（蔚蓝）
	defaultPanelColor = "#FFFFFF" // 下方毛玻璃区域底色（白）
)

// themeInt 定义一个整数型外观参数：设置键、默认值、上限。
type themeInt struct {
	Key string
	Def int
	Max int
}

var (
	// 各模块底色浓度（%，越小越透明）
	themeAlphas = []themeInt{
		{"card_alpha", 50, 100},   // 卡片（技能 / 项目 / 数据块 / 后台面板…）
		{"post_alpha", 50, 100},   // 文章卡片 + 左右两栏的卡片（名片 / 公告 / 统计 / 日历）
		{"panel_alpha", 100, 100}, // 下方区域：不透明（白色实底，不再是半透明玻璃）
		{"nav_alpha", 55, 100},    // 导航栏（滚到顶变色后的状态）
	}
	// 各模块模糊半径（px）
	themeBlurs = []themeInt{
		{"card_blur", 18, 50},
		{"post_blur", 0, 50},  // 文章卡片默认不带模糊
		{"panel_blur", 0, 50}, // 下方毛玻璃区域：默认不模糊
		{"nav_blur", 18, 50},
	}
)

// getThemeInt 读取一个整数型外观参数；没设置或越界时回退默认值。
func getThemeInt(t themeInt) int {
	n, err := strconv.Atoi(getSetting(t.Key))
	if err != nil || n < 0 || n > t.Max {
		return t.Def
	}
	return n
}

// snakeToCamel 把设置键转成前端字段名：card_alpha → cardAlpha。
func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	out := parts[0]
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		out += strings.ToUpper(p[:1]) + p[1:]
	}
	return out
}

// validHexColor 校验 #rgb 或 #rrggbb 形式的颜色值。
func validHexColor(s string) bool {
	if len(s) != 4 && len(s) != 7 {
		return false
	}
	if s[0] != '#' {
		return false
	}
	for _, ch := range s[1:] {
		ok := (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
		if !ok {
			return false
		}
	}
	return true
}

// getAccent 读取主题色；没设置或格式非法时回退默认值。
func getAccent() string {
	v := strings.TrimSpace(getSetting("accent"))
	if !validHexColor(v) {
		return defaultAccent
	}
	return v
}

// getPanelColor 读取下方毛玻璃区域的底色；没设置或格式非法时回退默认值。
func getPanelColor() string {
	v := strings.TrimSpace(getSetting("panel_color"))
	if !validHexColor(v) {
		return defaultPanelColor
	}
	return v
}

// getSetting 读取单条设置（不存在返回空串）。
func getSetting(key string) string {
	var v string
	// key 是 PostgreSQL 的非保留字，但加引号更稳妥（与建表语句一致）
	err := q.QueryRow(`SELECT value FROM settings WHERE "key" = ?`, key).Scan(&v)
	if err != nil {
		return ""
	}
	return v
}

// setSetting 写入/更新单条设置（幂等）。
func setSetting(key, value string) error {
	_, err := q.Exec(
		`INSERT INTO settings ("key", value) VALUES (?, ?)
		 ON CONFLICT ("key") DO UPDATE SET value = EXCLUDED.value`,
		key, value,
	)
	return err
}

// handleGetPublicSettings 公开：站点公开设置。GET /api/settings
// 响应示例：{"homeBg": "/uploads/bg_xxx.jpg", "startedAt": 1758730000000}
// （homeBg 未设置则为空串；startedAt 是本次服务启动的毫秒时间戳，供前端算运行时长）
func handleGetPublicSettings(c *gin.Context) {
	out := gin.H{}
	for k := range settingsPublicKeys {
		out[publicKeyJSON(k)] = getSetting(k)
	}
	// 本次进程启动时间（不落库，重启即刷新）
	out["startedAt"] = serverStart.UnixMilli()
	// 文章列表每页条数（数字类型，前端直接用来分页）
	out["pageSize"] = getPageSize()
	// 外观定制：主题色 + 面板底色 + 各模块的浓度/模糊
	out["accent"] = getAccent()
	out["panelColor"] = getPanelColor()
	for _, t := range themeAlphas {
		out[snakeToCamel(t.Key)] = getThemeInt(t)
	}
	for _, t := range themeBlurs {
		out[snakeToCamel(t.Key)] = getThemeInt(t)
	}
	// Momo Talk 功能开关：前端据此决定是否渲染桃信入口
	out["momoTalkEnabled"] = getMomoTalkEnabled()
	// 全站字体：前端把它写进 --font-sans
	out["siteFont"] = getSiteFont()
	// 点击特效（ba-click-fx）的全部参数，前端拿去构造特效实例
	out["clickFx"] = getClickFX()
	c.JSON(http.StatusOK, out)
}

// publicKeyJSON 设置键 → 前端 JSON 字段名（home_bg → homeBg）。
func publicKeyJSON(key string) string {
	switch key {
	case "home_bg":
		return "homeBg"
	case "post_bg":
		return "postBg"
	case "notice":
		return "notice"
	case "hero_nickname":
		return "heroNickname"
	case "hero_desc":
		return "heroDesc"
	case "hero_roles":
		return "heroRoles"
	case "hero_avatar":
		return "heroAvatar"
	case "login_bg":
		return "loginBg"
	}
	return key
}

// settingKeyFromJSON JSON 字段名 → 设置键。
func settingKeyFromJSON(name string) string {
	switch name {
	case "homeBg":
		return "home_bg"
	case "postBg":
		return "post_bg"
	}
	return name
}

// safeBgURL 校验背景图 URL：空串（清空）或 http(s):// 开头，或以 / 开头的站内路径
// （如 /uploads/xxx.jpg）。其余一律拒绝，防止 javascript: 等危险值注入样式。
func safeBgURL(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return true
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "/") {
		return true
	}
	return false
}

// handleAdminUpdateHomeBg 管理员：保存首页背景图。PUT /api/admin/settings/home
// 请求体: {"homeBg": "图片URL 或 空串"}
func handleAdminUpdateHomeBg(c *gin.Context) {
	var req struct {
		HomeBg string `json:"homeBg"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.HomeBg = strings.TrimSpace(req.HomeBg)
	if !safeBgURL(req.HomeBg) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "背景图地址不合法：仅支持 http(s):// 或站内 /uploads 路径"})
		return
	}
	if err := setSetting(settingHomeBg, req.HomeBg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "homeBg": req.HomeBg})
}

// handleAdminUpdatePostBg 管理员：保存「文章页背景图」。
// PUT /api/admin/settings/postbg   请求体: {"postBg": "https://…"}（空串 = 清除）
//
// 与首页背景图是两个独立的设置：文章详情页用这一张，首页用 home_bg。
func handleAdminUpdatePostBg(c *gin.Context) {
	var req struct {
		PostBg string `json:"postBg"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.PostBg = strings.TrimSpace(req.PostBg)
	if !safeBgURL(req.PostBg) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "背景图地址不合法：仅支持 http(s):// 或站内 /uploads 路径"})
		return
	}
	if err := setSetting(settingPostBg, req.PostBg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "postBg": req.PostBg})
}

// migratePostBg 老库升级：「文章页背景图」是后加的设置项。
// 只在 post_bg 这一行**完全不存在**时，从 home_bg 复制一份 —— 这样升级完
// 文章页的观感与升级前一致，不会突然变成没有背景。
// 之后两者彻底独立：后台把文章页背景清空（写一个空串行）不会被这条重新覆盖，
// 因为 ON CONFLICT DO NOTHING 只在键缺失时插入。
func migratePostBg() error {
	_, err := q.Exec(
		`INSERT INTO settings ("key", value)
		 SELECT ?, value FROM settings WHERE "key" = ?
		 ON CONFLICT ("key") DO NOTHING`,
		settingPostBg, settingHomeBg,
	)
	return err
}

// handleAdminUpdateNotice 管理员：保存首页公告。PUT /api/admin/settings/notice
// 请求体: {"notice": "公告纯文本"}（传空串 = 清空公告）
// 只存纯文本、最多 noticeMaxRunes 个字，前端也只按纯文本渲染，不会当 HTML 解析。
func handleAdminUpdateNotice(c *gin.Context) {
	var req struct {
		Notice string `json:"notice"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Notice = strings.TrimSpace(req.Notice)
	if n := len([]rune(req.Notice)); n > noticeMaxRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "公告最多 300 字，请精简后再保存"})
		return
	}
	if err := setSetting("notice", req.Notice); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "notice": req.Notice})
}

// handleAdminUpdatePageSize 管理员：设置文章列表每页条数。PUT /api/admin/settings/pagesize
// 请求体: {"pageSize": 20}，取值 1~50（与 parsePage 的上限一致）。
func handleAdminUpdatePageSize(c *gin.Context) {
	var req struct {
		PageSize int `json:"pageSize"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if req.PageSize < 1 || req.PageSize > maxPageSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "每页条数需在 1~50 之间"})
		return
	}
	if err := setSetting("page_size", strconv.Itoa(req.PageSize)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "pageSize": req.PageSize})
}

// handleAdminUpdateTheme 管理员：外观定制（主题色 + 卡片透明度 + 面板透明度）。
// PUT /api/admin/settings/theme
// 请求体: {"accent":"#5BAE7C","cardAlpha":55,"panelAlpha":3}
// 透明度都是 0~100 的「底色浓度」，越小越透明。
func handleAdminUpdateTheme(c *gin.Context) {
	var req struct {
		Accent     string `json:"accent"`
		PanelColor string `json:"panelColor"`
		CardAlpha  int    `json:"cardAlpha"`
		CardBlur   int    `json:"cardBlur"`
		PostAlpha  int    `json:"postAlpha"`
		PostBlur   int    `json:"postBlur"`
		PanelAlpha int    `json:"panelAlpha"`
		PanelBlur  int    `json:"panelBlur"`
		NavAlpha   int    `json:"navAlpha"`
		NavBlur    int    `json:"navBlur"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Accent = strings.TrimSpace(req.Accent)
	req.PanelColor = strings.TrimSpace(req.PanelColor)
	if !validHexColor(req.Accent) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "主题色需为 #RRGGBB 或 #RGB 格式"})
		return
	}
	if !validHexColor(req.PanelColor) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "毛玻璃区域底色需为 #RRGGBB 或 #RGB 格式"})
		return
	}

	// 数值参数统一校验（浓度 0~100，模糊 0~50）
	nums := []struct {
		key string
		val int
		max int
	}{
		{"card_alpha", req.CardAlpha, 100},
		{"card_blur", req.CardBlur, 50},
		{"post_alpha", req.PostAlpha, 100},
		{"post_blur", req.PostBlur, 50},
		{"panel_alpha", req.PanelAlpha, 100},
		{"panel_blur", req.PanelBlur, 50},
		{"nav_alpha", req.NavAlpha, 100},
		{"nav_blur", req.NavBlur, 50},
	}
	for _, n := range nums {
		if n.val < 0 || n.val > n.max {
			c.JSON(http.StatusBadRequest, gin.H{"error": "数值超出范围：" + n.key})
			return
		}
	}

	if err := setSetting("accent", req.Accent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	if err := setSetting("panel_color", req.PanelColor); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	for _, n := range nums {
		if err := setSetting(n.key, strconv.Itoa(n.val)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
			return
		}
	}

	out := gin.H{"ok": true, "accent": req.Accent, "panelColor": req.PanelColor}
	for _, n := range nums {
		out[snakeToCamel(n.key)] = n.val
	}
	c.JSON(http.StatusOK, out)
}

// handleAdminUpdateMomoTalk 管理员：启用 / 关闭 Momo Talk。
// PUT /api/admin/settings/momotalk   请求体: {"enabled": true|false}
// 关闭后前端不再渲染任何桃信入口（导航栏按钮、评论区私信、小窗）。
func handleAdminUpdateMomoTalk(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	v := "true"
	if !req.Enabled {
		v = "false"
	}
	if err := setSetting(settingMomoTalk, v); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "momoTalkEnabled": req.Enabled})
}

// handleAdminUpdateFont 管理员：切换全站字体。
// PUT /api/admin/settings/font   请求体: {"siteFont": "system|rouhei|round"}
func handleAdminUpdateFont(c *gin.Context) {
	var req struct {
		SiteFont string `json:"siteFont"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.SiteFont = strings.TrimSpace(req.SiteFont)
	if !validSiteFont(req.SiteFont) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的字体标识"})
		return
	}
	if err := setSetting(settingSiteFont, req.SiteFont); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "siteFont": req.SiteFont})
}

// ============================================================
// 点击特效（ba-click-fx）
//
// 参数有九个、又是后台一个表单整体保存，所以**整体存成一条 JSON**（键 click_fx），
// 不像「外观定制」那样拆成十几个独立键 —— 拆开只会让读写两边都变长，也没有
// 单独读取某一个参数的需求（前端永远是整体重建特效实例）。
//
// 前端拿到后直接喂给 new BAClickFX({...})，字段名与库的选项名一一对应。
// ============================================================

const settingClickFX = "click_fx"

// 渲染后端：auto = 让库自己挑（WebGPU → WebGL2 → Canvas2D 逐级回退）。
const (
	clickFXBackendAuto   = "auto"
	clickFXBackendWebGL2 = "webgl2"
	clickFXBackendCanvas = "canvas2d"
)

// ClickFXSettings 点击特效的全部可调参数（JSON 的键名 = 前端的字段名）。
type ClickFXSettings struct {
	Enabled      bool    `json:"enabled"`      // 总开关：关掉后前端完全不加载特效代码
	ClickEnabled bool    `json:"clickEnabled"` // 点击时的波纹 / 碎片
	TrailEnabled bool    `json:"trailEnabled"` // 鼠标移动的拖尾
	TrailAlways  bool    `json:"trailAlways"`  // true = 不按鼠标也拖尾；false = 按住左键才拖尾
	Color        string  `json:"color"`        // 特效颜色（"" = 跟随站点主题色）
	Scale        float64 `json:"scale"`        // 尺寸倍率 0.2 ~ 2
	Opacity      float64 `json:"opacity"`      // 整体不透明度 0.05 ~ 1
	MaxDpr       float64 `json:"maxDpr"`       // 画布设备像素比上限 1 ~ 2（调低更省性能）
	Backend      string  `json:"backend"`      // auto / webgl2 / canvas2d
}

// defaultClickFX 默认值：特效默认**开启**（需求是「全局装好」），
// 拖尾默认「不按鼠标也出」—— 一进站移动鼠标就能看到效果。
func defaultClickFX() ClickFXSettings {
	return ClickFXSettings{
		Enabled:      true,
		ClickEnabled: true,
		TrailEnabled: true,
		TrailAlways:  true,
		Color:        "",
		Scale:        1,
		Opacity:      1,
		MaxDpr:       1,
		Backend:      clickFXBackendAuto,
	}
}

// clampFloat 把 v 收进 [lo, hi]；v 不是有限数（含 JSON 里的 null/字符串）时取 def。
func clampFloat(v, lo, hi, def float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// normalizeClickFX 把越界 / 非法的值收回合法范围。
// 读取和写入都走它，保证「存进去的」和「读出来的」口径一致。
func normalizeClickFX(in ClickFXSettings) ClickFXSettings {
	// 空串是合法的（跟随主题色）；非空但不是 #rgb/#rrggbb 的一律退回空串
	if in.Color != "" && !validHexColor(in.Color) {
		in.Color = ""
	}
	in.Scale = clampFloat(in.Scale, 0.2, 2, 1)
	in.Opacity = clampFloat(in.Opacity, 0.05, 1, 1)
	in.MaxDpr = clampFloat(in.MaxDpr, 1, 2, 1)
	switch in.Backend {
	case clickFXBackendAuto, clickFXBackendWebGL2, clickFXBackendCanvas:
	default:
		in.Backend = clickFXBackendAuto
	}
	return in
}

// getClickFX 读取点击特效设置。
// 没存过 / JSON 坏了都回落到默认值；缺字段由 json 反序列化保留结构体里的默认值。
func getClickFX() ClickFXSettings {
	out := defaultClickFX()
	raw := strings.TrimSpace(getSetting(settingClickFX))
	if raw == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return defaultClickFX()
	}
	return normalizeClickFX(out)
}

// handleAdminUpdateClickFX 管理员：保存点击特效设置。
// PUT /api/admin/settings/clickfx   请求体: ClickFXSettings 的 JSON
func handleAdminUpdateClickFX(c *gin.Context) {
	// 先灌默认值再反序列化：这样前端只传「改过的那几个字段」也能工作
	in := defaultClickFX()
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	in = normalizeClickFX(in)

	raw, err := json.Marshal(in)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	if err := setSetting(settingClickFX, string(raw)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "clickFx": in})
}
