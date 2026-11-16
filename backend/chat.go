package main

// ============================================================
// chat.go —— 【桃信】私信模块（HTTP 层）
//
// 提供：
//   GET  /api/conversations                我的会话列表（含未读数、最近一条私信）
//   POST /api/conversations                打开与某人的会话（不存在则创建）{peerId}
//   GET  /api/conversations/:id/messages   会话内的消息（分页，默认最近 50 条）
//   POST /api/conversations/:id/messages   发一条私信 {content}
//   POST /api/conversations/:id/read       把会话标记为已读
//   POST /api/conversations/:id/typing     「我正在输入」（走 Redis，瞬时状态）
//
// 分层说明（本次重构的重点）：
//   · 消息与会话的「怎么存」全部下沉到 chatstore.Store（PostgreSQL 分区表实现）
//   · 「短命状态」（在线、正在输入、限流计数）走 rts（Redis），不可用时自动降级
//   · 本文件只负责：鉴权 → 参数校验 → 调 store/rts → 组装 JSON → 推送实时事件
// ============================================================

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"rineyce/blog/chatstore"
)

// maxMessageLen 单条私信最长字符数（与评论保持一致）
const maxMessageLen = 2000

// nowString 当前时间，格式与数据库时间戳一致（2026-09-25 21:42:35）。
// 用于 SSE 推送的事件体，让前端不必再拉一次接口就能显示正确时间。
func nowString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// storeCtx 给存储调用一个超时上限，避免慢查询把 HTTP 连接拖死。
func storeCtx(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), 5*time.Second)
}

// storeErr 把存储层的错误统一转成 HTTP 响应。
// ErrNotFound → 404（对越权也返回 404，不泄露「会话存在」这一信息）
//
// 注意：一定要把**原始错误**打进日志。前端只能看到「查询失败」这种笼统提示，
// 之前排查线上问题时就是因为这里把 err 吞了，日志里什么都没有。
func storeErr(c *gin.Context, err error, notFoundMsg string) {
	if errors.Is(err, chatstore.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": notFoundMsg})
		return
	}
	log.Printf("聊天存储错误 %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
}

// ------------------------------------------------------------
// 会话列表
// ------------------------------------------------------------

// handleListConversations 我的会话列表：GET /api/conversations
func handleListConversations(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	// PostgreSQL：SUBSTRING_INDEX -> SPLIT_PART；IF() -> CASE WHEN
	const nicknameExpr = `COALESCE(NULLIF(u.nickname,''), SPLIT_PART(u.email,'@',1), '')`
	rows, err := q.Query(
		`SELECT cv.id,
		        u.id, COALESCE(u.user_id,''), `+nicknameExpr+`, u.avatar, COALESCE(u.level,1), u.role,
		        COALESCE(m.content,''), COALESCE(to_char(m.created_at,'YYYY-MM-DD HH24:MI:SS'),''), COALESCE(m.sender_id,0),
		        COALESCE(unread.cnt,0),
		        EXISTS(SELECT 1 FROM friends f
		               WHERE f.status = 'accepted'
		                 AND f.small_id = LEAST(cv.user_a, cv.user_b)
		                 AND f.big_id = GREATEST(cv.user_a, cv.user_b)) AS is_friend
		 FROM conversations cv
		 JOIN users u ON u.id = CASE WHEN cv.user_a = ? THEN cv.user_b ELSE cv.user_a END
		 LEFT JOIN messages m ON m.id = (SELECT MAX(id) FROM messages WHERE conversation_id = cv.id)
		 LEFT JOIN (
		     SELECT cm.conversation_id, COUNT(*) AS cnt
		     FROM conversation_members cm
		     JOIN messages mm ON mm.conversation_id = cm.conversation_id AND mm.id > cm.last_read_seq
		     WHERE cm.user_id = ?
		     GROUP BY cm.conversation_id
		 ) unread ON unread.conversation_id = cv.id
		 WHERE cv.user_a = ? OR cv.user_b = ?
		 ORDER BY (m.created_at IS NULL), m.created_at DESC, cv.id DESC
		 LIMIT 200`,
		me.ID, me.ID, me.ID, me.ID,
	)
	if err != nil {
		log.Printf("会话列表查询失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()

	items := []ConversationItem{}
	peerIDs := []int64{}
	for rows.Next() {
		var it ConversationItem
		var nickname, avatar, role string
		var level int64
		var lastContent, lastAt string
		var isFriend bool
		if err := rows.Scan(&it.ConversationID,
			&it.Peer.ID, &it.Peer.UserID, &nickname, &avatar, &level, &role,
			&lastContent, &lastAt, &it.LastSenderID, &it.Unread, &isFriend); err != nil {
			log.Printf("会话列表扫描失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
			return
		}
		it.Peer.Nickname, it.Peer.Avatar, it.Peer.Role = nickname, avatar, role
		it.Peer.Level = int(level)
		if it.Peer.Level < 1 {
			it.Peer.Level = 1
		}
		it.LastMessage, it.LastMessageAt, it.IsFriend = lastContent, lastAt, isFriend
		items = append(items, it)
		peerIDs = append(peerIDs, it.Peer.ID)
	}
	if err := rows.Err(); err != nil {
		log.Printf("会话列表迭代失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 在线状态：一次 pipeline 批量查（Redis 不可用时返回空 map，全为离线）
	online := map[int64]bool{}
	if realtime != nil {
		online = realtime.onlineSet(peerIDs)
	}
	for i := range items {
		items[i].Peer.Online = online[items[i].Peer.ID]
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "unread": unreadSummaryJSON(me.ID), "realtime": realtime != nil})
}

// ------------------------------------------------------------
// 打开会话
// ------------------------------------------------------------

// handleOpenConversation 打开与某人的会话：POST /api/conversations
// 请求体: {"peerId": "123456789"}（9 位桃信 ID 或内部数字 ID）
func handleOpenConversation(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	var req struct {
		PeerID string `json:"peerId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	peer, err := findUserByParam(req.PeerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if peer.ID == me.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能和自己私信"})
		return
	}

	ctx, cancel := storeCtx(c)
	defer cancel()
	conv, err := chatStore.EnsureConversation(ctx, me.ID, peer.ID)
	if err != nil {
		storeErr(c, err, "会话不存在")
		return
	}

	peer.Relation = relationBetween(me.ID, peer.ID)
	peer.ConversationID = conv.ID
	if realtime != nil {
		peer.Online = realtime.isOnline(peer.ID)
	}
	c.JSON(http.StatusOK, gin.H{"conversationId": conv.ID, "peer": peer})
}

// ------------------------------------------------------------
// 消息读取 / 发送
// ------------------------------------------------------------

// handleListMessages 会话消息：GET /api/conversations/:id/messages?before=<id>&limit=50
// 返回按时间正序的消息数组；顺带把「我」标记为已读到最新一条（打开聊天即已读）。
func handleListMessages(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	convID, ok := parseChatID(c)
	if !ok {
		return
	}

	ctx, cancel := storeCtx(c)
	defer cancel()

	peerID, err := chatStore.Peer(ctx, convID, me.ID)
	if err != nil {
		storeErr(c, err, "会话不存在")
		return
	}

	limit := 50
	if v, ok := userIDFromString(c.Query("limit")); ok && v > 0 && v <= 200 {
		limit = int(v)
	}
	var before int64
	if v, ok := userIDFromString(c.Query("before")); ok {
		before = v
	}

	items, err := chatStore.ListMessages(ctx, convID, before, limit)
	if err != nil {
		storeErr(c, err, "会话不存在")
		return
	}

	// 组装成对外模型
	out := make([]DirectMessage, 0, len(items))
	for _, m := range items {
		out = append(out, DirectMessage{
			ID:             m.ID,
			ConversationID: m.ConversationID,
			SenderID:       m.SenderID,
			SenderUserID:   m.SenderUserID,
			Content:        m.Content,
			CreatedAt:      m.CreatedAt,
		})
	}

	// 读到消息即视为已读
	if before == 0 {
		_ = chatStore.MarkRead(ctx, convID, me.ID)
	}

	resp := gin.H{
		"items":          out,
		"conversationId": convID,
		"unread":         unreadSummaryJSON(me.ID),
	}
	// 对方的在线 / 正在输入状态（Redis 不可用时恒为 false）
	if realtime != nil {
		resp["peerOnline"] = realtime.isOnline(peerID)
		resp["peerTyping"] = realtime.isTyping(convID, peerID)
	} else {
		resp["peerOnline"] = false
		resp["peerTyping"] = false
	}
	c.JSON(http.StatusOK, resp)
}

// handleSendMessage 发私信：POST /api/conversations/:id/messages
// 请求体: {"content": "..."}
func handleSendMessage(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	convID, ok := parseChatID(c)
	if !ok {
		return
	}

	ctx, cancel := storeCtx(c)
	defer cancel()

	peerID, err := chatStore.Peer(ctx, convID, me.ID)
	if err != nil {
		storeErr(c, err, "会话不存在")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "消息不能为空"})
		return
	}
	if len([]rune(content)) > maxMessageLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "消息最多 2000 字"})
		return
	}

	// 限流：Redis 可用时按用户做固定窗口计数（失败即放行，不影响可用性）
	if realtime != nil {
		if allowed, wait := realtime.allowSend(me.ID); !allowed {
			c.Header("Retry-After", strconv.Itoa(wait))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "发送太频繁了，请 " + strconv.Itoa(wait) + " 秒后再试",
			})
			return
		}
	}

	msg, err := chatStore.AppendMessage(ctx, convID, me.ID, content)
	if err != nil {
		storeErr(c, err, "会话不存在")
		return
	}
	// 自己发的当然算已读
	_ = chatStore.MarkRead(ctx, convID, me.ID)

	dto := DirectMessage{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		SenderID:       msg.SenderID,
		SenderUserID:   msg.SenderUserID,
		Content:        msg.Content,
		CreatedAt:      msg.CreatedAt,
	}

	// 实时推给对方的聊天窗口
	broadcastChat(peerID, gin.H{
		"conversationId": convID,
		"message":        dto,
		"from":           *userBriefOf(me),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": dto, "unread": unreadSummaryJSON(me.ID)})
}

// handleReadConversation 标记会话已读：POST /api/conversations/:id/read
func handleReadConversation(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	convID, ok := parseChatID(c)
	if !ok {
		return
	}
	ctx, cancel := storeCtx(c)
	defer cancel()
	if _, err := chatStore.Peer(ctx, convID, me.ID); err != nil {
		storeErr(c, err, "会话不存在")
		return
	}
	if err := chatStore.MarkRead(ctx, convID, me.ID); err != nil {
		storeErr(c, err, "操作失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "unread": unreadSummaryJSON(me.ID)})
}

// handleTyping 我正在输入：POST /api/conversations/:id/typing
// 这是一个「瞬时状态」接口：只写 Redis（6 秒过期），不落库、不产生历史记录。
// 前端在输入框里打字时节流调用（约每 3 秒一次），对方可通过 SSE 或轮询看到。
func handleTyping(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	convID, ok := parseChatID(c)
	if !ok {
		return
	}
	ctx, cancel := storeCtx(c)
	defer cancel()
	peerID, err := chatStore.Peer(ctx, convID, me.ID)
	if err != nil {
		storeErr(c, err, "会话不存在")
		return
	}

	// Redis 不可用就直接返回 ok:false —— 前端据此不显示「正在输入」，不算错误
	if realtime == nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "reason": "realtime-disabled"})
		return
	}
	realtime.setTyping(convID, me.ID)
	// 顺手告诉对方「我在输入」（走 pub/sub，对方在线即刻可见）
	broadcastEvent(peerID, "typing", gin.H{"conversationId": convID, "userId": me.ID})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// parseChatID 解析路径里的会话 ID（与 parseID 类似，但错误文案贴合会话场景）。
func parseChatID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的会话 ID"})
		return 0, false
	}
	return id, true
}

// unreadSummaryJSON 未读汇总（走存储层，Redis 不可用时也能算）。
func unreadSummaryJSON(me int64) gin.H {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u, err := chatStore.UnreadSummary(ctx, me)
	if err != nil {
		return gin.H{"chat": 0, "friend": 0, "notification": 0, "total": 0}
	}
	return gin.H{
		"chat":         u.Messages,
		"friend":       u.FriendReqs,
		"notification": u.Notifications,
		"total":        u.Total(),
	}
}
