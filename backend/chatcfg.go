package main

// ============================================================
// chatcfg.go —— 聊天存储的装配（PostgreSQL）
//
// 全站只剩一个数据库：PostgreSQL。聊天三张表（conversations /
// conversation_members / messages）与用户、文章索引、评论等
// 都在同一个库里，装配只需要连上并建表。
//
// 环境变量（全部有默认值，本机开发直接跑即可）：
//
//	CHAT_PG_DSN  完整的 PostgreSQL 连接串（设了它就不用下面的分项）
//	CHAT_PG_HOST 默认 127.0.0.1
//	CHAT_PG_PORT 默认 5432
//	CHAT_PG_USER 默认 postgres
//	CHAT_PG_PASSWORD 默认空
//	CHAT_PG_DB   默认 blog（与主库同库）
//	CHAT_PG_SSLMODE 默认 disable
//
// 为什么聊天单独一层（chatstore）而不是直接写 SQL：
//   消息表是按月分区的大表，访问模式（按会话顺序追加、按时间倒序取 N 条、
//   整月归档）跟博客 CRUD 完全不同。Store 接口把这层差异关在包里，
//   handler 只依赖接口。既然现在全站都用 PG，这里直接建 PG 实现。
// ============================================================

import (
	"context"
	"log"
	"time"

	"rineyce/blog/chatstore"
)

// initChatStore 连接 PostgreSQL 并装配聊天存储。
// 失败即致命：PG 现在承载全部数据，连不上就没法提供服务，
// 直接退出比「半死不活地跑」更容易发现问题。
func initChatStore() {
	// 优先用完整 DSN；否则按分项拼（与主库共用同一套默认值）
	dsn := getenv("CHAT_PG_DSN", "")
	if dsn == "" {
		dsn = chatstore.ChatDSN(
			getenv("CHAT_PG_HOST", getenv("DB_HOST", "127.0.0.1")),
			getenv("CHAT_PG_PORT", getenv("DB_PORT", "5432")),
			getenv("CHAT_PG_USER", getenv("DB_USER", "postgres")),
			getenv("CHAT_PG_PASSWORD", getenv("DB_PASSWORD", "")),
			getenv("CHAT_PG_DB", getenv("DB_NAME", "blog")),
			getenv("CHAT_PG_SSLMODE", getenv("DB_SSLMODE", "disable")),
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pg, err := chatstore.NewPostgres(ctx, dsn)
	if err != nil {
		log.Fatalf("聊天存储：连接 PostgreSQL 失败 —— %v", err)
	}
	// 建立分区表与索引（幂等）
	if err := pg.EnsureSchema(ctx); err != nil {
		log.Fatalf("聊天存储：初始化表结构失败 —— %v", err)
	}

	chatStore = pg

	convs, msgs, err := pg.Counts(ctx)
	if err == nil {
		log.Printf("聊天存储：PostgreSQL 就绪（%d 个会话 / %d 条消息，消息表按月分区）", convs, msgs)
	} else {
		log.Printf("聊天存储：PostgreSQL 就绪")
	}
}
