package main

// ============================================================
// db.go —— 数据库模块（PostgreSQL）
//
// 全站只有这一个数据库了：用户、文章索引、评论、通知、点赞、好友、
// 会话与消息都在 PostgreSQL。聊天三张表由 chatstore 的 PG 实现负责建，
// 但它们在同一个库里，所以这里把九张表一起描述清楚。
//
// 设计说明：
//   - 通过 pgx 的 database/sql 适配层使用标准库 database/sql，
//     所以旧代码里的 db.Query / db.QueryRow / db.Exec 调用方式不变。
//   - SQL 里仍然写 MySQL 风格的 `?` 占位符，由 pgQuery 统一翻译成 PostgreSQL 的 $1/$2…
//     这样迁移时不必逐个改写 90 处查询，也避免手工编号出错。
//   - 启动时自动 CREATE TABLE IF NOT EXISTS，所以「不需要手动建表」。
// ============================================================

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL 驱动（注册驱动名为 "pgx"）
)

// db 全局数据库连接池（database/sql 自带连接池管理，可直接并发使用）
var db *sql.DB

// DBConfig PostgreSQL 连接配置，全部来自环境变量（main.go 中组装）。
// 对应环境变量：DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME
type DBConfig struct {
	Host     string // 地址，默认 127.0.0.1
	Port     string // 端口，默认 5432
	User     string // 用户名，默认 postgres
	Password string // 密码，默认空
	Name     string // 数据库名，默认 blog
	SSLMode  string // sslmode，默认 disable（本机/内网部署）
}

// dsn 构造 pgx 的连接串（URL 形式）。
func (c DBConfig) dsn() string {
	port := c.Port
	if port == "" {
		port = "5432"
	}
	user := c.User
	if user == "" {
		user = "postgres"
	}
	name := c.Name
	if name == "" {
		name = "blog"
	}
	ssl := c.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	auth := user
	if c.Password != "" {
		auth = user + ":" + c.Password
	}
	return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=%s", auth, c.Host, port, name, ssl)
}

// getenv 读环境变量，未设置或为空时返回默认值。
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// pgQuery 把 SQL 里的 `?` 占位符依次翻译成 PostgreSQL 的 $1、$2 …
// 为什么保留 `?` 而不是把 90 处查询都改成 $N：
//
//	· 手工编号很容易错（同一语句里加一个参数就要重排后面所有编号）；
//	· `?` 在单引号字符串里不出现（本项目的 SQL 没有含 `?` 的字符串字面量），
//	  但仍做了引号状态判断以策安全。
func pgQuery(sql string) string {
	if !strings.Contains(sql, "?") {
		return sql
	}
	var b strings.Builder
	b.Grow(len(sql) + 16)
	n := 0
	inSingle := false
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		switch {
		case c == '\'':
			// 处理 '' 转义（SQL 标准里两个单引号表示一个引号字符）
			if inSingle && i+1 < len(sql) && sql[i+1] == '\'' {
				b.WriteByte(c)
				b.WriteByte(sql[i+1])
				i++
				continue
			}
			inSingle = !inSingle
			b.WriteByte(c)
		case c == '?' && !inSingle:
			n++
			fmt.Fprintf(&b, "$%d", n)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// initDB 初始化数据库：连接 → 校验连通 → 建表（全部幂等）。
func initDB(cfg DBConfig) error {
	var err error
	db, err = sql.Open("pgx", cfg.dsn())
	if err != nil {
		return fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}
	// 连接池参数：最多 10 个连接、5 个空闲、单连接最长 30 分钟
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.Ping(); err != nil {
		return fmt.Errorf("连接 PostgreSQL 失败（库 %s 是否已创建？）: %w", cfg.Name, err)
	}
	// 让薄封装 q 复用同一个连接池（见 pgdb.go）
	q = pgDB{db}

	// ---------- ③ 建表（schema 与 docs 在线文档中的「数据库设计」一致） ----------
	// 全部 CREATE TABLE IF NOT EXISTS，重启不会出错。
	// 聊天三张表（conversations / conversation_members / messages）由
	// chatstore 的 EnsureSchema 负责建成分区表，这里不重复定义。
	schema := []string{
		// users 表：所有账户（管理员 + 注册用户）
		`CREATE TABLE IF NOT EXISTS users (
			id            BIGSERIAL PRIMARY KEY,
			username      VARCHAR(64),                                  -- 用户名（管理员登录用；注册用户为 NULL）
			email         VARCHAR(190),                                 -- 邮箱（注册用户登录用，唯一）
			password_hash VARCHAR(255) NOT NULL,                        -- bcrypt 加密后的密码
			nickname      VARCHAR(64)  NOT NULL DEFAULT '',             -- 昵称（评论区展示）
			avatar        VARCHAR(255) NOT NULL DEFAULT '',             -- 头像 URL（/uploads/avatars/...）
			role          VARCHAR(16)  NOT NULL DEFAULT 'user',         -- 角色：admin / user
			user_id       VARCHAR(16)  NOT NULL DEFAULT '',             -- 9 位桃信 ID
			level         INTEGER      NOT NULL DEFAULT 1,              -- 等级（由 points 换算）
			points        INTEGER      NOT NULL DEFAULT 0,              -- 累计经验
			-- 站长资料（全部可空，星座/年龄由 birthday 现算，不存）
			birthday      DATE,
			gender        VARCHAR(8)   NOT NULL DEFAULT '',
			github        VARCHAR(190) NOT NULL DEFAULT '',
			contact       VARCHAR(190) NOT NULL DEFAULT '',
			bio           TEXT         NOT NULL DEFAULT '',
			created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uk_users_email    UNIQUE (email),
			CONSTRAINT uk_users_username UNIQUE (username),
			CONSTRAINT uk_users_user_id  UNIQUE (user_id)
		)`,
		// posts 表：博客文章的「索引」部分。
		// 正文 content 已迁到文件系统（见 postcontent 包），这里只留列表/筛选用的轻量列；
		// content 列保留为可空，仅作为「未迁移时的兜底」与历史数据备份。
		`CREATE TABLE IF NOT EXISTS posts (
			id         BIGSERIAL PRIMARY KEY,
			slug       VARCHAR(32),                                  -- 公开链接随机短码（/blog/<slug>）
			title      VARCHAR(255) NOT NULL,                        -- 标题
			summary    TEXT         NOT NULL,                        -- 摘要（列表页展示）
			cover      TEXT         NOT NULL,                        -- 封面图 URL
			category   VARCHAR(64)  NOT NULL DEFAULT '',             -- 分类（单选；空串 = 未分类）
			tags       VARCHAR(255) NOT NULL DEFAULT '',             -- 标签（逗号分隔）
			content    TEXT,                                         -- 正文（NULL = 在文件里）
			status     VARCHAR(16)  NOT NULL DEFAULT 'draft',        -- draft / published
			likes      INTEGER      NOT NULL DEFAULT 0,              -- 点赞数
			views      INTEGER      NOT NULL DEFAULT 0,              -- 浏览量
			created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uk_posts_slug UNIQUE (slug)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_posts_status_created ON posts (status, created_at)`,

		// settings 表：键值对设置（JWT 密钥、公告、每页条数、主题等）
		`CREATE TABLE IF NOT EXISTS settings (
			"key" VARCHAR(64) PRIMARY KEY,
			value TEXT NOT NULL
		)`,

		// comments 表：文章评论 / 回复（只保留两级）
		`CREATE TABLE IF NOT EXISTS comments (
			id               BIGSERIAL PRIMARY KEY,
			post_id          BIGINT      NOT NULL,
			user_id          BIGINT      NOT NULL,
			parent_id        BIGINT,                                 -- 顶级评论 ID（NULL=顶级）
			reply_to_user_id BIGINT,                                 -- 回复 @ 的目标用户
			content          TEXT        NOT NULL,
			created_at       TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_post ON comments (post_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_user ON comments (user_id)`,

		// notifications 表：站内通知（有人回复我时，铃铛处展示）
		`CREATE TABLE IF NOT EXISTS notifications (
			id         BIGSERIAL PRIMARY KEY,
			user_id    BIGINT       NOT NULL,
			actor_id   BIGINT,
			post_id    BIGINT       NOT NULL,
			comment_id BIGINT,
			content    VARCHAR(500) NOT NULL,
			is_read    BOOLEAN      NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_read ON notifications (user_id, is_read)`,

		// post_likes 表：点赞去重（同一访客对同一文章只计一次，匿名按浏览器 Cookie）
		`CREATE TABLE IF NOT EXISTS post_likes (
			post_id    BIGINT      NOT NULL,
			visitor    VARCHAR(64) NOT NULL,
			created_at TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (post_id, visitor)
		)`,

		// friends 表：好友关系（含待处理申请）。
		// 一行代表一对用户，small_id < big_id 保证同一对用户只有一行。
		`CREATE TABLE IF NOT EXISTS friends (
			id           BIGSERIAL PRIMARY KEY,
			small_id     BIGINT      NOT NULL,
			big_id       BIGINT      NOT NULL,
			requester_id BIGINT      NOT NULL,
			status       VARCHAR(16) NOT NULL DEFAULT 'pending',
			created_at   TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at   TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uk_friends_pair UNIQUE (small_id, big_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_friends_requester ON friends (requester_id, status)`,

		// checkins 表：每日签到记录（等级系统的唯一积分来源）。
		// 主键 (user_id, check_date) 从**数据库层面**保证「一天只能签一次」——
		// 应用层即使并发点两次，也只会成功一条（靠 ON CONFLICT DO NOTHING）。
		// points 存「当次拿到的积分」而不是累计值，方便以后加连签奖励 / 补签而不动历史。
		`CREATE TABLE IF NOT EXISTS checkins (
			user_id    BIGINT      NOT NULL,
			check_date DATE        NOT NULL,
			points     INTEGER     NOT NULL DEFAULT 0,
			created_at TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, check_date)
		)`,
		// 「今天有多少人签到」这类运营统计用
		`CREATE INDEX IF NOT EXISTS idx_checkins_date ON checkins (check_date)`,

		// moments 表：说说（类似 Twitter 的短动态）。
		// images 存 JSON 数组字符串（["/uploads/a.jpg", ...]），不存逗号拼接 ——
		// URL 里一旦出现逗号就会被拆错。
		`CREATE TABLE IF NOT EXISTS moments (
			id         BIGSERIAL PRIMARY KEY,
			user_id    BIGINT      NOT NULL,
			content    TEXT        NOT NULL DEFAULT '',
			images     TEXT        NOT NULL DEFAULT '[]',
			likes      INTEGER     NOT NULL DEFAULT 0,
			views      INTEGER     NOT NULL DEFAULT 0,
			created_at TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_moments_created ON moments (created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_moments_user ON moments (user_id)`,

		// moment_likes：说说点赞去重（与文章点赞同样按访客 Cookie 计，匿名可用）
		`CREATE TABLE IF NOT EXISTS moment_likes (
			moment_id  BIGINT      NOT NULL,
			visitor    VARCHAR(64) NOT NULL,
			created_at TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (moment_id, visitor)
		)`,

		// moment_views：说说浏览去重（同一访客只计一次，刷新不会把数字刷上去）
		`CREATE TABLE IF NOT EXISTS moment_views (
			moment_id  BIGINT      NOT NULL,
			visitor    VARCHAR(64) NOT NULL,
			created_at TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (moment_id, visitor)
		)`,

		// moment_comments：说说的评论（结构与文章评论一致，仍然是两级）。
		// 单独一张表而不是给 comments 加可空列 —— 文章评论的 post_id 是 NOT NULL，
		// 混在一起会让两边查询都要带上「另一列为空」的条件，容易漏。
		`CREATE TABLE IF NOT EXISTS moment_comments (
			id               BIGSERIAL PRIMARY KEY,
			moment_id        BIGINT      NOT NULL,
			user_id          BIGINT      NOT NULL,
			parent_id        BIGINT,
			reply_to_user_id BIGINT,
			content          TEXT        NOT NULL,
			created_at       TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_moment_comments ON moment_comments (moment_id, created_at)`,

		// site_visits：按天累计的访问量（后台「总览」的折线图）。
		// 只存「每天一行」，一年也就 365 行，不占空间。
		`CREATE TABLE IF NOT EXISTS site_visits (
			day      VARCHAR(10) PRIMARY KEY,
			views    BIGINT NOT NULL DEFAULT 0,
			visitors BIGINT NOT NULL DEFAULT 0
		)`,

		// visit_days：谁在哪天来过（用 blog_visitor Cookie 去重算 UV）。
		`CREATE TABLE IF NOT EXISTS visit_days (
			day     VARCHAR(10) NOT NULL,
			visitor VARCHAR(64) NOT NULL,
			PRIMARY KEY (day, visitor)
		)`,
	}
	for _, stmt := range schema {
		if _, err := q.Exec(stmt); err != nil {
			return fmt.Errorf("初始化表结构失败: %w", err)
		}
	}
	// 老库升级（全新库此处为空操作）
	if err := migrateUsersTable(); err != nil {
		return fmt.Errorf("迁移 users 表失败: %w", err)
	}
	// posts 表：公开链接短码 slug（老文章回填随机值）
	if err := migratePostsTable(); err != nil {
		return fmt.Errorf("迁移 posts 表失败: %w", err)
	}
	// users 表：9 位桃信 ID + 等级（老用户回填）
	if err := migrateMessengerUsers(); err != nil {
		return fmt.Errorf("迁移用户 Momo Talk 字段失败: %w", err)
	}
	// 站长资料字段（老库补列）
	if err := migrateProfileFields(); err != nil {
		return fmt.Errorf("迁移站长资料字段失败: %w", err)
	}
	return nil
}

// slugChars 短码字符集（去掉易混淆的 0O1lI，url 安全）
const slugChars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// genSlug 生成随机短码（默认 12 位，用作文章公开 URL：/blog/<slug>）。
func genSlug() string {
	buf := make([]byte, 12)
	rb := make([]byte, 12)
	if _, err := rand.Read(rb); err != nil {
		// 兜底：用纳秒时间戳（几乎不可能触发）
		return fmt.Sprintf("p%d", time.Now().UnixNano())
	}
	for i := 0; i < len(buf); i++ {
		buf[i] = slugChars[int(rb[i])%len(slugChars)]
	}
	return string(buf)
}

// migratePostsTable 确保 posts 表有 slug / category / tags / views 列，并回填 slug。
// PostgreSQL 里列与唯一约束都由建表语句与新库自带，这里只处理「老库升上来」的情况。
func migratePostsTable() error {
	if err := ensureColumn("posts", "slug", "slug VARCHAR(32)"); err != nil {
		return err
	}
	if err := ensureColumn("posts", "category", "category VARCHAR(64) NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn("posts", "tags", "tags VARCHAR(255) NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn("posts", "views", "views INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	// 正文列必须可空（正文已迁到文件系统）
	if _, err := q.Exec(`ALTER TABLE posts ALTER COLUMN content DROP NOT NULL`); err != nil {
		return err
	}
	// 回填：老文章没有 slug 的一律补随机值（唯一冲突则换一个再试）
	rows, err := q.Query(`SELECT id FROM posts WHERE slug IS NULL OR slug = ''`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		for attempt := 0; attempt < 8; attempt++ {
			s := genSlug()
			if _, err := q.Exec(`UPDATE posts SET slug = ? WHERE id = ?`, s, id); err == nil {
				break
			}
		}
	}
	return nil
}

// ensureColumn 给表补一列（幂等）。ddl 是列定义，如 "slug VARCHAR(32)"。
func ensureColumn(table, column, ddl string) error {
	var n int
	// current_schema() 是函数，要写成 = current_schema()（不能当列用）
	err := q.QueryRow(
		`SELECT COUNT(*) FROM information_schema.columns
		 WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
		table, column).Scan(&n)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil // 已存在
	}
	// 表名/列名/类型都是代码里写死的，不存在注入风险
	if _, err := q.Exec("ALTER TABLE " + table + " ADD COLUMN " + ddl); err != nil {
		return err
	}
	return nil
}

// migrateUsersTable 把老版 users 表升级到新版结构（对全新库是空操作）。
// 旧结构里 username 是 NOT NULL 且没有 email 唯一约束，这里逐项补齐。
func migrateUsersTable() error {
	// username 允许为空（注册用户没有用户名）
	if _, err := q.Exec(`ALTER TABLE users ALTER COLUMN username DROP NOT NULL`); err != nil {
		return err
	}
	// 补上 email 唯一约束（已存在会报重复，忽略即可）
	if _, err := q.Exec(`ALTER TABLE users ADD CONSTRAINT uk_users_email UNIQUE (email)`); err != nil {
		if !isDuplicateObject(err) {
			return err
		}
	}
	// 老库里管理员可能只写了 username='admin'，补一个邮箱占位
	if _, err := q.Exec(`UPDATE users SET role = 'admin', email = 'admin@localhost' WHERE username = 'admin' AND (email IS NULL OR email = '')`); err != nil {
		return err
	}
	// 个人主页的封面上传图（空串 = 用站点首页背景图兜底）
	if err := ensureColumn("users", "cover", "cover VARCHAR(255) NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

// isDuplicateObject 判断错误是否为「对象已存在」（幂等 ALTER 用）。
func isDuplicateObject(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "already exists") || strings.Contains(s, "SQLSTATE 42P07") ||
		strings.Contains(s, "SQLSTATE 42710") || strings.Contains(s, "duplicate key value")
}

// migrateMessengerUsers 给 users 表补上桃信需要的两列（9 位 ID + 等级），并为老用户回填。
func migrateMessengerUsers() error {
	if err := ensureColumn("users", "user_id", "user_id VARCHAR(16) NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn("users", "level", "level INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	// 积分（等级由它换算出来，见 level.go）。老用户默认 0 分 = 1 级，与原来的 level 一致。
	if err := ensureColumn("users", "points", "points INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	// 唯一约束（已存在则忽略）
	if _, err := q.Exec(`ALTER TABLE users ADD CONSTRAINT uk_users_user_id UNIQUE (user_id)`); err != nil {
		if !isDuplicateObject(err) {
			return err
		}
	}
	// 回填：没有 9 位桃信 ID 的用户补一个不重复的随机值
	rows, err := q.Query(`SELECT id FROM users WHERE user_id IS NULL OR user_id = ''`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, id := range ids {
		for attempt := 0; attempt < 20; attempt++ {
			uid := randMessengerID()
			var exists int
			if err := q.QueryRow(`SELECT COUNT(*) FROM users WHERE user_id = ?`, uid).Scan(&exists); err != nil {
				return err
			}
			if exists > 0 {
				continue
			}
			if _, err := q.Exec(`UPDATE users SET user_id = ? WHERE id = ?`, uid, id); err != nil {
				return err
			}
			break
		}
	}
	// 等级不能为空或小于 1
	if _, err := q.Exec(`UPDATE users SET level = 1 WHERE level IS NULL OR level < 1`); err != nil {
		return err
	}
	return nil
}

// randMessengerID 生成 9 位桃信 ID（首位不为 0）。
func randMessengerID() string {
	const digits = "0123456789"
	buf := make([]byte, 9)
	rb := make([]byte, 9)
	if _, err := rand.Read(rb); err != nil {
		return fmt.Sprintf("%09d", time.Now().UnixNano()%1000000000)
	}
	for i := 0; i < 9; i++ {
		buf[i] = digits[int(rb[i])%10]
	}
	if buf[0] == '0' {
		buf[0] = digits[1+int(rb[0])%9] // 首位换成 1-9
	}
	return string(buf)
}

// genUserID 生成不重复的 9 位桃信 ID（数据库里做唯一性校验，最多重试 20 次）。
func genUserID() (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		uid := randMessengerID()
		var exists int
		if err := q.QueryRow(`SELECT COUNT(*) FROM users WHERE user_id = ?`, uid).Scan(&exists); err != nil {
			return "", err
		}
		if exists == 0 {
			return uid, nil
		}
	}
	return "", fmt.Errorf("生成 Momo Talk ID 失败（连续 20 次冲突）")
}
