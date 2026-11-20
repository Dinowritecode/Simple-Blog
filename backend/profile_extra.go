package main

// ============================================================
// profile_extra.go —— 站长资料扩展字段 + 每日登录经验
//
// 资料字段（都在 users 表上，全部可空）：
//   birthday  出生日期（YYYY-MM-DD）—— 星座、年龄都由它算出来，不单独存
//   gender    性别（自由文本，如「男」「女」「保密」）
//   github    GitHub 主页
//   contact   联系方式（QQ / 微信 / Telegram…随便写）
//   bio       资料卡上的一句话简介
//
// 每日登录经验：
//   每天第一次登录 +10 经验，记进 checkins（一天一条，不会重复加）。
//   经验就是 points，等级由 points 换算（见 level.go）。
// ============================================================

import (
	"fmt"
	"strings"
	"time"
)

// dailyLoginPoints 每日登录获得的经验值
const dailyLoginPoints = 10

// extraProfile 用户资料的额外字段（内嵌进 User）
type extraProfile struct {
	Birthday string `json:"birthday"` // YYYY-MM-DD，空 = 未设置
	Gender   string `json:"gender"`
	GitHub   string `json:"github"`
	Contact  string `json:"contact"`
	Bio      string `json:"bio"`
	// 派生字段（只读，由 birthday 算出）
	Zodiac string `json:"zodiac"` // 星座
	Age    int    `json:"age"`    // 周岁
}

// zodiacOf 按出生月日算星座
func zodiacOf(t time.Time) string {
	// 每个星座的起始日；用「日」和「月*100+日」比较，避免跨月麻烦
	m, d := int(t.Month()), t.Day()
	key := m*100 + d
	switch {
	case key >= 1222 || key <= 119:
		return "摩羯座"
	case key <= 218:
		return "水瓶座"
	case key <= 320:
		return "双鱼座"
	case key <= 419:
		return "白羊座"
	case key <= 520:
		return "金牛座"
	case key <= 621:
		return "双子座"
	case key <= 722:
		return "巨蟹座"
	case key <= 822:
		return "狮子座"
	case key <= 922:
		return "处女座"
	case key <= 1023:
		return "天秤座"
	case key <= 1122:
		return "天蝎座"
	case key <= 1221:
		return "射手座"
	}
	return "摩羯座"
}

// ageOf 周岁：今年生日还没过就减一岁
func ageOf(birth, now time.Time) int {
	age := now.Year() - birth.Year()
	if now.Month() < birth.Month() || (now.Month() == birth.Month() && now.Day() < birth.Day()) {
		age--
	}
	if age < 0 {
		age = 0
	}
	return age
}

// fillDerived 把 birthday 解析成星座和年龄
func (e *extraProfile) fillDerived(now time.Time) {
	e.Zodiac, e.Age = "", 0
	if e.Birthday == "" {
		return
	}
	t, err := time.Parse("2006-01-02", e.Birthday)
	if err != nil {
		return
	}
	e.Zodiac = zodiacOf(t)
	e.Age = ageOf(t, now)
}

// fillUserDerived 给 User 补上星座 / 年龄（查询后调用一次即可）
func fillUserDerived(u *User) {
	if u == nil {
		return
	}
	u.fillDerived(time.Now())
}

// normalizeBirthday 校验并规范化日期字符串
func normalizeBirthday(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return "", fmt.Errorf("出生日期格式应为 YYYY-MM-DD")
	}
	if t.Year() < 1900 || t.After(time.Now()) {
		return "", fmt.Errorf("出生日期看起来不太对")
	}
	return t.Format("2006-01-02"), nil
}

// ---------- 每日登录经验 ----------

// awardDailyLogin 每天第一次登录 +10 经验。
// 幂等：同一天重复调用不会重复加（靠 checkins 的主键冲突）。
// 返回本次是否真的加了经验，以及最新的 points。
func awardDailyLogin(userID int64) (awarded bool, points int, err error) {
	day := time.Now().Format("2006-01-02")
	res, err := q.Exec(
		`INSERT INTO checkins (user_id, check_date, points) VALUES (?, ?, ?)
		 ON CONFLICT (user_id, check_date) DO NOTHING`, userID, day, dailyLoginPoints)
	if err != nil {
		return false, 0, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		awarded = true
		if _, err := q.Exec(`UPDATE users SET points = COALESCE(points,0) + ? WHERE id = ?`, dailyLoginPoints, userID); err != nil {
			return awarded, 0, err
		}
	}
	if err := q.QueryRow(`SELECT COALESCE(points,0) FROM users WHERE id = ?`, userID).Scan(&points); err != nil {
		return awarded, 0, err
	}
	// 等级由积分换算，写回 users.level 供全站展示
	if _, err := q.Exec(`UPDATE users SET level = ? WHERE id = ?`, levelFromPoints(points), userID); err != nil {
		return awarded, points, err
	}
	return awarded, points, nil
}

// migrateProfileFields 老库补「站长资料」相关列（新库建表时就带了，这里是空操作）。
func migrateProfileFields() error {
	stmts := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS points   INTEGER    NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS birthday DATE`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS gender   VARCHAR(8) NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS github   VARCHAR(190) NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS contact  VARCHAR(190) NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS bio      TEXT NOT NULL DEFAULT ''`,
	}
	for _, s := range stmts {
		if _, err := q.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
