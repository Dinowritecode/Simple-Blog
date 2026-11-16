package main

// ============================================================
// level.go —— 用户等级系统 + 每日签到
//
// 规则（当前版本）：
//   · 满级 10 级，每级 50 积分 —— 0 分 = 1 级，450 分 = 10 级（满级）
//   · 唯一的积分来源是**每日签到**，每次 +5 分（一天一次，签满 90 天满级）
//
// 为什么等级是从积分**算出来**的，而不是直接加一：
//   · 只有一个真值来源（points），等级是派生值，不会出现「积分和等级对不上」
//   · 以后要改规则（比如每级 60 分、或者加发帖得分），只动 levelFromPoints 一处，
//     已有的积分不用迁移
//   之所以仍然把算出来的等级写回 users.level：全站的等级展示（评论区、桃信列表、
//   资料卡）现在都直接读这一列，写回去就不用挨个改成实时计算。
//
// 接口：
//   GET  /api/user/checkin   今日签到状态 + 等级进度 + 本月签到日历
//   POST /api/user/checkin   执行签到（幂等：同一天重复调用不会重复加分）
// ============================================================

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// maxLevel 满级。
	maxLevel = 10
	// pointsPerLevel 升一级需要的积分。
	pointsPerLevel = 50
	// checkinPoints 每次签到获得的积分。
	checkinPoints = 5
)

// levelFromPoints 由累计积分算出等级：0 分 = 1 级，每满 50 分升一级，封顶 maxLevel。
func levelFromPoints(points int) int {
	if points < 0 {
		points = 0
	}
	lv := points/pointsPerLevel + 1
	if lv > maxLevel {
		return maxLevel
	}
	return lv
}

// pointsToNextLevel 距离下一级还差多少积分；已满级返回 0。
func pointsToNextLevel(points int) int {
	if levelFromPoints(points) >= maxLevel {
		return 0
	}
	// 例如 120 分 → 下一级要 150 分 → 还差 30
	need := levelFromPoints(points) * pointsPerLevel
	return need - points
}

// CheckinStatus 签到面板需要的全部数据。
type CheckinStatus struct {
	Points         int      `json:"points"`         // 累计积分
	Level          int      `json:"level"`          // 当前等级
	MaxLevel       int      `json:"maxLevel"`       // 满级
	PointsPerLevel int      `json:"pointsPerLevel"` // 每级所需积分
	CheckinPoints  int      `json:"checkinPoints"`  // 每次签到得分
	ToNextLevel    int      `json:"toNextLevel"`    // 距下一级还差多少分（满级 0）
	LevelPoints    int      `json:"levelPoints"`    // 当前等级已积累的积分（用于画进度条）
	CheckedToday   bool     `json:"checkedToday"`   // 今天是否已签到
	TotalDays      int      `json:"totalDays"`      // 累计签到天数
	Streak         int      `json:"streak"`         // 连续签到天数
	MonthDays      []string `json:"monthDays"`      // 本月已签到的日期（YYYY-MM-DD）
	Today          string   `json:"today"`          // 服务器认定的今天（便于前端对齐）
}

// buildCheckinStatus 组装状态。
// 参数都是已经查好的值，纯计算 + 一次查询（本月签到日期），便于复用。
func buildCheckinStatus(userID int64, points int, today time.Time) (*CheckinStatus, error) {
	lv := levelFromPoints(points)
	st := &CheckinStatus{
		Points:         points,
		Level:          lv,
		MaxLevel:       maxLevel,
		PointsPerLevel: pointsPerLevel,
		CheckinPoints:  checkinPoints,
		ToNextLevel:    pointsToNextLevel(points),
		// 进度条口径：当前等级内已经攒了多少分（满级时把整条填满）
		LevelPoints: points % pointsPerLevel,
		Today:       today.Format("2006-01-02"),
		MonthDays:   []string{},
	}
	if lv >= maxLevel {
		st.LevelPoints = pointsPerLevel
	}

	day := today.Format("2006-01-02")

	// 累计天数
	if err := q.QueryRow(`SELECT COUNT(*) FROM checkins WHERE user_id = ?`, userID).Scan(&st.TotalDays); err != nil {
		return nil, err
	}
	// 本月已签到的日子（画日历用）
	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	nextMonth := monthStart.AddDate(0, 1, 0)
	rows, err := q.Query(`
		SELECT TO_CHAR(check_date, 'YYYY-MM-DD') FROM checkins
		WHERE user_id = ? AND check_date >= ? AND check_date < ?
		ORDER BY check_date`, userID, monthStart.Format("2006-01-02"), nextMonth.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		st.MonthDays = append(st.MonthDays, d)
		if d == day {
			st.CheckedToday = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 连续天数：从今天（没签就是昨天）往前数，遇到断档就停。
	// 只取最近 maxLevel*pointsPerLevel 天够用了，不必扫全表。
	st.Streak = calcStreak(userID, today, st.CheckedToday)
	return st, nil
}

// calcStreak 计算连续签到天数。
// checkedToday=false 时从昨天开始数（今天还没签不该把之前的连签清掉）。
func calcStreak(userID int64, today time.Time, checkedToday bool) int {
	rows, err := q.Query(`
		SELECT TO_CHAR(check_date, 'YYYY-MM-DD') FROM checkins
		WHERE user_id = ? AND check_date <= ?
		ORDER BY check_date DESC LIMIT 400`,
		userID, today.Format("2006-01-02"))
	if err != nil {
		return 0
	}
	defer rows.Close()

	days := map[string]bool{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return 0
		}
		days[d] = true
	}

	// 从哪天开始往回数：签了就今天，没签就昨天
	cursor := today
	if !checkedToday {
		cursor = today.AddDate(0, 0, -1)
	}
	streak := 0
	for i := 0; i < 400; i++ {
		if !days[cursor.Format("2006-01-02")] {
			break
		}
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return streak
}

// loadPoints 读某人的累计积分。
func loadPoints(userID int64) (int, error) {
	var points int
	err := q.QueryRow(`SELECT COALESCE(points, 0) FROM users WHERE id = ?`, userID).Scan(&points)
	return points, err
}

// ------------------------------------------------------------
// 接口
// ------------------------------------------------------------

// handleCheckinStatus 登录用户：查询签到状态。GET /api/user/checkin
func handleCheckinStatus(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	points, err := loadPoints(me.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	st, err := buildCheckinStatus(me.ID, points, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, st)
}

// handleCheckin 登录用户：每日签到。POST /api/user/checkin
//
// 「一天一次」由数据库主键 (user_id, check_date) 兜底：
//
//	INSERT ... ON CONFLICT DO NOTHING 的 RowsAffected 是 0 就说明今天已经签过，
//	此时直接返回当前状态、不加分。所以并发/重复点击都是安全的。
//
// 加分的 UPDATE 把「积分」和「等级」放在同一条语句里算：
// PostgreSQL 的 SET 里所有表达式都用**改之前**的 points，
// 所以 level 也是按「老积分 + 5」算的，两步不会错位。
func handleCheckin(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	now := time.Now()
	day := now.Format("2006-01-02")

	res, err := q.Exec(
		`INSERT INTO checkins (user_id, check_date, points) VALUES (?, ?, ?)
		 ON CONFLICT (user_id, check_date) DO NOTHING`,
		me.ID, day, checkinPoints,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "签到失败"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		// 今天已经签过：返回现状，不报错（前端重复点也不会弹错误）
		points, _ := loadPoints(me.ID)
		st, err := buildCheckinStatus(me.ID, points, now)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "already": true, "gained": 0, "status": st})
		return
	}

	// 加分 + 同步等级（等级是全站展示读的那一列，必须一起更新）
	var points, level int
	err = q.QueryRow(
		`UPDATE users
		 SET points = points + ?,
		     level  = LEAST(?, (points + ?) / ? + 1)
		 WHERE id = ?
		 RETURNING points, level`,
		checkinPoints, maxLevel, checkinPoints, pointsPerLevel, me.ID,
	).Scan(&points, &level)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加分失败"})
		return
	}

	// 升级判定：按「签到前的等级」比。签到前的积分 = 现在的积分 - 本次得分。
	levelUp := levelFromPoints(points-checkinPoints) < level

	st, err := buildCheckinStatus(me.ID, points, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"already": false,
		"gained":  checkinPoints,
		"levelUp": levelUp,
		"status":  st,
	})
}
