package main

// ============================================================
// pgdb.go —— PostgreSQL 访问的薄封装
//
// 目的：让散落在 12 个文件里的 86 处查询保持 `db.Query(sql, args...)` 的写法不变，
// 同时满足 PostgreSQL 的两点差异：
//
//   ① 占位符是 $1/$2 而不是 ?（由 pgQuery 统一翻译，见 db.go）
//   ② INSERT 不能靠 LastInsertId 取自增主键，必须用 RETURNING
//      —— 所以提供 insertReturningID 一族。
//
// q 提供与 *sql.DB 相同的方法签名，所以「把 db 换成 q」是纯机械替换。
// db 仍然保留给需要 Begin（事务）与 Ping 等场景使用。
// ============================================================

import (
	"context"
	"database/sql"
	"strings"
)

// pgDB 包装 *sql.DB，在 Query/QueryRow/Exec 三个入口做占位符翻译。
type pgDB struct{ *sql.DB }

// q 全局查询入口：SQL 里写 `?`，这里翻译成 $N 后交给 PostgreSQL。
var q = pgDB{}

// Query 执行查询（返回多行）。
func (p pgDB) Query(sql string, args ...any) (*sql.Rows, error) {
	return p.DB.Query(pgQuery(sql), args...)
}

// QueryRow 执行查询（返回单行）。
func (p pgDB) QueryRow(sql string, args ...any) *sql.Row {
	return p.DB.QueryRow(pgQuery(sql), args...)
}

// Exec 执行写操作（不关心返回值）。
func (p pgDB) Exec(sql string, args ...any) (sql.Result, error) {
	return p.DB.Exec(pgQuery(sql), args...)
}

// QueryContext / QueryRowContext / ExecContext：带 context 的版本
// （后台任务与迁移逻辑用，可以带超时取消）。
func (p pgDB) QueryContext(ctx context.Context, sql string, args ...any) (*sql.Rows, error) {
	return p.DB.QueryContext(ctx, pgQuery(sql), args...)
}

func (p pgDB) QueryRowContext(ctx context.Context, sql string, args ...any) *sql.Row {
	return p.DB.QueryRowContext(ctx, pgQuery(sql), args...)
}

func (p pgDB) ExecContext(ctx context.Context, sql string, args ...any) (sql.Result, error) {
	return p.DB.ExecContext(ctx, pgQuery(sql), args...)
}

// insertReturningID 执行一条 INSERT ... RETURNING id，返回新生成的主键。
//
// 为什么需要它：PostgreSQL 的 database/sql 驱动不支持 LastInsertId()
// （没有 MySQL 那种「最后一次自增 ID」的概念），标准做法就是 RETURNING id。
// 调用方在 SQL 末尾自行写 `RETURNING id` 也可以，但这个helper 更省事、也更醒目。
func insertReturningID(query string, args ...any) (int64, error) {
	var id int64
	if err := q.QueryRow(query, args...).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// insertReturningIDContext 带 context 的插入。
func insertReturningIDContext(ctx context.Context, query string, args ...any) (int64, error) {
	var id int64
	if err := q.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// isUniqueViolation 判断错误是否为唯一约束冲突（PostgreSQL SQLSTATE 23505）。
// 旧代码里判断的是 MySQL 的 "1062" / "Duplicate entry"，换库后统一走这里。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "23505") || strings.Contains(s, "duplicate key value") ||
		strings.Contains(s, "SQLSTATE 23505")
}

// pgTx 同 pgDB，用于事务内的查询（同样做占位符翻译）。
type pgTx struct{ tx *sql.Tx }

// QueryRow 事务内单行查询。
func (p pgTx) QueryRow(sql string, args ...any) *sql.Row {
	return p.tx.QueryRow(pgQuery(sql), args...)
}

// Query 事务内多行查询。
func (p pgTx) Query(sql string, args ...any) (*sql.Rows, error) {
	return p.tx.Query(pgQuery(sql), args...)
}

// Exec 事务内写操作。
func (p pgTx) Exec(sql string, args ...any) (sql.Result, error) {
	return p.tx.Exec(pgQuery(sql), args...)
}
