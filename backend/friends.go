package main

// ============================================================
// friends.go —— 【桃信】好友模块
//
// 接口一览（都在 /api 下，需要登录）：
//   GET    /api/users/search?q=xxx   按 9 位桃信 ID 或昵称搜索用户
//   GET    /api/users/:id            查看某个用户的资料
//   GET    /api/friends              我的好友列表（附带最近一条私信）
//   GET    /api/friends/requests     收到的好友申请 / 我发出的申请
//   POST   /api/friends/request      发起好友申请 {userId:"123456789"}
//   POST   /api/friends/:id/accept   同意申请
//   POST   /api/friends/:id/reject   拒绝申请
//   DELETE /api/friends/:id          删除好友（或撤回申请）
//
// 数据设计：friends 表一行代表「一对用户」，small_id < big_id 保证唯一；
//   requester_id 记录谁发起的申请，于是同一行既能表示「A 申请加 B（待处理）」，
//   也能表示「A 与 B 已是好友」。
// ============================================================

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 好友关系状态
const (
	friendStatusPending  = "pending"
	friendStatusAccepted = "accepted"
	friendStatusRejected = "rejected"
)

// findConversation 查两人之间的会话 ID（没有则返回 0）。
// 只是 chatstore.Store.FindConversation 的一层包装：调用方只关心「有没有会话」，
// 查不到就按「没有会话」处理，不该让错误冒到接口层。
func findConversation(u1, u2 int64) int64 {
	if chatStore == nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conv, err := chatStore.FindConversation(ctx, u1, u2)
	if err != nil || conv == nil {
		return 0
	}
	return conv.ID
}

// pairOrder 把两个用户 ID 归一成 (小, 大)，与 friends / conversations 表的存储约定一致。
func pairOrder(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}

// messengerUserColumns 查询用户时统一用到的列（桃信用户模型 MessengerUser 的字段）。
const messengerUserColumns = `u.id, COALESCE(u.user_id,''), COALESCE(u.nickname,''), COALESCE(u.avatar,''),
	COALESCE(u.level,1), COALESCE(u.role,'user'), u.created_at`

// scanMessengerUser 扫描一行用户（桃信用户模型）。
func scanMessengerUser(row interface{ Scan(...any) error }) (*MessengerUser, error) {
	var u MessengerUser
	if err := row.Scan(&u.ID, &u.UserID, &u.Nickname, &u.Avatar, &u.Level, &u.Role, &u.CreatedAt); err != nil {
		return nil, err
	}
	if u.Nickname == "" {
		u.Nickname = "用户"
	}
	return &u, nil
}

// displayNick 昵称兜底：昵称为空时用邮箱前缀，再空就用「用户」。
func displayNick(u *User) string {
	if u == nil {
		return "用户"
	}
	if u.Nickname != "" {
		return u.Nickname
	}
	if u.Email != "" {
		if i := strings.Index(u.Email, "@"); i > 0 {
			return u.Email[:i]
		}
		return u.Email
	}
	return "用户"
}

// handleSearchUsers 搜索用户：GET /api/users/search?q=xxx
// 支持两种输入：完整的 9 位桃信 ID（数字）或昵称关键字，最多返回 20 条。
// 同时带出「我与这个人的关系」，前端直接决定按钮显示「加好友 / 已申请 / 已是好友 / 发消息」。
func handleSearchUsers(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	kw := strings.TrimSpace(c.Query("q"))
	if kw == "" {
		c.JSON(http.StatusOK, gin.H{"items": []MessengerUser{}})
		return
	}
	if len([]rune(kw)) > 30 {
		kw = string([]rune(kw)[:30])
	}

	// 昵称匹配：昵称为空时退回邮箱前缀（与其它模块的展示规则保持一致）。
	// SPLIT_PART 是 PostgreSQL 的函数（MySQL 里对应 SUBSTRING_INDEX）。
	const nicknameExpr = `COALESCE(NULLIF(u.nickname,''), SPLIT_PART(u.email,'@',1), '')`
	rows, err := q.Query(
		`SELECT `+messengerUserColumns+`
		 FROM users u
		 WHERE u.id <> ? AND (u.user_id LIKE ? OR `+nicknameExpr+` LIKE ?)
		 ORDER BY (u.user_id = ?) DESC, u.id ASC
		 LIMIT 20`,
		me.ID, kw+"%", "%"+kw+"%", kw,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败"})
		return
	}
	defer rows.Close()

	items := []MessengerUser{}
	for rows.Next() {
		u, err := scanMessengerUser(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败"})
			return
		}
		u.Relation = relationBetween(me.ID, u.ID)
		u.ConversationID = findConversation(me.ID, u.ID)
		items = append(items, *u)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// handleGetUser 查看用户资料：GET /api/users/:id
// :id 支持 9 位桃信 ID，也支持内部数字 ID。
func handleGetUser(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	u, err := findUserByParam(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	u.Relation = relationBetween(me.ID, u.ID)
	u.ConversationID = findConversation(me.ID, u.ID)

	// 实时状态：调用方拿到资料卡时顺带知道对方在不在线
	if realtime != nil {
		u.Online = realtime.isOnline(u.ID)
	}
	c.JSON(http.StatusOK, u)
}

// findUserByParam 按「9 位桃信 ID」或「内部数字 ID」查用户。
func findUserByParam(param string) (*MessengerUser, error) {
	param = strings.TrimSpace(param)
	if param == "" {
		return nil, sql.ErrNoRows
	}
	// 先按桃信 ID 查
	u, err := messengerUserByUserID(param)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	// 再按内部数字 ID 查
	id, perr := strconv.ParseInt(param, 10, 64)
	if perr != nil {
		return nil, sql.ErrNoRows
	}
	return messengerUserByInternalID(id)
}

// messengerUserByUserID 按 9 位桃信 ID（users.user_id 列）查用户。
func messengerUserByUserID(userID string) (*MessengerUser, error) {
	row := q.QueryRow(`SELECT `+messengerUserColumns+` FROM users u WHERE u.user_id = ?`, userID)
	return scanMessengerUser(row)
}

// messengerUserByInternalID 按内部自增 ID 查用户。
func messengerUserByInternalID(id int64) (*MessengerUser, error) {
	row := q.QueryRow(`SELECT `+messengerUserColumns+` FROM users u WHERE u.id = ?`, id)
	return scanMessengerUser(row)
}

// relationBetween 返回 me 与 other 的关系：self / friend / pending_out / pending_in / none。
func relationBetween(me, other int64) string {
	if me == other {
		return "self"
	}
	small, big := pairOrder(me, other)
	var requester int64
	var status string
	err := q.QueryRow(
		`SELECT requester_id, status FROM friends WHERE small_id = ? AND big_id = ?`,
		small, big).Scan(&requester, &status)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		return "none"
	}
	switch status {
	case friendStatusAccepted:
		return "friend"
	case friendStatusPending:
		if requester == me {
			return "pending_out"
		}
		return "pending_in"
	default:
		// 被拒绝过：允许重新申请
		return "none"
	}
}

// handleListFriends 好友列表：GET /api/friends
// 返回好友 + 每人最近一条私信 + 未读数，前端左栏直接用。
func handleListFriends(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	rows, err := q.Query(
		`SELECT `+messengerUserColumns+`,
		        COALESCE(m.content, ''), COALESCE(to_char(m.created_at,'YYYY-MM-DD HH24:MI:SS'), ''), COALESCE(m.sender_id, 0),
		        COALESCE(cv.id, 0),
		        COALESCE((
		            SELECT COUNT(*) FROM conversation_members cm
		            JOIN messages mm ON mm.conversation_id = cm.conversation_id AND mm.id > cm.last_read_seq
		            WHERE cm.conversation_id = cv.id AND cm.user_id = ?
		        ), 0)
		 FROM friends f
		 JOIN users u ON u.id = CASE WHEN f.small_id = ? THEN f.big_id ELSE f.small_id END
		 LEFT JOIN conversations cv ON cv.user_a = f.small_id AND cv.user_b = f.big_id
		 LEFT JOIN messages m ON m.id = (SELECT MAX(id) FROM messages WHERE conversation_id = cv.id)
		 WHERE f.status = 'accepted' AND (f.small_id = ? OR f.big_id = ?)
		 ORDER BY m.created_at DESC NULLS LAST, u.id ASC`,
		me.ID, me.ID, me.ID, me.ID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()

	items := []FriendItem{}
	for rows.Next() {
		var (
			u          MessengerUser
			lastMsg    string
			lastAt     string
			lastSender int64
			convID     int64
			unread     int64
		)
		if err := rows.Scan(&u.ID, &u.UserID, &u.Nickname, &u.Avatar, &u.Level, &u.Role, &u.CreatedAt,
			&lastMsg, &lastAt, &lastSender, &convID, &unread); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
			return
		}
		if u.Nickname == "" {
			u.Nickname = "用户"
		}
		u.Relation = "friend"
		u.ConversationID = convID
		if realtime != nil {
			u.Online = realtime.isOnline(u.ID)
		}
		items = append(items, FriendItem{
			MessengerUser: u,
			LastMessage:   lastMsg,
			LastMessageAt: lastAt,
			LastSenderID:  lastSender,
			Unread:        unread,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"items":    items,
		"incoming": incomingRequests(me.ID),
		"outgoing": outgoingRequests(me.ID),
		"realtime": realtime != nil,
	})
}

// handleListFriendRequests 只看申请：GET /api/friends/requests
func handleListFriendRequests(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"incoming": incomingRequests(me.ID),
		"outgoing": outgoingRequests(me.ID),
	})
}

// incomingRequests 别人申请加我（待处理），带上申请人信息与 friends 行 ID。
func incomingRequests(me int64) []FriendRequestItem {
	rows, err := q.Query(
		`SELECT f.id, f.requester_id, `+messengerUserColumns+`
		 FROM friends f
		 JOIN users u ON u.id = f.requester_id
		 WHERE f.status = 'pending' AND f.requester_id <> ? AND (f.small_id = ? OR f.big_id = ?)
		 ORDER BY f.created_at DESC`,
		me, me, me,
	)
	if err != nil {
		return []FriendRequestItem{}
	}
	defer rows.Close()
	out := []FriendRequestItem{}
	for rows.Next() {
		var (
			relationID int64
			requester  int64
			u          MessengerUser
		)
		if err := rows.Scan(&relationID, &requester, &u.ID, &u.UserID, &u.Nickname, &u.Avatar,
			&u.Level, &u.Role, &u.CreatedAt); err != nil {
			return out
		}
		if u.Nickname == "" {
			u.Nickname = "用户"
		}
		out = append(out, FriendRequestItem{
			RelationID: relationID,
			ID:         u.ID,
			UserID:     u.UserID,
			Nickname:   u.Nickname,
			Avatar:     u.Avatar,
			Level:      u.Level,
			Role:       u.Role,
			Status:     friendStatusPending,
			Incoming:   true,
			CreatedAt:  u.CreatedAt,
		})
	}
	return out
}

// outgoingRequests 我发出、对方还没处理的申请。
func outgoingRequests(me int64) []FriendRequestItem {
	rows, err := q.Query(
		`SELECT f.id, CASE WHEN f.small_id = ? THEN f.big_id ELSE f.small_id END AS peer, `+messengerUserColumns+`
		 FROM friends f
		 JOIN users u ON u.id = CASE WHEN f.small_id = ? THEN f.big_id ELSE f.small_id END
		 WHERE f.status = 'pending' AND f.requester_id = ? AND (f.small_id = ? OR f.big_id = ?)
		 ORDER BY f.created_at DESC`,
		me, me, me, me, me,
	)
	if err != nil {
		return []FriendRequestItem{}
	}
	defer rows.Close()
	out := []FriendRequestItem{}
	for rows.Next() {
		var (
			relationID int64
			peer       int64
			u          MessengerUser
		)
		if err := rows.Scan(&relationID, &peer, &u.ID, &u.UserID, &u.Nickname, &u.Avatar,
			&u.Level, &u.Role, &u.CreatedAt); err != nil {
			return out
		}
		if u.Nickname == "" {
			u.Nickname = "用户"
		}
		out = append(out, FriendRequestItem{
			RelationID: relationID,
			ID:         u.ID,
			UserID:     u.UserID,
			Nickname:   u.Nickname,
			Avatar:     u.Avatar,
			Level:      u.Level,
			Role:       u.Role,
			Status:     friendStatusPending,
			Incoming:   false,
			CreatedAt:  u.CreatedAt,
		})
	}
	return out
}

// handleFriendRequest 发起好友申请：POST /api/friends/request  {userId:"123456789"}
// 规则：对方也申请过我 → 直接互为好友；已是好友 → 提示；否则新建 pending（被拒过则重置为 pending）。
func handleFriendRequest(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	target, err := findUserByParam(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if target.ID == me.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能添加自己为好友"})
		return
	}

	small, big := pairOrder(me.ID, target.ID)
	var relationID int64
	var requester int64
	var status string
	err = q.QueryRow(
		`SELECT id, requester_id, status FROM friends WHERE small_id = ? AND big_id = ?`, small, big,
	).Scan(&relationID, &requester, &status)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		// 全新申请（PostgreSQL 用 RETURNING 取回自增主键）
		newID, ierr := insertReturningID(
			`INSERT INTO friends (small_id, big_id, requester_id, status)
			 VALUES (?, ?, ?, 'pending') RETURNING id`,
			small, big, me.ID,
		)
		if ierr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "申请失败"})
			return
		}
		relationID = newID
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "申请失败"})
		return
	case status == friendStatusAccepted:
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": friendStatusAccepted, "message": "你们已经是好友了"})
		return
	case status == friendStatusPending && requester != me.ID:
		// 对方之前申请过我，现在我也申请 → 直接双向确认
		if _, err := q.Exec(
			`UPDATE friends SET status = 'accepted', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, relationID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
			return
		}
		broadcastFriend(target.ID, gin.H{"kind": "accepted", "by": *userBriefOf(me)})
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": friendStatusAccepted, "message": "已互为好友"})
		return
	default:
		// 我自己重复申请 / 之前被拒绝 → 重新置为 pending
		if _, err := q.Exec(
			`UPDATE friends SET requester_id = ?, status = 'pending', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			me.ID, relationID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
			return
		}
	}

	broadcastFriend(target.ID, gin.H{"kind": "request", "by": *userBriefOf(me)})
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": friendStatusPending, "message": "好友申请已发送"})
}

// handleFriendAccept 同意好友申请：POST /api/friends/:id/accept
// :id 是 friends 表的行 ID（前端从申请列表里拿到）。只有被申请方能同意。
func handleFriendAccept(c *gin.Context) {
	handleFriendDecision(c, true)
}

// handleFriendReject 拒绝好友申请：POST /api/friends/:id/reject
func handleFriendReject(c *gin.Context) {
	handleFriendDecision(c, false)
}

// handleFriendDecision 同意 / 拒绝的统一实现。
func handleFriendDecision(c *gin.Context, accept bool) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var small, big, requester int64
	var status string
	err := q.QueryRow(
		`SELECT small_id, big_id, requester_id, status FROM friends WHERE id = ?`, id,
	).Scan(&small, &big, &requester, &status)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "申请不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}
	// 只有「收到申请的人」能处理（发起人不能自己同意自己）
	if requester == me.ID || (small != me.ID && big != me.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权处理该申请"})
		return
	}
	peer := big
	if me.ID == big {
		peer = small
	}

	if accept {
		if _, err := q.Exec(`UPDATE friends SET status = 'accepted', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
			return
		}
		broadcastFriend(peer, gin.H{"kind": "accepted", "by": *userBriefOf(me)})
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已同意好友申请"})
		return
	}
	if _, err := q.Exec(`UPDATE friends SET status = 'rejected', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已拒绝该申请"})
}

// handleFriendDelete 删除好友 / 撤回申请：DELETE /api/friends/:id
// :id 是对方的用户 ID（前端好友列表里点删除时传的）。
func handleFriendDelete(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	peer, ok := parseID(c)
	if !ok {
		return
	}
	small, big := pairOrder(me.ID, peer)
	if _, err := q.Exec(`DELETE FROM friends WHERE small_id = ? AND big_id = ?`, small, big); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已删除好友"})
}

// handleUnreadSummary 未读汇总：GET /api/messenger/unread
// 红点数字 = 未读私信（chatStore）+ 好友申请 + 未读通知。
// 后两项在 PostgreSQL 的 friends / notifications 表里，
// 由 chatstore 的 PG 实现通过注入的回调补齐（见 chatcfg.go）。
func handleUnreadSummary(c *gin.Context) {
	me := authUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	ctx, cancel := storeCtx(c)
	defer cancel()
	un, err := chatStore.UnreadSummary(ctx, me.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"chat":         un.Messages,
		"friend":       un.FriendReqs,
		"notification": un.Notifications,
		"total":        un.Total(),
	})
}

// userBriefOf 把登录用户压成广播用的精简结构。
func userBriefOf(u *User) *UserBrief {
	if u == nil {
		return &UserBrief{}
	}
	return &UserBrief{
		ID:       u.ID,
		UserID:   u.UserID,
		Nickname: displayNick(u),
		Avatar:   u.Avatar,
		Level:    u.Level,
	}
}
