// ============================================================
// useMessenger.js —— 【桃信】私信小窗的全局状态 + 实时事件
//
// 职责：
//   1. 维护窗口状态（关闭 / 开场动画 / 列表 / 资料 / 聊天）—— 窗口是「主页上的小窗口」，
//      不是独立路由页面，所以状态放在模块级，任何组件（导航栏、评论区）都能直接打开它。
//   2. 拉数据：好友列表、好友申请、会话列表、会话消息、角标汇总。
//   3. SSE 长连接：/api/user/events，收到 chat / friend / notification 事件时
//      立即刷新对应数据 —— 这就是「消息实时」的来源（其余接口都是普通 REST）。
//
// 设计要点：所有状态都是模块级单例（和 useAuth / useUi 一样的写法），
// 所以 App.vue 里挂一个 <MessengerWidget> 就够，不需要额外的状态库。
// ============================================================

import { computed, reactive, ref } from 'vue'
import { api, eventStreamUrl } from '../api'

// ---------- 窗口阶段 ----------
// closed   ：窗口关着（只剩右下角的「桃信」小圆钮）
// splash   ：刚打开，屏幕中间只有一颗放大的桃子图标（对应第一张图）
// morph    ：粉色层缩小上移、变成面板顶部，图标淡出（第二张图的开头）
// panel    ：面板完整显示（列表 / 资料 / 聊天）
const stage = ref('closed')

// ---------- 数据 ----------
const friends = ref([]) // 好友（含最近一条私信、未读数）
const incoming = ref([]) // 别人申请加我（待处理）
const outgoing = ref([]) // 我申请加别人
const conversations = ref([]) // 会话列表（含未读数）
const messages = ref([]) // 当前会话的消息（时间正序）
const messagesLoading = ref(false)
const hasMore = ref(false)
const unread = reactive({ chat: 0, friend: 0, notification: 0, total: 0 })

// ---------- 视图状态 ----------
const view = ref('friends') // friends（好友列表 / 未读信息） | profile（某个用户的资料卡）
const listMode = ref('friends') // 左侧栏第二列显示什么：friends（成员）| chats（未读信息）
const profile = ref(null) // 资料卡展示的用户
const active = ref(null) // 当前聊天对象（MessengerUser 形态）
const activeConvId = ref(0) // 当前会话 ID（0=还没建立）
const sending = ref(false)
const error = ref('')

// 实时状态（依赖后端 Redis；未启用 Redis 时两者恒为 false，界面自动不显示）
const peerOnline = ref(false) // 对方是否在线
const peerTyping = ref(false) // 对方是否正在输入
const realtimeOn = ref(false) // 后端是否启用了实时层（据此决定要不要显示在线小绿点）
let typingTimer = null // 「正在输入」的本地过期定时器

// 未读私信总数 = 会话列表里的未读之和（好友列表里的未读是同一份数据）
const totalUnread = computed(() => unread.total || 0)
const chatUnread = computed(() => unread.chat || 0)
const friendRequestCount = computed(() => incoming.value.length)

// ============================================================
// 数据加载
// ============================================================

/** 刷新「消息」按钮的红点（三类未读汇总） */
async function refreshUnread() {
  try {
    const d = await api.messengerUnread()
    unread.chat = d.chat || 0
    unread.friend = d.friend || 0
    unread.notification = d.notification || 0
    unread.total = d.total || 0
  } catch {
    /* 未登录 / 网络异常：忽略，保持上一次的数值 */
  }
}

/** 好友列表 + 好友申请（一次请求全拿到） */
async function loadFriends() {
  try {
    const d = await api.listFriends()
    friends.value = d.items || []
    incoming.value = d.incoming || []
    outgoing.value = d.outgoing || []
    realtimeOn.value = !!d.realtime
    error.value = ''
  } catch (e) {
    error.value = e.message
  }
}

/** 会话列表（含未读数），顺手更新红点汇总 */
async function loadConversations() {
  try {
    const d = await api.listConversations()
    conversations.value = d.items || []
    if (d.unread) {
      unread.chat = d.unread.chat || 0
      unread.friend = d.unread.friend || 0
      unread.notification = d.unread.notification || 0
      unread.total = d.unread.total || 0
    }
  } catch (e) {
    error.value = e.message
  }
}

/** 拉当前会话的消息（首次进入或对方发来新消息时调用） */
async function loadMessages({ scrollToEnd = true } = {}) {
  const convId = activeConvId.value
  if (!convId) return
  messagesLoading.value = true
  const LIMIT = 60
  try {
    const d = await api.listMessages(convId, { limit: LIMIT })
    messages.value = d.items || []
    // 拿满一页说明上面还有更早的消息，允许向上翻页
    hasMore.value = messages.value.length >= LIMIT
    if (d.unread) {
      unread.chat = d.unread.chat || 0
      unread.friend = d.unread.friend || 0
      unread.notification = d.unread.notification || 0
      unread.total = d.unread.total || 0
    }
    if (scrollToEnd) scrollToBottom()
    // 同步对方的在线 / 正在输入状态（后端在 Redis 未启用时会返回 false）
    peerOnline.value = !!d.peerOnline
    peerTyping.value = !!d.peerTyping
  } catch (e) {
    error.value = e.message
  } finally {
    messagesLoading.value = false
  }
}

/** 向上翻页：加载更早的消息（保持滚动位置由组件负责） */
async function loadOlder() {
  if (!activeConvId.value || !messages.value.length) return
  const firstId = messages.value[0].id
  try {
    const d = await api.listMessages(activeConvId.value, { limit: 40, before: firstId })
    const older = d.items || []
    hasMore.value = older.length >= 40
    if (older.length) messages.value = [...older, ...messages.value]
  } catch (e) {
    error.value = e.message
  }
}

// 滚动到底部：等 DOM 更新后再滚，避免消息还没渲染就滚
function scrollToBottom() {
  requestAnimationFrame(() => {
    const box = document.querySelector('[data-msg-scroll]')
    if (box) box.scrollTop = box.scrollHeight
  })
}

// ============================================================
// 窗口开关与导航
// ============================================================

let stageTimers = []

function clearStageTimers() {
  stageTimers.forEach(clearTimeout)
  stageTimers = []
}

/** 打开小窗：先播「桃子从无到有放大」再变形为面板顶部（对应三张截图的动画顺序） */
async function open(opts = {}) {
  clearStageTimers()
  if (stage.value !== 'closed') {
    // 已经开着：直接按需求切到指定视图，不再重播动画
    await applyIntent(opts)
    return
  }
  // 时间点与 MessengerWindow.vue 里的 CSS 动画一一对应：
  //   0ms    窗口放大出现（0.34s）；图标 + 「Momo Talk」文字在 .mz-window 内部，
  //          跟着窗口一起放大，所以到 340ms 时它们已经到位了
  //   340ms  窗口与图标都停稳（画面：一张完整的粉色卡片 + 居中图标）
  //   640ms  粉色区域开始收缩（CSS 里还有 0.01s 延迟，即 650ms 开始收；
  //          收缩 0.26s，910ms 收完），同时中央图标原地缩小消失、
  //          标题栏新图标弹出、左侧灰色栏从左侧滑入
  //   1050ms 定格为完整界面
  //
  // 也就是说「粉色卡片停住」约 0.3s 后立刻收进标题栏；
  // 整段 1.05s（原 1.5s → 1.2s → 1.1s → 1.05s），一次比一次干脆。
  stage.value = 'splash'
  stageTimers.push(setTimeout(() => { stage.value = 'morph' }, 640))
  stageTimers.push(setTimeout(() => { stage.value = 'panel' }, 1050))
  // 动画期间就开始拿数据，等界面露出时列表已经就绪
  loadFriends()
  loadConversations()
  stageTimers.push(setTimeout(() => applyIntent(opts), 400))
}

/** 关闭小窗：倒着播一遍（标题栏回落 → 整张卡片淡出）。
 *  与开场一样收紧了节奏：整张卡片的缩小淡出是 0.26s，
 *  所以 280ms 后就卸载组件，状态复位留到 320ms。 */
function close() {
  clearStageTimers()
  stage.value = 'morph'
  stageTimers.push(setTimeout(() => { stage.value = 'closed' }, 280))
  stageTimers.push(setTimeout(() => {
    view.value = 'friends'
    active.value = null
    activeConvId.value = 0
    messages.value = []
    profile.value = null
  }, 320))
}

function toggle() {
  if (stage.value === 'closed') open()
  else close()
}

/** open() 可以带意图：直接打开好友的聊天 / 某个用户的资料卡 */
async function applyIntent(opts) {
  if (opts.tab === 'chats') listMode.value = 'chats'
  if (opts.tab === 'friends') listMode.value = 'friends'
  if (opts.userId != null) {
    await showProfile(opts.userId, { autoChat: !!opts.autoChat })
  }
}

/** 切到「好友 / 成员」列表（顶部 Tab） */
function showFriends() {
  view.value = 'friends'
  listMode.value = 'friends'
  profile.value = null
}

/** 切到「未读信息」列表（顶部 Tab） */
function showChats() {
  view.value = 'friends'
  listMode.value = 'chats'
  profile.value = null
  loadConversations()
}

/** 打开某个用户的资料卡（userId 支持 9 位桃信 ID / 内部 ID） */
async function showProfile(userId, { autoChat = false } = {}) {
  try {
    const u = await api.getUser(userId)
    profile.value = u
    view.value = 'profile'
    if (autoChat && u.relation !== 'self') await startChat(u)
  } catch (e) {
    error.value = e.message
  }
}

/** 切换会话时重置对方的实时状态，避免把上一个会话的「在线/输入中」带过来 */
function resetPeerState() {
  peerOnline.value = false
  peerTyping.value = false
  clearTimeout(typingTimer)
}

/** 从资料卡进入私信（没有会话就先创建） */
async function startChat(user) {
  const target = user || profile.value
  if (!target || target.relation === 'self') return
  try {
    const d = await api.openConversation(target.userId)
    activeConvId.value = d.conversationId
    active.value = d.peer || target
    resetPeerState()
    peerOnline.value = !!active.value.online
    view.value = 'chat'
    listMode.value = 'chats'
    messages.value = []
    await loadMessages()
    loadConversations()
  } catch (e) {
    error.value = e.message
  }
}

/** 从会话列表点开会话 */
async function openConversation(item) {
  activeConvId.value = item.conversationId
  active.value = { ...item.peer, relation: item.isFriend ? 'friend' : 'none' }
  resetPeerState()
  peerOnline.value = !!item.peer.online
  view.value = 'chat'
  messages.value = []
  await loadMessages()
  // 打开即已读
  api.readConversation(item.conversationId).then((d) => {
    if (d && d.unread) Object.assign(unread, d.unread)
    const row = conversations.value.find((c) => c.conversationId === item.conversationId)
    if (row) row.unread = 0
  }).catch(() => {})
}

/**
 * 告诉后端「我正在输入」。
 * 节流：3 秒内只发一次（Redis 里的键 6 秒过期，对方看到后会自己消失）。
 * 这属于锦上添花的功能，失败静默忽略，不打扰用户。
 */
let lastTypingSent = 0
function notifyTyping() {
  if (!activeConvId.value || !realtimeOn.value) return
  const now = Date.now()
  if (now - lastTypingSent < 3000) return
  lastTypingSent = now
  api.sendTyping(activeConvId.value).catch(() => {})
}

/** 关闭聊天，回到列表 */
function backToList() {
  view.value = 'friends'
  listMode.value = 'chats'
  active.value = null
  activeConvId.value = 0
  messages.value = []
  resetPeerState()
  loadConversations()
  loadFriends()
}

// ============================================================
// 好友操作
// ============================================================

async function addFriend(user) {
  const target = user || profile.value
  if (!target) return { ok: false }
  try {
    const d = await api.requestFriend(target.userId)
    // 后端可能直接判定为「互为好友」（对方也申请过我）
    if (profile.value && profile.value.id === target.id) profile.value.relation = d.status === 'accepted' ? 'friend' : 'pending_out'
    await loadFriends()
    refreshUnread()
    return { ok: true, message: d.message || '好友申请已发送', status: d.status }
  } catch (e) {
    return { ok: false, message: e.message }
  }
}

async function acceptFriend(req) {
  try {
    await api.acceptFriend(req.relationId)
    await loadFriends()
    refreshUnread()
    if (profile.value && profile.value.id === req.id) profile.value.relation = 'friend'
    return { ok: true, message: `已和 ${req.nickname || req.userId} 成为好友` }
  } catch (e) {
    return { ok: false, message: e.message }
  }
}

async function rejectFriend(req) {
  try {
    await api.rejectFriend(req.relationId)
    await loadFriends()
    refreshUnread()
    return { ok: true, message: '已拒绝该申请' }
  } catch (e) {
    return { ok: false, message: e.message }
  }
}

async function removeFriend(user) {
  const target = user || profile.value
  if (!target) return { ok: false }
  try {
    await api.removeFriend(target.id)
    await loadFriends()
    refreshUnread()
    if (profile.value && profile.value.id === target.id) profile.value.relation = 'none'
    return { ok: true, message: '已删除好友' }
  } catch (e) {
    return { ok: false, message: e.message }
  }
}

/** 发一条私信（乐观不需要：后端返回后直接追加，同时 SSE 不推给自己） */
async function sendMessage(content) {
  const text = String(content || '').trim()
  if (!text || !activeConvId.value) return { ok: false }
  sending.value = true
  try {
    const d = await api.sendMessage(activeConvId.value, text)
    messages.value.push(d.message)
    scrollToBottom()
    if (d.unread) Object.assign(unread, d.unread)
    // 会话列表的「最近一条」同步更新（不必再拉一次接口）
    const row = conversations.value.find((c) => c.conversationId === activeConvId.value)
    if (row) {
      row.lastMessage = d.message.content
      row.lastMessageAt = d.message.createdAt
      row.lastSenderId = d.message.senderId
    }
    return { ok: true }
  } catch (e) {
    return { ok: false, message: e.message }
  } finally {
    sending.value = false
  }
}

// ============================================================
// SSE 实时事件流
// ============================================================

let es = null
let retryTimer = null

/** 建立 SSE 连接（重复调用是安全的：已连接则先关掉旧的） */
function connectStream() {
  disconnectStream()
  const url = eventStreamUrl()
  if (!url || typeof EventSource === 'undefined') return

  es = new EventSource(url)

  // 新私信：对方在线时立刻进聊天窗口 / 更新列表
  es.addEventListener('chat', (e) => {
    let payload = {}
    try { payload = JSON.parse(e.data) } catch { return }
    const msg = payload.message
    if (!msg) return
    refreshUnread()
    if (activeConvId.value === payload.conversationId) {
      // 正在和这个人聊天：直接追加，并标记已读
      if (!messages.value.some((m) => m.id === msg.id)) {
        messages.value.push(msg)
        scrollToBottom()
      }
      api.readConversation(activeConvId.value).then((d) => {
        if (d && d.unread) Object.assign(unread, d.unread)
      }).catch(() => {})
    }
    loadConversations()
    loadFriends()
  })

  // 好友相关：新申请 / 被同意 / 被拒绝 / 被删除
  es.addEventListener('friend', () => {
    loadFriends()
    refreshUnread()
    if (profile.value) {
      api.getUser(profile.value.id).then((u) => { profile.value = u }).catch(() => {})
    }
  })

  // 评论回复通知：导航栏铃铛的红点由 UserMenu 自己轮询，这里只更新桃信角标汇总
  es.addEventListener('notification', () => refreshUnread())

  // 「对方正在输入」：由后端经 Redis 推来，6 秒后自动消失
  es.addEventListener('typing', (e) => {
    let payload = {}
    try { payload = JSON.parse(e.data) } catch { return }
    if (payload.conversationId !== activeConvId.value) return
    // 只认「对方」的输入状态
    if (payload.userId && active.value && payload.userId === active.value.id) {
      peerTyping.value = true
      clearTimeout(typingTimer)
      typingTimer = setTimeout(() => { peerTyping.value = false }, 6000)
    }
  })

  // 好友上线 / 下线：更新列表里的在线小绿点
  es.addEventListener('presence', (e) => {
    let payload = {}
    try { payload = JSON.parse(e.data) } catch { return }
    const on = !!payload.online
    friends.value.forEach((f) => { if (f.id === payload.userId) f.online = on })
    conversations.value.forEach((c) => { if (c.peer && c.peer.id === payload.userId) c.peer.online = on })
    if (active.value && active.value.id === payload.userId) peerOnline.value = on
  })

  es.onerror = () => {
    // EventSource 自带重连；这里只在连接彻底关闭时补一个兜底重连
    if (es && es.readyState === 2) {
      clearTimeout(retryTimer)
      retryTimer = setTimeout(connectStream, 5000)
    }
  }
}

function disconnectStream() {
  clearTimeout(retryTimer)
  if (es) {
    es.close()
    es = null
  }
}

// 退出登录时清空所有桃信状态，避免下一个登录的人看到上一个人的会话
function reset() {
  disconnectStream()
  clearStageTimers()
  stage.value = 'closed'
  view.value = 'friends'
  listMode.value = 'friends'
  friends.value = []
  incoming.value = []
  outgoing.value = []
  conversations.value = []
  messages.value = []
  profile.value = null
  active.value = null
  activeConvId.value = 0
  realtimeOn.value = false
  resetPeerState()
  unread.chat = unread.friend = unread.notification = unread.total = 0
}

export function useMessenger() {
  return {
    // 状态
    stage,
    view,
    listMode,
    friends,
    incoming,
    outgoing,
    conversations,
    messages,
    messagesLoading,
    hasMore,
    unread,
    totalUnread,
    chatUnread,
    friendRequestCount,
    profile,
    active,
    activeConvId,
    sending,
    error,
    // 实时状态
    peerOnline,
    peerTyping,
    realtimeOn,
    // 窗口
    open,
    close,
    toggle,
    showFriends,
    showChats,
    showProfile,
    startChat,
    openConversation,
    backToList,
    // 数据
    refreshUnread,
    loadFriends,
    loadConversations,
    loadMessages,
    loadOlder,
    scrollToBottom,
    // 好友
    addFriend,
    acceptFriend,
    rejectFriend,
    removeFriend,
    // 消息
    sendMessage,
    notifyTyping,
    // 实时
    connectStream,
    disconnectStream,
    reset
  }
}
