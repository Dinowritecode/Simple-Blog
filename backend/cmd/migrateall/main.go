// migrate_all.go —— 一次性全量数据迁移工具（MariaDB → PostgreSQL）
//
// 用法（在 backend 目录下）：
//   go run ./cmd/migrateall
//
// 环境变量：
//   SRC_DSN  MariaDB 连接串，默认 root:@tcp(127.0.0.1:3306)/blog?charset=utf8mb4&parseTime=false
//   DST_DSN  PostgreSQL 连接串，默认 postgres://postgres@127.0.0.1:5432/blog?sslmode=disable
//
// 特点：
//   · 幂等：全部用 ON CONFLICT DO NOTHING，可重复执行
//   · 保留原 id：users / posts / comments 等表的主键原样搬运，
//     这样表之间的引用关系（parent_id / post_id / user_id）不用重建；
//     搬完把每个表的 BIGSERIAL 序列推到 MAX(id) 之后。
//   · posts.content 不搬 —— 正文已迁到文件系统（data/posts），库里存 NULL。
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// pgPlaceholders 把 `?` 换成 $1..$n（目标库是 PostgreSQL）
func pgPlaceholders(sqlText string) string {
	var b strings.Builder
	n := 0
	for i := 0; i < len(sqlText); i++ {
		if sqlText[i] == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteByte(sqlText[i])
	}
	return b.String()
}

var dst *sql.DB

func insert(table string, cols []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	ph := make([]string, len(cols))
	for i := range cols {
		ph[i] = "?"
	}
	for _, row := range rows {
		sqlText := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING",
			table, strings.Join(cols, ", "), strings.Join(ph, ", "))
		if _, err := dst.Exec(pgPlaceholders(sqlText), row...); err != nil {
			return fmt.Errorf("插入 %s 失败: %w", table, err)
		}
	}
	log.Printf("  %-22s %d 行", table, len(rows))
	return nil
}

// syncSeq 把 PostgreSQL 的序列推到 MAX(id)，避免后续插入主键冲突
func syncSeq(table, idCol string) error {
	_, err := dst.Exec(fmt.Sprintf(
		`SELECT setval(pg_get_serial_sequence('%s','%s'),
		               COALESCE((SELECT MAX(%s) FROM %s), 1),
		               (SELECT COUNT(*) > 0 FROM %s))`, table, idCol, idCol, table, table))
	return err
}

// quoteSrc 给源库（MariaDB）的列名加反引号。
// 需要它的原因：settings 的主键列名是 `key`，而 key 在 MariaDB 里是保留字，
// 裸写会报语法错误；PostgreSQL 里 key 不是保留字，所以目标侧不加引号。
func quoteSrc(col string) string { return "`" + col + "`" }

// copyTable 通用搬运：按列名从源库读到内存再写目标库（数据量小，直接全量读）
func copyTable(src *sql.DB, table string, cols []string) error {
	srcCols := make([]string, len(cols))
	for i, c := range cols {
		srcCols[i] = quoteSrc(c)
	}
	rows, err := src.Query(fmt.Sprintf("SELECT %s FROM %s", strings.Join(srcCols, ", "), table))
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", table, err)
	}
	defer rows.Close()

	var out [][]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return fmt.Errorf("扫描 %s 失败: %w", table, err)
		}
		// 把 []byte 归一成 string（MySQL 驱动对 TEXT 返回 []byte，PG 需要 string）
		for i, v := range vals {
			switch t := v.(type) {
			case []byte:
				vals[i] = string(t)
			default:
				_ = t
			}
		}
		out = append(out, vals)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return insert(table, cols, out)
}

func main() {
	srcDSN := os.Getenv("SRC_DSN")
	if srcDSN == "" {
		srcDSN = "root:@tcp(127.0.0.1:3306)/blog?charset=utf8mb4&parseTime=false"
	}
	dstDSN := os.Getenv("DST_DSN")
	if dstDSN == "" {
		dstDSN = "postgres://postgres@127.0.0.1:5432/blog?sslmode=disable"
	}

	src, err := sql.Open("mysql", srcDSN)
	if err != nil {
		log.Fatalf("连接 MariaDB 失败: %v", err)
	}
	defer src.Close()
	dst, err = sql.Open("pgx", dstDSN)
	if err != nil {
		log.Fatalf("连接 PostgreSQL 失败: %v", err)
	}
	defer dst.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := src.PingContext(ctx); err != nil {
		log.Fatalf("MariaDB 不可用: %v", err)
	}
	if err := dst.PingContext(ctx); err != nil {
		log.Fatalf("PostgreSQL 不可用: %v", err)
	}
	log.Println("开始迁移 MariaDB -> PostgreSQL")

	steps := []struct {
		table string
		cols  []string
	}{
		{"users", []string{"id", "username", "email", "password_hash", "nickname", "avatar", "role", "user_id", "level", "created_at"}},
		// settings 的主键列名就是 key（小写标识符）。这里必须写裸 key：
		// 写成 "key" 会被 PostgreSQL 当成字符串常量，导致只插入一行且值就是 key 字面量。
		{"settings", []string{"key", "value"}},
		// 正文已迁至文件系统，这里只搬索引列；content 留 NULL
		{"posts", []string{"id", "slug", "title", "summary", "cover", "category", "tags", "status", "likes", "views", "created_at", "updated_at"}},
		{"post_likes", []string{"post_id", "visitor", "created_at"}},
		{"comments", []string{"id", "post_id", "user_id", "parent_id", "reply_to_user_id", "content", "created_at"}},
		{"notifications", []string{"id", "user_id", "actor_id", "post_id", "comment_id", "content", "is_read", "created_at"}},
		{"friends", []string{"id", "small_id", "big_id", "requester_id", "status", "created_at", "updated_at"}},
		{"conversations", []string{"id", "user_a", "user_b", "created_at"}},
		// 注意：conversation_members 不在这里搬 —— 两边的已读水位列名不同
		// （MariaDB 是 last_read_id，PostgreSQL 是 last_read_seq），
		// 由下面的 migrateMembers 专门处理。
		{"messages", []string{"id", "conversation_id", "sender_id", "content", "created_at"}},
	}

	for _, s := range steps {
		log.Printf("迁移 %s ...", s.table)
		if err := copyTable(src, s.table, s.cols); err != nil {
			log.Fatalf("%v", err)
		}
	}

	// conversation_members 的已读水位列名不同，单独处理
	log.Println("迁移 conversation_members（已读水位 last_read_id -> last_read_seq）")
	if err := migrateMembers(src); err != nil {
		log.Fatalf("%v", err)
	}

	// 把序列推到 MAX(id)
	for _, t := range []string{"users", "posts", "comments", "notifications", "friends", "conversations", "messages"} {
		if err := syncSeq(t, "id"); err != nil {
			log.Printf("  (同步 %s 序列失败: %v)", t, err)
		}
	}
	log.Println("迁移完成 ✅")
}

// migrateMembers 处理会话成员已读水位（列名在两边不同）。
func migrateMembers(src *sql.DB) error {
	rows, err := src.Query(`SELECT conversation_id, user_id, last_read_id FROM conversation_members`)
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var convID, userID, lastRead int64
		if err := rows.Scan(&convID, &userID, &lastRead); err != nil {
			return err
		}
		if _, err := dst.Exec(pgPlaceholders(
			`INSERT INTO conversation_members (conversation_id, user_id, last_read_seq)
			 VALUES (?, ?, ?)
			 ON CONFLICT (conversation_id, user_id)
			 DO UPDATE SET last_read_seq = GREATEST(conversation_members.last_read_seq, EXCLUDED.last_read_seq)`),
			convID, userID, lastRead); err != nil {
			return err
		}
		n++
	}
	log.Printf("  %-22s %d 行", "conversation_members", n)
	return rows.Err()
}
