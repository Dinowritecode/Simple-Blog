package main

// ============================================================
// rts.go —— 聊天「实时状态层」（Redis）
//
// 为什么需要它：聊天系统的热数据不是「消息正文」，而是这些东西 ——
//   · 谁在线（在线状态）
//   · 谁正在输入（瞬时状态，生命周期只有几秒）
//   · 某个用户这一分钟发了几条（限流计数）
//   · 一条新消息要立刻推给「可能不在这台机器上」的连接（跨实例广播）
// 这些全都是「带过期时间的短命键」或「事件」，放在关系库里既写得多又难清理，
// 正是 Redis 的强项；消息正文仍然只落 PostgreSQL（见 chatstore 包）。
//
// 关键设计：**Redis 是可选加速件，不是必需品**。
//   · 没配 REDIS_ADDR → 整个实时层关闭，接口照常工作（online 一律 false）
//   · 运行中 Redis 掉线 → 每个方法内部自动降级：在线状态返回 false、
//     限流直接放行、广播退回进程内 eventHub
// 这样「Redis 挂了」不会让聊天功能不可用，只是少了在线/输入中提示。
// ============================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// rts 全局实时状态层（未配置 Redis 时为 nil，方法都用值接收者判空）。
type rts struct {
	rdb *redis.Client
	// prefix 键前缀：允许多套环境共用同一个 Redis 而互不干扰
	prefix string
	// instance 本进程的唯一标识：广播时用它跳过自己发的消息（避免重复推送）
	instance string

	// 跨实例广播
	sub      *redis.PubSub
	mu       sync.Mutex
	handlers map[string]func(kind string, payload []byte)
	closed   bool
}

// 常量：键名与过期时间
const (
	// presenceTTL 在线状态有效期。SSE 心跳每 25 秒一次，这里给 3 倍余量：
	// 浏览器崩了 / 断网了，最多 90 秒后就不再显示为在线。
	presenceTTL = 90 * time.Second
	// typingTTL 「正在输入」有效期，客户端每 3 秒续一次
	typingTTL = 6 * time.Second
	// rateWindow 发消息限流窗口
	rateWindow = 10 * time.Second
	// rateLimit 窗口内最多发多少条
	rateLimit = 20

	// channelEvents 跨实例广播的 pub/sub 频道名
	channelEvents = "events"
)

// redisConfig Redis 连接配置（environment 变量，全部有默认值）。
type redisConfig struct {
	Addr     string
	Password string
	DB       int
	Prefix   string
}

// loadRedisConfig 从环境变量读配置。
// 刻意「默认不连」：只有显式设置了 REDIS_ADDR 才启用，
// 避免在没装 Redis 的机器上启动就报错。
func loadRedisConfig() redisConfig {
	db, _ := strconv.Atoi(getenv("REDIS_DB", "0"))
	return redisConfig{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
		Prefix:   getenv("REDIS_PREFIX", "blog:"),
	}
}

// initRTS 建立 Redis 连接并启动订阅。
// 连不上时返回 nil + 日志（不阻断启动：聊天功能降级为「无在线状态」）。
func initRTS(cfg redisConfig) *rts {
	if cfg.Addr == "" {
		log.Printf("未配置 REDIS_ADDR：实时状态层关闭（在线状态/正在输入不可用，聊天与消息不受影响）")
		return nil
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     8,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Redis 连接失败（%s）：实时状态层已关闭，聊天功能继续可用 —— %v", cfg.Addr, err)
		_ = rdb.Close()
		return nil
	}

	r := &rts{
		rdb:      rdb,
		prefix:   cfg.Prefix,
		instance: fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano()),
		handlers: map[string]func(kind string, payload []byte){},
	}
	r.startSubscriber()
	log.Printf("Redis 实时状态层已启用：%s（db=%d, prefix=%s）", cfg.Addr, cfg.DB, cfg.Prefix)
	return r
}

// enabled Redis 是否可用。
func (r *rts) enabled() bool { return r != nil && r.rdb != nil }

// ctx 给所有 Redis 调用一个短超时：Redis 慢不能拖垮聊天接口。
func rtsCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}

// key 拼上统一前缀。
func (r *rts) key(parts ...string) string {
	s := r.prefix
	for i, p := range parts {
		if i > 0 {
			s += ":"
		}
		s += p
	}
	return s
}

// Close 关闭连接与订阅。
func (r *rts) Close() {
	if !r.enabled() {
		return
	}
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	if r.sub != nil {
		_ = r.sub.Close()
	}
	_ = r.rdb.Close()
}

// ------------------------------------------------------------
// 在线状态（presence）
// ------------------------------------------------------------

// touchPresence 续期在线状态（SSE 心跳 + 连接建立时调用）。
func (r *rts) touchPresence(userID int64) {
	if !r.enabled() {
		return
	}
	ctx, cancel := rtsCtx()
	defer cancel()
	key := r.key("online", strconv.FormatInt(userID, 10))
	if err := r.rdb.Set(ctx, key, time.Now().Unix(), presenceTTL).Err(); err != nil {
		r.warn("续期在线状态", err)
	}
}

// clearPresence 主动下线（SSE 断开时调用）。
func (r *rts) clearPresence(userID int64) {
	if !r.enabled() {
		return
	}
	ctx, cancel := rtsCtx()
	defer cancel()
	if err := r.rdb.Del(ctx, r.key("online", strconv.FormatInt(userID, 10))).Err(); err != nil {
		r.warn("清除在线状态", err)
	}
}

// expirePresence 把在线状态的剩余有效期压短（SSE 断开时用，替代直接删除）。
//
// 为什么要这样：同一个人可能同时开着几个标签页，每个标签页都是一条独立 SSE。
// 原来一断开就直接 DEL，关掉其中一个标签页会把「还开着的那个」也判成离线。
// 改成「只把 TTL 压到 30 秒」——
//   · 如果还有别的连接，它 25 秒一次的心跳会把 TTL 续回去，在线状态不受影响；
//   · 如果确实没有连接了，30 秒后自动过期，跟原来一样会变离线，只是晚一点。
func (r *rts) expirePresence(userID int64, ttl time.Duration) {
	if !r.enabled() {
		return
	}
	ctx, cancel := rtsCtx()
	defer cancel()
	if err := r.rdb.Expire(ctx, r.key("online", strconv.FormatInt(userID, 10)), ttl).Err(); err != nil {
		r.warn("缩短在线状态有效期", err)
	}
}

// isOnline 判断某个用户是否在线。
func (r *rts) isOnline(userID int64) bool {
	if !r.enabled() {
		return false
	}
	ctx, cancel := rtsCtx()
	defer cancel()
	n, err := r.rdb.Exists(ctx, r.key("online", strconv.FormatInt(userID, 10))).Result()
	if err != nil {
		r.warn("查询在线状态", err)
		return false
	}
	return n > 0
}

// onlineSet 批量判断在线状态（好友列表一次拿到，避免 N 次往返）。
// 用一次 pipeline 把所有 EXISTS 打包，网络往返只有 1 次。
func (r *rts) onlineSet(userIDs []int64) map[int64]bool {
	out := make(map[int64]bool, len(userIDs))
	if !r.enabled() || len(userIDs) == 0 {
		return out
	}
	ctx, cancel := rtsCtx()
	defer cancel()
	pipe := r.rdb.Pipeline()
	cmds := make(map[int64]*redis.IntCmd, len(userIDs))
	for _, id := range userIDs {
		cmds[id] = pipe.Exists(ctx, r.key("online", strconv.FormatInt(id, 10)))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		r.warn("批量查询在线状态", err)
		return out
	}
	for id, cmd := range cmds {
		out[id] = cmd.Val() > 0
	}
	return out
}

// ------------------------------------------------------------
// 正在输入（typing）
// ------------------------------------------------------------

// setTyping 标记「某人正在给某个会话输入」。
func (r *rts) setTyping(convID, userID int64) {
	if !r.enabled() {
		return
	}
	ctx, cancel := rtsCtx()
	defer cancel()
	key := r.key("typing", strconv.FormatInt(convID, 10), strconv.FormatInt(userID, 10))
	if err := r.rdb.Set(ctx, key, 1, typingTTL).Err(); err != nil {
		r.warn("写入正在输入", err)
	}
}

// isTyping 判断「对方是否正在这个会话里输入」。
func (r *rts) isTyping(convID, userID int64) bool {
	if !r.enabled() {
		return false
	}
	ctx, cancel := rtsCtx()
	defer cancel()
	n, err := r.rdb.Exists(ctx, r.key("typing", strconv.FormatInt(convID, 10), strconv.FormatInt(userID, 10))).Result()
	if err != nil {
		r.warn("查询正在输入", err)
		return false
	}
	return n > 0
}

// ------------------------------------------------------------
// 发消息限流
// ------------------------------------------------------------

// allowSend 判断是否允许再发一条消息（固定窗口计数）。
// 返回 (是否放行, 还需要等待的秒数)。
// 刻意「失败即放行」：Redis 出问题不能导致谁都发不出消息。
func (r *rts) allowSend(userID int64) (bool, int) {
	if !r.enabled() {
		return true, 0
	}
	ctx, cancel := rtsCtx()
	defer cancel()
	key := r.key("rate", strconv.FormatInt(userID, 10), strconv.FormatInt(time.Now().Unix()/int64(rateWindow.Seconds()), 10))
	n, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		r.warn("限流计数", err)
		return true, 0
	}
	if n == 1 {
		// 第一次计数时设置过期，窗口结束自动清理
		_ = r.rdb.Expire(ctx, key, rateWindow).Err()
	}
	if n > rateLimit {
		ttl, _ := r.rdb.TTL(ctx, key).Result()
		wait := int(ttl.Seconds())
		if wait < 1 {
			wait = 1
		}
		return false, wait
	}
	return true, 0
}

// ------------------------------------------------------------
// 跨实例广播（pub/sub）
// ------------------------------------------------------------

// rtsEnvelope 广播信封：包一层是为了带上「谁发的」，避免自己收到自己的消息。
type rtsEnvelope struct {
	From    string          `json:"from"`    // 发送方实例 ID
	Kind    string          `json:"kind"`    // 事件类型：chat / friend / notification / …
	Payload json.RawMessage `json:"payload"` // 原始事件体
}

// publish 把事件广播给所有实例（本机由订阅回调再分发给本地 SSE 连接）。
func (r *rts) publish(kind string, payload any) {
	if !r.enabled() {
		return
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	env, err := json.Marshal(rtsEnvelope{From: r.instance, Kind: kind, Payload: raw})
	if err != nil {
		return
	}
	ctx, cancel := rtsCtx()
	defer cancel()
	if err := r.rdb.Publish(ctx, r.key(channelEvents), env).Err(); err != nil {
		r.warn("广播事件", err)
	}
}

// onEvent 注册「收到其它实例广播」时的处理函数（本文件外只需注册一次）。
func (r *rts) onEvent(fn func(kind string, payload []byte)) {
	if !r.enabled() {
		return
	}
	r.mu.Lock()
	r.handlers["*"] = fn
	r.mu.Unlock()
}

// startSubscriber 启动订阅循环，断线自动重连（go-redis 的 PubSub 自带重连）。
func (r *rts) startSubscriber() {
	r.sub = r.rdb.Subscribe(context.Background(), r.key(channelEvents))
	go func() {
		ch := r.sub.Channel()
		for msg := range ch {
			var env rtsEnvelope
			if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
				continue
			}
			if env.From == r.instance {
				continue // 自己发的，本机已经直接推过了
			}
			r.mu.Lock()
			fn := r.handlers["*"]
			r.mu.Unlock()
			if fn != nil {
				fn(env.Kind, env.Payload)
			}
		}
	}()
}

// warn 统一打日志（Redis 故障只提示，不 panic、不返回错误给用户）。
func (r *rts) warn(what string, err error) {
	if err == nil || err == redis.Nil {
		return
	}
	log.Printf("Redis %s 失败（已降级）：%v", what, err)
}
