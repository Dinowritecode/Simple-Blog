package main

// ============================================================
// profile.go —— 公开的用户个人主页
//
//   GET /api/users/:id/profile   任何人可看：基本信息 + 数据统计 + 他的说说
//
// 和 /api/users/:id（桃信资料卡，需登录、带好友关系）不是一回事：
// 这里是给「点别人头像/昵称进来看看」用的公开页，所以
//   · 只暴露公开字段（昵称 / 头像 / 9 位 ID / 等级 / 注册时间 / 角色），**不含邮箱**；
//   · 附带他发过的说说（最多 20 条）与三个统计数字。
// ============================================================

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ProfileUser 个人主页展示的用户信息（白名单字段，别把 email 混进来）
type ProfileUser struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"userId"`
	Nickname  string    `json:"nickname"`
	Avatar    string    `json:"avatar"`
	Cover     string    `json:"cover"` // 主页封面图（空串 = 前端退回站点首页背景）
	Level     int       `json:"level"`
	Points    int       `json:"points"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

// ActivityDay 热力图的一格：某一天有多少次活动
type ActivityDay struct {
	Date  string `json:"date"` // YYYY-MM-DD
	Count int    `json:"count"`
}

// activityDays 热力图的时间跨度：53 周 × 7 天 = 371 天（和 GitHub 一屏差不多）
const activityDays = 371

// handleGetUserProfile GET /api/users/:id/profile
func handleGetUserProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的用户 ID"})
		return
	}

	var u ProfileUser
	err = q.QueryRow(
		`SELECT id, COALESCE(user_id, ''), COALESCE(NULLIF(nickname, ''), NULLIF(split_part(COALESCE(email, ''), '@', 1), ''), '用户'),
		        COALESCE(avatar, ''), COALESCE(cover, ''), COALESCE(level, 1), COALESCE(points, 0), COALESCE(role, 'user'), created_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.UserID, &u.Nickname, &u.Avatar, &u.Cover, &u.Level, &u.Points, &u.Role, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "这个用户不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 三个统计数字：说说数 / 说说获得的赞 / 写过的评论（文章 + 说说）
	var moments, likes, comments int
	_ = q.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(likes), 0) FROM moments WHERE user_id = ?`, id,
	).Scan(&moments, &likes)
	_ = q.QueryRow(
		`SELECT (SELECT COUNT(*) FROM comments WHERE user_id = ?)
		      + (SELECT COUNT(*) FROM moment_comments WHERE user_id = ?)`, id, id,
	).Scan(&comments)

	// 他最近的说说（与首页同一套列，liked 仍按当前访客判定）
	rows, err := q.Query(
		momentSelect+` WHERE m.user_id = ? ORDER BY m.created_at DESC, m.id DESC LIMIT 20`,
		visitorID(c), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()

	list := make([]Moment, 0, 20)
	for rows.Next() {
		m, err := scanMoment(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
			return
		}
		list = append(list, m)
	}

	c.JSON(http.StatusOK, gin.H{
		"user": u,
		"stats": gin.H{
			"moments":  moments,
			"likes":    likes,
			"comments": comments,
		},
		"activity": loadActivity(id),
		"moments":  list,
	})
}

// loadActivity 汇总某人最近 371 天的活跃度（说说 + 说说评论 + 文章评论），
// 前端用它画 GitHub 那种热力图。没有活动的那天也补 0，否则前端要自己补空。
func loadActivity(userID int64) []ActivityDay {
	counts := map[string]int{}
	add := func(sql string) {
		rows, err := q.Query(sql, userID)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var day string
			var n int
			if rows.Scan(&day, &n) == nil {
				counts[day] += n
			}
		}
	}
	// created_at 是 timestamp，按天截断即可（服务器与数据库同为本地时区）
	add(`SELECT to_char(created_at, 'YYYY-MM-DD'), COUNT(*) FROM moments
	     WHERE user_id = ? AND created_at >= CURRENT_DATE - INTERVAL '370 days' GROUP BY 1`)
	add(`SELECT to_char(created_at, 'YYYY-MM-DD'), COUNT(*) FROM moment_comments
	     WHERE user_id = ? AND created_at >= CURRENT_DATE - INTERVAL '370 days' GROUP BY 1`)
	add(`SELECT to_char(created_at, 'YYYY-MM-DD'), COUNT(*) FROM comments
	     WHERE user_id = ? AND created_at >= CURRENT_DATE - INTERVAL '370 days' GROUP BY 1`)

	out := make([]ActivityDay, 0, activityDays)
	today := time.Now()
	for i := activityDays - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, ActivityDay{Date: d, Count: counts[d]})
	}
	return out
}
