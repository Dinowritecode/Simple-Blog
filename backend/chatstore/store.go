// Package chatstore 是聊天数据的存储层抽象。
//
// 为什么要有这层：
//   聊天数据的访问模式（按会话顺序追加、按时间倒序取最近 N 条、按时间归档）
//   与博客的 CRUD 很不一样，将来很可能换成 PostgreSQL / MongoDB / ScyllaDB。
//   把「怎么存」关在这个包里，HTTP handler 只依赖 Store 接口，
//   换库时只需要新增一个实现文件，chat.go 一行都不用改。
//
// 目前提供：
//   postgres.go —— 基于 PostgreSQL 的实现（本项目当前在用的，messages 按月分区）
//
// 目录下 schema_postgres.sql 给出了同样语义的 PostgreSQL 建表脚本与迁移说明，
// 方便将来切库（当前这台机器上没装 PostgreSQL，所以只落文档、不做实现）。
package chatstore

import (
	"context"
	"errors"
)

// ErrNotFound 目标不存在（会话 / 消息）。
var ErrNotFound = errors.New("chatstore: 记录不存在")

// Message 一条私信。
//
// 注意时间字段的类型是 string 而不是 time.Time：
// 时间字段统一以 "2006-01-02 15:04:05" 字符串承接，
// DATETIME 列会以 []byte（"2026-09-25 23:01:46"）返回。直接按字符串承接，
// handler 也就能原样返回给前端，省掉「解析成 time.Time 再格式化回来」这一趟。
type Message struct {
	ID             int64  // 消息 ID（同一会话内单调递增）
	ConversationID int64  // 所属会话
	SenderID       int64  // 发送者用户 ID
	SenderUserID   string // 发送者 9 位桃信 ID（冗余出来省一次 join）
	Content        string // 正文
	CreatedAt      string // 发送时间（"2006-01-02 15:04:05"）
}

// Conversation 一个一对一会话。
type Conversation struct {
	ID     int64 // 会话 ID
	UserA  int64 // 较小的用户 ID
	UserB  int64 // 较大的用户 ID
	Exists bool  // 是否已存在（Ensure 时用来告诉调用方「新建了没有」）
}

// Unread 未读汇总。
type Unread struct {
	Messages      int64 // 未读私信条数
	FriendReqs    int64 // 待处理好友申请数
	Notifications int64 // 未读回复通知数
}

// Total 红点上的总数。
func (u Unread) Total() int64 { return u.Messages + u.FriendReqs + u.Notifications }

// Store 聊天存储接口。
//
// 约定：
//   - 所有方法都必须自己处理「参数非法」的情况，调用方只管传值
//   - userA < userB 的规范化由 Order 负责，实现里不再重复判断
type Store interface {
	// Ping 健康检查（启动时用来确认表结构就绪）。
	Ping(ctx context.Context) error

	// FindConversation 查两人之间的会话；不存在返回 nil（不建）。
	FindConversation(ctx context.Context, u1, u2 int64) (*Conversation, error)

	// EnsureConversation 取（不存在则创建）两人之间的会话，并保证成员行存在。
	EnsureConversation(ctx context.Context, u1, u2 int64) (*Conversation, error)

	// Peer 校验 convID 是 me 参与的会话，并返回对方用户 ID。
	// 不是自己的会话要返回 ErrNotFound（防越权）。
	Peer(ctx context.Context, convID, me int64) (int64, error)

	// ListMessages 取会话消息。
	//   beforeID > 0：取 id 小于它的更早消息（向上翻页）
	//   beforeID = 0：取最新的 limit 条
	// 返回结果按时间正序（最旧在前），便于前端直接渲染。
	ListMessages(ctx context.Context, convID int64, beforeID int64, limit int) ([]Message, error)

	// AppendMessage 追加一条消息。
	AppendMessage(ctx context.Context, convID, senderID int64, content string) (*Message, error)

	// MarkRead 把 me 在该会话的已读位置推进到最新一条。
	MarkRead(ctx context.Context, convID, me int64) error

	// UnreadOf 某人在某会话的未读条数。
	UnreadOf(ctx context.Context, convID, me int64) (int64, error)

	// UnreadSummary 全站未读汇总（私信 + 好友申请 + 通知）。
	UnreadSummary(ctx context.Context, me int64) (Unread, error)

	// Maintain 定期维护（建下个月的分区、清理过期数据）。由后台定时任务调用。
	Maintain(ctx context.Context) error
}

// Order 把两个用户 ID 规范成 (小, 大)。
// 这是「同一对用户只有一行」的关键约定：friends / conversations / messages
// 的所有实现都必须用它，否则会出现 A→B 与 B→A 两条记录。
func Order(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}
