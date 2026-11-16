package chatstore

// ============================================================
// postgres.go —— Store 的 PostgreSQL 实现
//
// 为什么聊天数据更适合 PostgreSQL（相对于 MariaDB/MySQL）：
//
//   ① 声明式分区维护成本低
//      MySQL 的自增列必须是某个键的首列，导致分区表主键只能写成
//      (id, conversation_id, created_at) 这种折中；提前建分区还得
//      REORGANIZE PARTITION pmax（见 mariadb.go 的 Maintain）。
//      PG 用 PARTITION BY RANGE (created_at) + 默认分区，
//      建分区就是 CREATE TABLE ... PARTITION OF，删旧数据就是一整月 DROP。
//
//   ② 分区表上能做真正的「范围扫描」
//      聊天查询永远是「某会话 + 时间倒序取 N 条」，PG 的
//      BRIN 索引对「只增不改、按时间写入」的消息表极其契合（索引体积极小），
//      配合分区裁剪可以只扫命中的月份。
//
//   ③ 类型与约束更省心
//      TIMESTAMPTZ 带时区、CHECK 约束（user_a < user_b）能把
//      「同一对用户只有一行」的约定直接压在数据库层，而不是只靠应用层。
//
// 连接管理（已修正）：
// 原来用的是「单连接 + 互斥锁」，但那把锁只包住 Query/Exec **发起**的那一瞬：
// pgx 返回的 Rows 是「边读边取」的，锁已经释放时结果集还没读完。
// 这时另一个请求（列表 + 轮询未读 + 标记已读几乎总是并发）在同一连接上发新查询，
// pgx 会直接报 `conn busy` —— 表现就是「刚发完消息好好的，重进会话后全线查询失败」。
//
// 现在改成 pgxpool 连接池：每次查询各占用一条连接，并发天然安全，锁也一并去掉。
// ============================================================

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore Store 的 PostgreSQL 实现。
type PostgresStore struct {
	db *pgxpool.Pool
}

// NewPostgres 用 DSN 连接 PostgreSQL 并校验连通性。
// DSN 形如：postgres://user:pass@127.0.0.1:5432/blogchat
func NewPostgres(ctx context.Context, dsn string) (*PostgresStore, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("解析 DSN 失败: %w", err)
	}
	// 每条新连接都把会话时区设成「程序所在时区」。
	// 为什么必须：to_char(created_at, ...) 和分区边界的字符串都按**会话时区**解释。
	// 应用在 Asia/Shanghai、数据库容器在 UTC 时，消息时间会整体差 8 小时，
	// 月初/月末的消息还可能落进上/下个月的分区。
	if tz := localZoneName(); tz != "" {
		cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
			_, err := c.Exec(ctx, `SELECT set_config('TimeZone', $1, false)`, tz)
			return err
		}
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}
	return &PostgresStore{db: pool}, nil
}

// localZoneName 取进程的时区名（如 Asia/Shanghai）；拿不到有效名字时返回空串。
func localZoneName() string {
	name := time.Local.String()
	switch name {
	case "", "Local", "UTC":
		return ""
	}
	return name
}

// Close 关闭连接池。
func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	s.db.Close()
	return nil
}

// Ping 健康检查。
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.Ping(ctx)
}

// Counts 返回当前会话数与消息数。
// 用途：启动时判断「PG 是不是空库」，据此决定要不要自动从 MariaDB 迁移历史数据。
func (s *PostgresStore) Counts(ctx context.Context) (conversations, messages int64, err error) {
	if err = s.queryRow(ctx, `SELECT COUNT(*) FROM conversations`).Scan(&conversations); err != nil {
		return 0, 0, err
	}
	if err = s.queryRow(ctx, `SELECT COUNT(*) FROM messages`).Scan(&messages); err != nil {
		return 0, 0, err
	}
	return conversations, messages, nil
}

// query / queryRow / exec 是对连接池的薄封装。
// 连接池每次查询各占一条连接，Rows 没读完也不会挡住别人 —— 这正是原来
// 「单连接 + 提前解锁」踩到的 conn busy 的根治办法。
func (s *PostgresStore) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return s.db.Query(ctx, sql, args...)
}

func (s *PostgresStore) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return s.db.QueryRow(ctx, sql, args...)
}

func (s *PostgresStore) exec(ctx context.Context, sql string, args ...any) error {
	_, err := s.db.Exec(ctx, sql, args...)
	return err
}

// ---------- 表结构 ----------

// EnsureSchema 幂等建表：会话 / 成员 / 按月分区的消息表。
func (s *PostgresStore) EnsureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS conversations (
			id         BIGSERIAL PRIMARY KEY,
			user_a     BIGINT      NOT NULL,
			user_b     BIGINT      NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			CONSTRAINT uk_conversations_pair UNIQUE (user_a, user_b),
			CONSTRAINT ck_conversations_order CHECK (user_a < user_b)
		)`,
		`CREATE TABLE IF NOT EXISTS conversation_members (
			conversation_id BIGINT      NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			user_id         BIGINT      NOT NULL,
			last_read_seq   BIGINT      NOT NULL DEFAULT 0,
			joined_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (conversation_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cm_user ON conversation_members (user_id)`,
		// 消息主表：按月 RANGE 分区。
		// 主键必须包含分区键 created_at（PG 的硬性要求），
		// 同时 id 用 BIGSERIAL 保证全局单调递增 —— 游标翻页与已读水位
		// （last_read_seq）都依赖这个单调性来定位「读到哪了」。
		`CREATE TABLE IF NOT EXISTS messages (
			id              BIGSERIAL   NOT NULL,
			conversation_id BIGINT      NOT NULL,
			sender_id       BIGINT      NOT NULL,
			content         TEXT        NOT NULL,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (id, created_at)
		) PARTITION BY RANGE (created_at)`,
		// 会话内按时间取最近 N 条的主索引
		`CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages (conversation_id, created_at DESC, id DESC)`,
		// 消息表按「只增不改、按时间写入」组织，BRIN 体积极小、范围扫描很划算
		`CREATE INDEX IF NOT EXISTS idx_messages_created_brin ON messages USING BRIN (created_at)`,
	}
	for _, sql := range stmts {
		if err := s.exec(ctx, sql); err != nil {
			return fmt.Errorf("初始化聊天表失败: %w", err)
		}
	}
	return s.EnsurePartitions(ctx)
}

// EnsurePartitions 保证「过去 12 个月 ~ 未来 6 个月」都有分区，
// 并保留一个 default 分区兜底：万一定时任务没跑，新消息也只会落进
// default 而不会插入失败。
func (s *PostgresStore) EnsurePartitions(ctx context.Context) error {
	now := time.Now()
	for i := -12; i <= 6; i++ {
		month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, i, 0)
		if err := s.createPartition(ctx, month); err != nil {
			return err
		}
	}
	return s.ensureDefaultPartition(ctx)
}

// createPartition 建一个月分区；已存在则忽略。
func (s *PostgresStore) createPartition(ctx context.Context, month time.Time) error {
	name := pgPartitionName(month)
	upper := month.AddDate(0, 1, 0)
	sql := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF messages FOR VALUES FROM ('%s') TO ('%s')`,
		name, month.Format("2006-01-02"), upper.Format("2006-01-02"))
	return s.exec(ctx, sql)
}

// ensureDefaultPartition 建兜底分区（未匹配任何月份的数据落这里）。
// 注意：default 分区一旦有了数据，再往里 ADD 新分区会失败（需要先搬走数据），
// 所以 EnsurePartitions 会提前把未来半年的分区建好。
func (s *PostgresStore) ensureDefaultPartition(ctx context.Context) error {
	return s.exec(ctx, `CREATE TABLE IF NOT EXISTS messages_default PARTITION OF messages DEFAULT`)
}

// pgPartitionName 月份到分区名，如 2026-09 -> messages_p202609。
// mariadb.go 里已有同名的 partitionName（返回 p202609），这里加 pg 前缀区分。
func pgPartitionName(t time.Time) string { return "messages_p" + t.Format("200601") }

// ---------- 会话 ----------

// FindConversation 查两人之间的会话。
func (s *PostgresStore) FindConversation(ctx context.Context, u1, u2 int64) (*Conversation, error) {
	a, b := Order(u1, u2)
	var id int64
	err := s.queryRow(ctx, `SELECT id FROM conversations WHERE user_a = $1 AND user_b = $2`, a, b).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Conversation{ID: id, UserA: a, UserB: b, Exists: true}, nil
}

// EnsureConversation 取（或新建）会话，并补齐成员行。
func (s *PostgresStore) EnsureConversation(ctx context.Context, u1, u2 int64) (*Conversation, error) {
	a, b := Order(u1, u2)
	// ON CONFLICT DO NOTHING：并发同时创建时不会报错，随后 SELECT 拿到同一个 id
	if err := s.exec(ctx,
		`INSERT INTO conversations (user_a, user_b) VALUES ($1, $2) ON CONFLICT (user_a, user_b) DO NOTHING`, a, b); err != nil {
		return nil, err
	}
	conv, err := s.FindConversation(ctx, a, b)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, ErrNotFound
	}
	// 成员行：已存在则不动，保留 last_read_seq
	for _, uid := range []int64{a, b} {
		if err := s.exec(ctx,
			`INSERT INTO conversation_members (conversation_id, user_id) VALUES ($1, $2)
			 ON CONFLICT (conversation_id, user_id) DO NOTHING`, conv.ID, uid); err != nil {
			return nil, err
		}
	}
	return conv, nil
}

// Peer 校验权限并返回对方 ID（不是自己的会话一律 ErrNotFound，不泄露存在性）。
func (s *PostgresStore) Peer(ctx context.Context, convID, me int64) (int64, error) {
	var a, b int64
	err := s.queryRow(ctx, `SELECT user_a, user_b FROM conversations WHERE id = $1`, convID).Scan(&a, &b)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if a != me && b != me {
		return 0, ErrNotFound
	}
	if a == me {
		return b, nil
	}
	return a, nil
}

// ---------- 消息 ----------

// ListMessages 取会话消息（按时间正序返回）。
// beforeID > 0 时取 id 小于它的更早消息（向上翻页）。
func (s *PostgresStore) ListMessages(ctx context.Context, convID int64, beforeID int64, limit int) ([]Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// 倒序取最近 limit 条（走 idx_messages_conv），再在内存里反转成正序
	const base = `SELECT m.id, m.conversation_id, m.sender_id, m.content,
	                     to_char(m.created_at, 'YYYY-MM-DD HH24:MI:SS')
	              FROM messages m
	              WHERE m.conversation_id = $1`
	var (
		rows pgx.Rows
		err  error
	)
	if beforeID > 0 {
		rows, err = s.query(ctx, base+` AND m.id < $2 ORDER BY m.created_at DESC, m.id DESC LIMIT $3`, convID, beforeID, limit)
	} else {
		rows, err = s.query(ctx, base+` ORDER BY m.created_at DESC, m.id DESC LIMIT $2`, convID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 反转成正序
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// AppendMessage 追加一条消息并返回落库后的完整记录。
func (s *PostgresStore) AppendMessage(ctx context.Context, convID, senderID int64, content string) (*Message, error) {
	var (
		id        int64
		createdAt string
	)
	// RETURNING 回读 created_at：避免应用时钟与数据库时钟不一致
	err := s.queryRow(ctx,
		`INSERT INTO messages (conversation_id, sender_id, content) VALUES ($1, $2, $3)
		 RETURNING id, to_char(created_at, 'YYYY-MM-DD HH24:MI:SS')`,
		convID, senderID, content).Scan(&id, &createdAt)
	if err != nil {
		return nil, err
	}
	m := &Message{ID: id, ConversationID: convID, SenderID: senderID, Content: content, CreatedAt: createdAt}
	return m, nil
}

// MarkRead 把 me 在该会话的已读位置推进到最新一条消息的 id。
func (s *PostgresStore) MarkRead(ctx context.Context, convID, me int64) error {
	return s.exec(ctx,
		`UPDATE conversation_members
		 SET last_read_seq = COALESCE((SELECT MAX(id) FROM messages WHERE conversation_id = $1), 0)
		 WHERE conversation_id = $1 AND user_id = $2`, convID, me)
}

// UnreadOf 某人在某会话的未读条数。
func (s *PostgresStore) UnreadOf(ctx context.Context, convID, me int64) (int64, error) {
	var n int64
	err := s.queryRow(ctx,
		`SELECT COUNT(*) FROM conversation_members cm
		 JOIN messages m ON m.conversation_id = cm.conversation_id AND m.id > cm.last_read_seq
		 WHERE cm.conversation_id = $1 AND cm.user_id = $2`, convID, me).Scan(&n)
	return n, err
}

// UnreadSummary 全站未读汇总：未读私信 + 好友申请 + 未读通知。
// 三张表都在同一个 PostgreSQL 库里，直接查即可（不再需要跨库回调）。
// 注意占位符写法：这里是 pgx 直连（不经过 main 包的 pgQuery 翻译），
// 所以必须写 $1/$2，不能写 ?。
func (s *PostgresStore) UnreadSummary(ctx context.Context, me int64) (Unread, error) {
	var u Unread
	if err := s.queryRow(ctx,
		`SELECT COUNT(*) FROM conversation_members cm
		 JOIN messages m ON m.conversation_id = cm.conversation_id AND m.id > cm.last_read_seq
		 WHERE cm.user_id = $1`, me).Scan(&u.Messages); err != nil {
		return u, err
	}
	// 好友申请数：待处理、且不是我发起的
	if err := s.queryRow(ctx,
		`SELECT COUNT(*) FROM friends
		 WHERE status = 'pending' AND requester_id <> $1 AND (small_id = $1 OR big_id = $1)`,
		me).Scan(&u.FriendReqs); err != nil {
		return u, err
	}
	// 未读通知数（is_read 是 BOOLEAN，不能用 = 0 比较）
	if err := s.queryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`,
		me).Scan(&u.Notifications); err != nil {
		return u, err
	}
	return u, nil
}

// Maintain 定期维护：把未来几个月的分区建好。
// PG 里没有 MySQL 那种「重建 pmax」的复杂语义，就是几条 CREATE TABLE IF NOT EXISTS。
func (s *PostgresStore) Maintain(ctx context.Context) error {
	return s.EnsurePartitions(ctx)
}

// ---------- MariaDB 到 PostgreSQL 的一次性迁移 ----------

// MigrateStats 迁移结果统计。
type MigrateStats struct {
	Conversations int
	Messages      int
	Members       int
	Skipped       int // 因冲突跳过的行数
}

// MigrateFromSQL 把 MariaDB（database/sql 源库）里的聊天数据搬进 PostgreSQL。
//
// 参数直接收 *sql.DB 而不是自定义接口：*sql.DB 的 QueryContext 返回 *sql.Rows，
// Go 的方法签名是精确匹配的（*sql.Rows 与自定义接口类型不能互相赋值），
// 收接口反而会让调用点无法直接传 *sql.DB。
//
// 语义对齐说明：
//
//	· conversations.id 原样保留 —— 这样 MariaDB 侧其他表里引用会话 id 的地方不用改；
//	  搬完把 PG 的 BIGSERIAL 序列推到 MAX(id) 之后即可。
//	· last_read_id（MariaDB）-> last_read_seq（PG）：语义完全一致，都是消息 id 水位。
//	· 消息的 id 与 created_at 原样保留；ON CONFLICT DO NOTHING 保证幂等，
//	  可以重复执行、也可以在服务运行中执行。
func (s *PostgresStore) MigrateFromSQL(ctx context.Context, src *sql.DB, logf func(string, ...any)) (MigrateStats, error) {
	var st MigrateStats
	if logf == nil {
		logf = func(string, ...any) {}
	}

	// ---------- ① 会话 ----------
	rows, err := src.QueryContext(ctx, `SELECT id, user_a, user_b FROM conversations ORDER BY id`)
	if err != nil {
		return st, fmt.Errorf("读取源会话失败: %w", err)
	}
	for rows.Next() {
		var id, a, b int64
		if err := rows.Scan(&id, &a, &b); err != nil {
			rows.Close()
			return st, err
		}
		ok, err := s.execReturning(ctx,
			`INSERT INTO conversations (id, user_a, user_b) VALUES ($1, $2, $3)
			 ON CONFLICT (user_a, user_b) DO NOTHING`, id, a, b)
		if err != nil {
			rows.Close()
			return st, fmt.Errorf("写入会话 %d 失败: %w", id, err)
		}
		if ok {
			st.Conversations++
		} else {
			st.Skipped++
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return st, err
	}
	rows.Close()
	logf("会话迁移完成：新增 %d，跳过 %d", st.Conversations, st.Skipped)

	// ---------- ② 成员（已读水位）----------
	rows, err = src.QueryContext(ctx, `SELECT conversation_id, user_id, last_read_id FROM conversation_members`)
	if err != nil {
		return st, fmt.Errorf("读取源成员失败: %w", err)
	}
	for rows.Next() {
		var convID, userID, lastRead int64
		if err := rows.Scan(&convID, &userID, &lastRead); err != nil {
			rows.Close()
			return st, err
		}
		// 已存在则只把已读水位往「更靠后」推进，不回退
		if err := s.exec(ctx,
			`INSERT INTO conversation_members (conversation_id, user_id, last_read_seq) VALUES ($1, $2, $3)
			 ON CONFLICT (conversation_id, user_id)
			 DO UPDATE SET last_read_seq = GREATEST(conversation_members.last_read_seq, EXCLUDED.last_read_seq)`,
			convID, userID, lastRead); err != nil {
			rows.Close()
			return st, fmt.Errorf("写入成员 (%d,%d) 失败: %w", convID, userID, err)
		}
		st.Members++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return st, err
	}
	rows.Close()
	logf("成员/已读水位迁移完成：%d 行", st.Members)

	// ---------- ③ 消息（边跑边搬）----------
	msgRows, err := src.QueryContext(ctx,
		`SELECT id, conversation_id, sender_id, content, created_at FROM messages ORDER BY id`)
	if err != nil {
		return st, fmt.Errorf("读取源消息失败: %w", err)
	}
	for msgRows.Next() {
		var (
			id, convID, senderID int64
			content, createdAt   string
		)
		if err := msgRows.Scan(&id, &convID, &senderID, &content, &createdAt); err != nil {
			msgRows.Close()
			return st, err
		}
		// 显式带上 id 与 created_at：分区键必须落在正确的月份分区里
		if err := s.exec(ctx,
			`INSERT INTO messages (id, conversation_id, sender_id, content, created_at)
			 VALUES ($1, $2, $3, $4, $5::timestamp)
			 ON CONFLICT (id, created_at) DO NOTHING`,
			id, convID, senderID, content, createdAt); err != nil {
			msgRows.Close()
			return st, fmt.Errorf("写入消息 %d 失败: %w", id, err)
		}
		st.Messages++
	}
	if err := msgRows.Err(); err != nil {
		msgRows.Close()
		return st, err
	}
	msgRows.Close()
	logf("消息迁移完成：%d 条", st.Messages)

	// ---------- ④ 把序列推到 MAX(id) 之后，避免后续插入主键冲突 ----------
	// setval 第三个参数 is_called：表里有数据时为 true，表示下一个值从 max+1 开始。
	if err := s.exec(ctx,
		`SELECT setval(pg_get_serial_sequence('messages', 'id'),
		               COALESCE((SELECT MAX(id) FROM messages), 1),
		               (SELECT COUNT(*) > 0 FROM messages))`); err != nil {
		return st, fmt.Errorf("同步 messages 序列失败: %w", err)
	}
	if err := s.exec(ctx,
		`SELECT setval(pg_get_serial_sequence('conversations', 'id'),
		               COALESCE((SELECT MAX(id) FROM conversations), 1),
		               (SELECT COUNT(*) > 0 FROM conversations))`); err != nil {
		return st, fmt.Errorf("同步 conversations 序列失败: %w", err)
	}
	return st, nil
}

// SQLSource / SQLRows 已不再需要：MigrateFromSQL 直接收 *sql.DB。
// 保留这两个注释是为了说明「为什么当初想抽接口却放弃」——
// Go 的方法签名精确匹配使得 *sql.DB 无法隐式满足自定义接口。

// execReturning 执行一条 INSERT，返回「是否真的插入了」（用于统计）。
func (s *PostgresStore) execReturning(ctx context.Context, sql string, args ...any) (bool, error) {
	tag, err := s.db.Exec(ctx, sql, args...)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ChatDSN 由配置拼出 PostgreSQL DSN。
// 传空 host 时返回空串（表示未启用）。
func ChatDSN(host, port, user, password, dbname, sslmode string) string {
	if strings.TrimSpace(host) == "" {
		return ""
	}
	if port == "" {
		port = "5432"
	}
	if user == "" {
		user = "postgres"
	}
	if dbname == "" {
		dbname = "blogchat"
	}
	if sslmode == "" {
		sslmode = "disable"
	}
	auth := user
	if password != "" {
		auth = user + ":" + password
	}
	return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=%s", auth, host, port, dbname, sslmode)
}
