package main

// ============================================================
// events.go —— 【桃信】实时事件推送（SSE）
//
// 为什么用 SSE 而不是 WebSocket：
//   - 桃信的实时需求是「服务器 → 浏览器」单向推送（新私信 / 新好友申请 / 新通知），
//     发消息走普通 POST 即可，不需要双向长连接；
//   - SSE 就是普通 HTTP 长连接，Go 标准库直接支持（http.Flusher），
//     不用引入 gorilla/websocket 之类的依赖，也不用处理心跳、粘包、协议升级；
//   - 浏览器 EventSource 断线会自动重连（默认 3 秒），前端只需处理 onmessage。
//
// 结构：
//   hub      —— 全局事件总线：userID → 该用户在线的所有订阅通道
//   event    —— 一条事件（type + 任意 JSON 载荷）
//   subscribe / unsubscribe —— 连接建立 / 断开时登记
//   broadcastUser           —— 给某个用户的所有标签页推送
//
// 注意：通道带缓冲 + 非阻塞发送。订阅者（浏览器）读得慢时宁可丢事件也不阻塞
// 其它人的推送；前端拿到事件后都会再拉一次接口兜底，所以丢事件不会导致数据不一致。
// ============================================================

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// event 一条待推送事件。Type 对应前端 EventSource 的 event 名（如 chat / friend / notification）。
type event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// hub 全局事件总线（进程内）。
type hub struct {
	mu sync.RWMutex
	// subs: userID → 订阅通道集合。一个用户可能开着多个标签页，所以是集合。
	subs map[int64]map[chan event]struct{}
}

var eventHub = &hub{subs: map[int64]map[chan event]struct{}{}}

// subscribe 登记一个订阅通道，返回取消函数。
func (h *hub) subscribe(userID int64) (chan event, func()) {
	ch := make(chan event, 16)
	h.mu.Lock()
	if h.subs[userID] == nil {
		h.subs[userID] = map[chan event]struct{}{}
	}
	h.subs[userID][ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			if set, ok := h.subs[userID]; ok {
				delete(set, ch)
				if len(set) == 0 {
					delete(h.subs, userID)
				}
			}
			h.mu.Unlock()
			close(ch)
		})
	}
	return ch, cancel
}

// broadcast 给指定用户的所有在线通道推送一条事件（非阻塞：通道满则丢弃）。
func (h *hub) broadcast(userID int64, ev event) {
	h.mu.RLock()
	set := h.subs[userID]
	// 复制一份再发，避免持锁期间做发送
	targets := make([]chan event, 0, len(set))
	for ch := range set {
		targets = append(targets, ch)
	}
	h.mu.RUnlock()

	for _, ch := range targets {
		select {
		case ch <- ev:
		default:
			// 该标签页消费不过来，丢弃这条；前端重连/轮询时会补齐
		}
	}
}

// 统一广播入口：先在本进程内分发（所有连接都在本机时立刻送达），
// 再经 Redis 广播给「可能存在的其它实例」。
//
// 为什么两边都要做：
//   · 单实例部署（当前情况）：Redis 只是多一跳，本机分发已经完成投递
//   · 多实例部署：其它实例的 SSE 连接靠 Redis 才能收到
// Redis 不可用时 publish 内部直接返回，等价于「退化成单实例进程内广播」。
func broadcastEvent(toUserID int64, kind string, payload any) {
	eventHub.broadcast(toUserID, event{Type: kind, Data: payload})
	if realtime != nil {
		realtime.publish(kind, userEvent{UserID: toUserID, Data: payload})
	}
}

// userEvent 广播信封：带上「这条事件是给谁的」，其它实例照此找到本地连接。
type userEvent struct {
	UserID int64 `json:"userId"`
	Data   any   `json:"data"`
}

// broadcastChat 推送给「会话里的另一个人」：有新私信。
func broadcastChat(toUserID int64, payload any) {
	broadcastEvent(toUserID, "chat", payload)
}

// broadcastFriend 推送给「被申请的人」：收到好友申请 / 申请被处理。
func broadcastFriend(toUserID int64, payload any) {
	broadcastEvent(toUserID, "friend", payload)
}

// broadcastNotification 推送给「被回复的人」：新的评论回复通知（红点）。
func broadcastNotification(toUserID int64, payload any) {
	broadcastEvent(toUserID, "notification", payload)
}

// broadcastPresence 推送给「关心某人在线状态的人」：上线 / 下线变化。
func broadcastPresence(toUserID int64, payload any) {
	broadcastEvent(toUserID, "presence", payload)
}

// handleEventStream SSE 长连接：GET /api/user/events
//
// 鉴权：EventSource 不能自定义请求头，所以 token 允许放查询参数 ?token=xxx，
// 同时也兼容标准的 Authorization: Bearer（方便用 curl 调试）。
//
// 事件格式（标准 SSE）：
//
//	event: chat
//	data: {"conversationId":3,"message":{…}}
//
// 另外每 25 秒发一行 ":ping" 注释帧做心跳，防止中间的代理/浏览器把空闲连接掐掉。
func handleEventStream(c *gin.Context) {
	u, ok := sseAuthUser(c)
	if !ok {
		return // sseAuthUser 已经写过响应
	}

	// SSE 必需的响应头
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	// 关掉 nginx 等中间代理的缓冲（否则事件会被攒着一起发）
	h.Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, canFlush := c.Writer.(http.Flusher)
	if canFlush {
		flusher.Flush()
	}

	ch, cancel := eventHub.subscribe(u.ID)
	defer cancel()

	// 在线状态：连上即标记在线，断线即清除（Redis 可用时才有意义）。
	// 之后每个心跳都会续期，所以浏览器崩溃 / 断网也会在 presenceTTL 后自动判离线。
	if realtime != nil {
		realtime.touchPresence(u.ID)
		notifyPresenceChange(u.ID, true)
		defer func() {
			// 这条 SSE 断了，但同一个人可能还开着别的标签页（多条 SSE）。
			// 所以不立刻删键，只把 TTL 压到 30 秒：
			//   · 还有连接 → 25 秒的心跳会把 TTL 续回去，别人看到的仍是在线；
			//   · 没有连接了 → 30 秒后自然过期，再补一条「离线」推送。
			realtime.expirePresence(u.ID, 30*time.Second)
			go func(uid int64) {
				time.Sleep(31 * time.Second)
				if !realtime.isOnline(uid) {
					notifyPresenceChange(uid, false)
				}
			}(u.ID)
		}()
	}

	// 首帧：告诉前端连接已就绪（也让 fetch/EventSource 立刻拿到响应头）
	writeSSE(c, event{Type: "ready", Data: gin.H{"userId": u.ID}})
	if canFlush {
		flusher.Flush()
	}

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return // 浏览器关闭页面 / 断网
		case ev, open := <-ch:
			if !open {
				return
			}
			writeSSE(c, ev)
			if canFlush {
				flusher.Flush()
			}
		case <-ping.C:
			// 心跳同时给在线状态续期（25s < presenceTTL，留足容错）
			if realtime != nil {
				realtime.touchPresence(u.ID)
			}
			_, _ = io.WriteString(c.Writer, ": ping\n\n")
			if canFlush {
				flusher.Flush()
			}
		}
	}
}

// notifyPresenceChange 把「某人上线/下线」告诉他的好友，让对方的列表实时更新。
// 在线状态是弱一致的：这里只做「尽力而为」的推送，失败不影响主流程。
func notifyPresenceChange(userID int64, online bool) {
	if realtime == nil {
		return
	}
	rows, err := q.Query(
		`SELECT CASE WHEN small_id = ? THEN big_id ELSE small_id END FROM friends
		 WHERE status = 'accepted' AND (small_id = ? OR big_id = ?)`,
		userID, userID, userID)
	if err != nil {
		return
	}
	defer rows.Close()
	payload := gin.H{"userId": userID, "online": online}
	for rows.Next() {
		var friendID int64
		if err := rows.Scan(&friendID); err != nil {
			return
		}
		broadcastPresence(friendID, payload)
	}
}

// writeSSE 按 SSE 协议写一条事件（data 为单行 JSON，避免多行 data 的解析歧义）。
func writeSSE(c *gin.Context, ev event) {
	payload, err := json.Marshal(ev.Data)
	if err != nil {
		return
	}
	// 去掉换行，保证 data 只有一行
	payload = []byte(strings.ReplaceAll(string(payload), "\n", " "))
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", ev.Type, payload)
}

// sseAuthUser 从 ?token= 或 Authorization 头解析登录用户（专供 SSE 用）。
func sseAuthUser(c *gin.Context) (*User, bool) {
	tokenStr := c.Query("token")
	if tokenStr == "" {
		tokenStr = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	}
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return nil, false
	}
	u, err := userFromToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
		return nil, false
	}
	return u, true
}

// userIDFromString 把查询参数里的用户 ID 转成 int64（供好友查询等接口用）。
func userIDFromString(s string) (int64, bool) {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || v < 1 {
		return 0, false
	}
	return v, true
}
