package main

// ============================================================
// moments.go —— 说说（类似 Twitter 的短动态）
//
// 和文章的区别：
//   · 内容短，没有标题 / 分类 / 标签，只有一段文字 + 最多 9 张图；
//   · 不进博客列表、不进站内搜索的文章结果，独立成页（/moments）；
//   · 匿名访客也能点赞 / 记浏览量（用与文章点赞相同的 blog_visitor Cookie 去重）。
//
// 图片存成 JSON 数组字符串（不是逗号拼接）—— URL 里出现逗号的概率虽低，
// 但一旦出现就会把一张图拆成两张，JSON 没有这个问题。
// ============================================================

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// 单条说说的限制
const (
	momentMaxImages = 9    // 一条最多几张图
	momentMaxChars  = 2000 // 正文最多多少字
)

// MomentAuthor 说说作者的精简信息（够前端渲染头像和昵称即可）
type MomentAuthor struct {
	ID       int64  `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Role     string `json:"role"`
}

// Moment 一条说说
type Moment struct {
	ID        int64        `json:"id"`
	Content   string       `json:"content"`
	Images    []string     `json:"images"`
	Likes     int          `json:"likes"`
	Views     int          `json:"views"`
	Comments  int          `json:"comments"` // 评论数（列表里直接显示，点开才加载内容）
	Liked     bool         `json:"liked"`    // 当前访客是否已赞
	Author    MomentAuthor `json:"author"`
	CreatedAt time.Time    `json:"createdAt"`
}

func encodeImages(list []string) string {
	if len(list) == 0 {
		return "[]"
	}
	b, err := json.Marshal(list)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func decodeImages(raw string) []string {
	out := []string{}
	if raw == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return []string{}
	}
	return out
}

// cleanImages 只接受站内 /uploads/ 或 http(s) 图片地址，并限制张数。
// 前端上传后会拿到 /uploads/xxx.webp 这样的路径，直接存下来。
func cleanImages(in []string) []string {
	out := make([]string, 0, len(in))
	for _, u := range in {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if strings.HasPrefix(u, "/uploads/") || strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			out = append(out, u)
		}
		if len(out) >= momentMaxImages {
			break
		}
	}
	return out
}

// momentSelect 列表 / 详情共用的列。liked 由 EXISTS 子查询算，占第 1 个参数。
// 昵称回退顺序与 displayNick 一致：昵称 → 邮箱前缀 → 「用户」。
const momentSelect = `SELECT m.id, m.content, m.images, m.likes, m.views,
	(SELECT COUNT(*) FROM moment_comments mc WHERE mc.moment_id = m.id) AS comments,
	m.created_at,
	COALESCE(u.id, 0),
	COALESCE(NULLIF(u.nickname, ''), NULLIF(split_part(COALESCE(u.email, ''), '@', 1), ''), '用户'),
	COALESCE(u.avatar, ''), COALESCE(u.role, 'user'),
	EXISTS(SELECT 1 FROM moment_likes ml WHERE ml.moment_id = m.id AND ml.visitor = ?)
	FROM moments m LEFT JOIN users u ON u.id = m.user_id`

func scanMoment(row interface{ Scan(...any) error }) (Moment, error) {
	var m Moment
	var rawImages string
	if err := row.Scan(&m.ID, &m.Content, &rawImages, &m.Likes, &m.Views, &m.Comments, &m.CreatedAt,
		&m.Author.ID, &m.Author.Nickname, &m.Author.Avatar, &m.Author.Role, &m.Liked); err != nil {
		return m, err
	}
	m.Images = decodeImages(rawImages)
	if strings.TrimSpace(m.Author.Nickname) == "" {
		m.Author.Nickname = "匿名"
	}
	return m, nil
}

// handleListMoments GET /api/moments?page=1&pageSize=10
// 公开接口：未登录也能看，liked 对未登录访客用 Cookie 判定。
func handleListMoments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if size > 30 {
		size = 30
	}
	visitor := visitorID(c)

	var total int
	if err := q.QueryRow(`SELECT COUNT(*) FROM moments`).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	rows, err := q.Query(momentSelect+` ORDER BY m.created_at DESC, m.id DESC LIMIT ? OFFSET ?`,
		visitor, size, (page-1)*size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()

	items := make([]Moment, 0, size)
	for rows.Next() {
		m, err := scanMoment(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
			return
		}
		items = append(items, m)
	}

	c.JSON(http.StatusOK, gin.H{
		"items":   items,
		"total":   total,
		"page":    page,
		"hasMore": page*size < total,
	})
}

// handleCreateMoment POST /api/user/moments（需登录）
func handleCreateMoment(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	var in struct {
		Content string   `json:"content"`
		Images  []string `json:"images"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	content := strings.TrimSpace(in.Content)
	images := cleanImages(in.Images)
	if content == "" && len(images) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "写点什么，或者选一张图"})
		return
	}
	if utf8.RuneCountInString(content) > momentMaxChars {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容太长了，最多 2000 字"})
		return
	}

	var id int64
	if err := q.QueryRow(
		`INSERT INTO moments (user_id, content, images) VALUES (?, ?, ?) RETURNING id`,
		me.ID, content, encodeImages(images)).Scan(&id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发布失败"})
		return
	}

	m, err := scanMoment(q.QueryRow(momentSelect+` WHERE m.id = ?`, visitorID(c), id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发布成功，但读取失败"})
		return
	}
	c.JSON(http.StatusOK, m)
}

// handleDeleteMoment DELETE /api/user/moments/:id（本人或管理员）
func handleDeleteMoment(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的 ID"})
		return
	}

	var owner int64
	if err := q.QueryRow(`SELECT user_id FROM moments WHERE id = ?`, id).Scan(&owner); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "这条说说不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	if owner != me.ID && me.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能删除自己发布的说说"})
		return
	}

	if _, err := q.Exec(`DELETE FROM moments WHERE id = ?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	// 点赞 / 浏览记录一起清掉（这两张表没有外键，得手动删）
	_, _ = q.Exec(`DELETE FROM moment_likes WHERE moment_id = ?`, id)
	_, _ = q.Exec(`DELETE FROM moment_views WHERE moment_id = ?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleLikeMoment POST /api/moments/:id/like
// 与文章点赞同一套语义：匿名、同一访客只能赞一次，再点一次取消。
func handleLikeMoment(c *gin.Context) {
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

	visitor := visitorID(c)
	var n int
	if err := q.QueryRow(`SELECT COUNT(*) FROM moment_likes WHERE moment_id = ? AND visitor = ?`,
		id, visitor).Scan(&n); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	liked := n == 0
	if liked {
		if _, err := q.Exec(`INSERT INTO moment_likes (moment_id, visitor) VALUES (?, ?) ON CONFLICT DO NOTHING`,
			id, visitor); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "点赞失败"})
			return
		}
		_, _ = q.Exec(`UPDATE moments SET likes = likes + 1 WHERE id = ?`, id)
	} else {
		if _, err := q.Exec(`DELETE FROM moment_likes WHERE moment_id = ? AND visitor = ?`, id, visitor); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "取消点赞失败"})
			return
		}
		_, _ = q.Exec(`UPDATE moments SET likes = GREATEST(likes - 1, 0) WHERE id = ?`, id)
	}

	var likes int
	_ = q.QueryRow(`SELECT likes FROM moments WHERE id = ?`, id).Scan(&likes)
	c.JSON(http.StatusOK, gin.H{"liked": liked, "likes": likes})
}

// handleViewMoment POST /api/moments/:id/view
// 浏览量：同一访客只计一次（Cookie 去重），刷新页面不会把数字刷上去。
func handleViewMoment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的 ID"})
		return
	}
	visitor := visitorID(c)

	if res, err := q.Exec(
		`INSERT INTO moment_views (moment_id, visitor) VALUES (?, ?) ON CONFLICT DO NOTHING`,
		id, visitor); err == nil {
		if n, _ := res.RowsAffected(); n > 0 {
			_, _ = q.Exec(`UPDATE moments SET views = views + 1 WHERE id = ?`, id)
		}
	}

	var views int
	_ = q.QueryRow(`SELECT views FROM moments WHERE id = ?`, id).Scan(&views)
	c.JSON(http.StatusOK, gin.H{"views": views})
}
