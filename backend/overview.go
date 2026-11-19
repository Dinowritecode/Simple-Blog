package main

// ============================================================
// overview.go —— 后台「总览」页的数据
//
//   POST /api/visit          记录一次页面访问（前端每次路由切换调一次）
//   GET  /api/admin/overview 用户/文章/说说/评论数 + 主机性能 + 近 14 天访问量
//
// 「主机性能」在各平台差异很大：
//   · Linux：读 /proc 拿 CPU/内存，statfs 拿磁盘（见 hostmetrics_linux.go）
//   · 其它：返回 0，前端显示「—」，不影响页面其它部分
// ============================================================

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ---------- 访问统计 ----------

// trackVisit 记录一次访问：当天 views+1；同一个 visitor 当天只算一次 UV。
// 前端在每次路由切换时调用（后台页面不计）。
// 访客标识复用点赞那套 visitorID()（同一个 blog_visitor Cookie），不另起一套。
func trackVisit(c *gin.Context) {
	day := time.Now().Format("2006-01-02")
	visitor := visitorID(c)

	if _, err := q.Exec(
		`INSERT INTO site_visits (day, views, visitors) VALUES (?, 1, 0)
		 ON CONFLICT (day) DO UPDATE SET views = site_visits.views + 1`, day); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false})
		return
	}

	// ON CONFLICT DO NOTHING 的 RowsAffected 为 0 表示今天已经来过，不重复计 UV
	res, err := q.Exec(`INSERT INTO visit_days (day, visitor) VALUES (?, ?) ON CONFLICT DO NOTHING`, day, visitor)
	if err == nil {
		if n, _ := res.RowsAffected(); n > 0 {
			_, _ = q.Exec(`UPDATE site_visits SET visitors = visitors + 1 WHERE day = ?`, day)
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- 总览数据 ----------

type overviewTraffic struct {
	Day      string `json:"day"`
	Views    int64  `json:"views"`
	Visitors int64  `json:"visitors"`
}

// handleAdminOverview 后台总览：数字卡片 + 主机性能 + 近 14 天访问量。GET /api/admin/overview
func handleAdminOverview(c *gin.Context) {
	counts := gin.H{}
	var err error
	countOf := func(sql string) int64 {
		var n int64
		if e := q.QueryRow(sql).Scan(&n); e != nil {
			err = e
		}
		return n
	}
	counts["users"] = countOf(`SELECT COUNT(*) FROM users`)
	counts["posts"] = countOf(`SELECT COUNT(*) FROM posts WHERE status = 'published'`)
	counts["moments"] = countOf(`SELECT COUNT(*) FROM moments`)
	counts["comments"] = countOf(`SELECT COUNT(*) FROM comments`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "统计失败"})
		return
	}

	// 近 14 天访问量：先查出有数据的，再补齐没数据的日期（折线图要连续）
	traffic := make([]overviewTraffic, 0, 14)
	today := time.Now()
	byDay := map[string]overviewTraffic{}
	rows, qerr := q.Query(`SELECT day, views, visitors FROM site_visits WHERE day >= ?`,
		today.AddDate(0, 0, -13).Format("2006-01-02"))
	if qerr == nil {
		defer rows.Close()
		for rows.Next() {
			var t overviewTraffic
			if rows.Scan(&t.Day, &t.Views, &t.Visitors) == nil {
				byDay[t.Day] = t
			}
		}
	}
	for i := 13; i >= 0; i-- {
		d := today.AddDate(0, 0, -i).Format("2006-01-02")
		if t, ok := byDay[d]; ok {
			traffic = append(traffic, t)
		} else {
			traffic = append(traffic, overviewTraffic{Day: d})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"counts":  counts,
		"host":    hostMetrics(),
		"traffic": traffic,
	})
}

// ---------- 主机性能 ----------

type hostStatus struct {
	Load1      float64 `json:"load1"`      // 1 分钟平均负载
	CPUCount   int     `json:"cpuCount"`   // 逻辑核数
	CPUPercent float64 `json:"cpuPercent"` // 0-100
	MemUsed    float64 `json:"memUsed"`    // GB
	MemTotal   float64 `json:"memTotal"`   // GB
	DiskUsed   float64 `json:"diskUsed"`   // GB
	DiskTotal  float64 `json:"diskTotal"`  // GB
	// 逐项可用性：Windows 没有 loadavg，只有负载这一项是 false，
	// 其余三项照常给真实值（前端按项决定显示数字还是「—」）。
	Supported bool `json:"supported"`
	LoadOK    bool `json:"loadOK"`
	CPUOK     bool `json:"cpuOK"`
	MemOK     bool `json:"memOK"`
	DiskOK    bool `json:"diskOK"`
}

// 采样缓存：CPU 百分比要两次采样才有意义，10 秒内复用，避免每次开页面都等
var (
	hostCache   hostStatus
	hostCacheAt time.Time
)

func hostMetrics() hostStatus {
	if time.Since(hostCacheAt) < 10*time.Second {
		return hostCache
	}
	hostCache = readHostMetrics()
	hostCacheAt = time.Now()
	return hostCache
}

// 给前端用的四舍五入（保留两位小数）
func round2(v float64) float64 {
	f, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', 2, 64), 64)
	return f
}
