<script setup>
// 桃信聊天面板：标题条（对方昵称 + 桃信 ID）+ 消息列表（对方灰气泡 / 我蓝气泡）+ 输入框。
// 消息来自 SSE 实时推送，发送走 REST，两边都追加到同一个 messages 数组。
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useMessenger } from '../../composables/useMessenger'
import { useToast } from '../../composables/useToast'
import { useAuth } from '../../composables/useAuth'

const {
  active, activeConvId, messages, messagesLoading, hasMore, sending,
  peerOnline, peerTyping, realtimeOn,
  sendMessage, loadOlder, backToList, loadMessages, notifyTyping
} = useMessenger()
const { toast } = useToast()
// 自己的头像 / 昵称（用于「我发的消息」左侧（右侧）那个头像）
const { avatarUrl, displayName } = useAuth()

const draft = ref('')
const scrollBox = ref(null)
const meId = Number(JSON.parse(localStorage.getItem('user') || '{}').id || 0)

// 输入时告诉后端「我正在输入」（内部有 3 秒节流；Redis 未启用时自动跳过）
function onInput() {
  notifyTyping()
}

// 在线状态的文案：正在输入优先于在线
const peerStatus = computed(() => {
  if (!realtimeOn.value) return ''
  if (peerTyping.value) return '正在输入…'
  return peerOnline.value ? '在线' : '离线'
})

// ---------- 时间标记 ----------
// 不再挂在每个气泡下面，而是作为「居中分隔」出现在消息之间，规则：
//   · 1 分钟内  → 不显示（同一串对话不打断）
//   · 1~5 分钟  → 「X 分钟前」
//   · 超过 5 分钟 → 具体时间（当天只给 HH:MM，跨天补上 MM-DD）
// 同一个标记只会出现一次：连续几条消息的时间段相同就不重复刷屏。
const nowTs = ref(Date.now())
let clockTimer = null

function hhmm(s) {
  const d = new Date(s)
  const now = new Date(nowTs.value)
  const hm = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  if (d.toDateString() === now.toDateString()) return hm
  const md = `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  return `${md} ${hm}`
}

// 单条消息对应的标记（空串 = 不显示）
function markOf(m) {
  if (!m || !m.createdAt) return ''
  const age = nowTs.value - new Date(m.createdAt).getTime()
  if (age < 60 * 1000) return ''
  if (age < 5 * 60 * 1000) return `${Math.max(1, Math.floor(age / 60000))} 分钟前`
  return hhmm(m.createdAt)
}

// 真正要渲染的那一条：标记有值、且与上一条不同才显示
const timeMarks = computed(() => {
  let prev = ''
  return messages.value.map((m) => {
    const mark = markOf(m)
    const show = mark && mark !== prev ? mark : ''
    if (mark) prev = mark
    return show
  })
})

// 每 30 秒刷新一次「现在」，让「X 分钟前」不会一直停在打开那一刻
onMounted(() => {
  clockTimer = setInterval(() => { nowTs.value = Date.now() }, 30000)
})
onBeforeUnmount(() => {
  if (clockTimer) clearInterval(clockTimer)
})

const canSend = computed(() => draft.value.trim().length > 0 && !sending.value)

async function submit() {
  if (!canSend.value) return
  const text = draft.value
  draft.value = ''
  const r = await sendMessage(text)
  if (!r.ok) {
    draft.value = text // 失败还原，避免用户白打一遍
    if (r.message) toast.error(r.message)
  }
}

// Enter 发送，Shift+Enter 换行
function onKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
    e.preventDefault()
    submit()
  }
}

// 滚到顶部时自动加载更早的消息
function onScroll() {
  const box = scrollBox.value
  if (!box || !hasMore.value || messagesLoading.value) return
  if (box.scrollTop < 40) {
    const before = box.scrollHeight
    loadOlder().then(() => {
      nextTick(() => {
        // 保持视觉位置不跳：把新增内容的高度补回滚动位置
        box.scrollTop = box.scrollHeight - before
      })
    })
  }
}

// 切换会话时滚到底部
watch(activeConvId, () => {
  nextTick(() => { if (scrollBox.value) scrollBox.value.scrollTop = scrollBox.value.scrollHeight })
})

function refresh() {
  if (activeConvId.value) loadMessages()
}
</script>

<template>
  <div v-if="active" class="chat-pane">
    <!-- 标题条：返回 + 对方昵称 + 桃信 ID -->
    <div class="chat-head">
      <button class="back-btn" title="返回列表" @click="backToList">
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M15 18l-6-6 6-6" />
        </svg>
      </button>
      <span class="chat-avatar">
        <img v-if="active.avatar" :src="active.avatar" alt="" />
        <span v-else class="chat-avatar-letter">{{ (active.nickname || '?').slice(0, 1) }}</span>
        <!-- 在线小绿点（后端启用 Redis 时才显示） -->
        <i v-if="realtimeOn" class="chat-dot" :class="{ on: peerOnline }"></i>
      </span>
      <span class="chat-title">
        <span class="chat-name">
          {{ active.nickname || '用户' }}
          <i v-if="active.role === 'admin'" class="badge-admin">站长</i>
        </span>
        <span class="chat-sub" :class="{ typing: peerTyping }">
          {{ peerStatus ? peerStatus + ' · ' : '' }}Momo Talk ID：{{ active.userId }}
        </span>
      </span>
      <button class="chat-act" title="刷新消息" @click="refresh">
        <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M20 11a8 8 0 1 0-2.3 5.7" /><path d="M20 4v7h-7" />
        </svg>
      </button>
    </div>

    <!-- 消息列表 -->
    <div ref="scrollBox" class="chat-body" data-msg-scroll @scroll.passive="onScroll">
      <div v-if="messagesLoading && !messages.length" class="chat-state">消息加载中…</div>
      <div v-else-if="!messages.length" class="chat-state">
        还没有消息，发一句「在吗」开启聊天吧～
      </div>

      <template v-for="(m, i) in messages" :key="m.id">
        <!-- 时间标记：居中分隔，只在时间段变化时出现 -->
        <div v-if="timeMarks[i]" class="msg-time-sep">{{ timeMarks[i] }}</div>

        <div class="msg-line" :class="{ mine: m.senderId === meId }">
          <!-- 双方都显示头像：对方用会话对象的头像，自己用当前登录用户的头像 -->
          <span class="msg-avatar">
            <template v-if="m.senderId === meId">
              <img v-if="avatarUrl" :src="avatarUrl" alt="" />
              <span v-else class="msg-avatar-letter">{{ (displayName || '我').slice(0, 1) }}</span>
            </template>
            <template v-else>
              <img v-if="active.avatar" :src="active.avatar" alt="" />
              <span v-else class="msg-avatar-letter">{{ (active.nickname || '?').slice(0, 1) }}</span>
            </template>
          </span>
          <span class="msg-stack">
            <span class="msg-bubble">{{ m.content }}</span>
          </span>
        </div>
      </template>
    </div>

    <!-- 输入区 -->
    <div class="chat-input">
      <textarea
        v-model="draft"
        rows="2"
        maxlength="2000"
        placeholder="输入消息，Enter 发送，Shift+Enter 换行"
        @input="onInput"
        @keydown="onKeydown"
      />
      <button class="send-btn" :disabled="!canSend" @click="submit">{{ sending ? '发送中…' : '发送' }}</button>
    </div>
  </div>

  <div v-else class="chat-empty">选择一位好友开始私信</div>
</template>

<style scoped>
.chat-pane { flex: 1; min-width: 0; display: flex; flex-direction: column; background: #fff; }

.chat-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  border-bottom: 1px solid #eef1f5;
  flex-shrink: 0;
}
.back-btn {
  border: none;
  background: none;
  color: #8c96a6;
  cursor: pointer;
  display: inline-flex;
  padding: 4px;
  border-radius: 6px;
}
.back-btn:hover { background: #f2f5f9; color: #2b3445; }
.chat-avatar {
  position: relative;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  overflow: visible;
  background: #f2f5f9;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.chat-avatar img { width: 100%; height: 100%; border-radius: 50%; object-fit: cover; }
.chat-avatar-letter { font-size: 15px; font-weight: 700; color: #8c96a6; }
/* 在线小绿点：压在头像右下角 */
.chat-dot {
  position: absolute;
  right: -1px;
  bottom: -1px;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #c3cbd6;
  border: 2px solid #fff;
}
.chat-dot.on { background: #35c46a; }
.chat-title { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.chat-name { font-size: 14px; font-weight: 700; color: #2b3445; }
.chat-sub { font-size: 11.5px; color: #9aa5b4; font-family: var(--font-mono); }
/* 「正在输入…」用主题粉强调一下 */
.chat-sub.typing { color: #ff5a8a; }
.chat-act {
  border: none;
  background: none;
  color: #9aa5b4;
  cursor: pointer;
  padding: 5px;
  border-radius: 6px;
}
.chat-act:hover { background: #f2f5f9; color: #2b3445; }

/* 消息区：浅灰底，气泡左右分开 */
.chat-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  background: #f7f9fc;
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.chat-state { text-align: center; color: #9aa5b4; font-size: 12.5px; padding: 28px 10px; line-height: 1.9; }

/* 头像与气泡垂直居中对齐 —— 尖角在气泡的垂直中点，头像中心也在同一条线上，
   两者才对得上；间距拉到 14px，给尖角留出位置，气泡不会贴着头像。 */
.msg-line { display: flex; gap: 14px; align-items: center; }
.msg-line.mine { flex-direction: row-reverse; }

.msg-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  overflow: hidden;
  flex-shrink: 0;
  background: #e9eef5;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.msg-avatar img { width: 100%; height: 100%; object-fit: cover; }
.msg-avatar-letter { font-size: 14px; font-weight: 700; color: #8c96a6; }

.msg-stack { display: flex; flex-direction: column; gap: 4px; max-width: 74%; }
.msg-line.mine .msg-stack { align-items: flex-end; }

.msg-bubble {
  position: relative;
  background: #4a5b73;
  color: #fff;
  font-size: 13.5px;
  line-height: 1.65;
  padding: 9px 13px;
  /* 四角都是圆角；尖角单独贴在左侧中部，不再从角上长出来 */
  border-radius: 12px;
  word-break: break-word;
  white-space: pre-wrap;
}
/* 尖角：垂直居中贴住气泡左缘。
   宽度 9px 里只有 7px 露在外面，另外 2px 压进气泡内部、和主体用同一个底色 ——
   两块颜色完全重叠才不会在 Edge/Chrome 上出现一条抗锯齿白缝。 */
.msg-line:not(.mine) .msg-bubble::before {
  content: "";
  position: absolute;
  left: -7px;
  top: 50%;
  margin-top: -6px;
  width: 9px;
  height: 12px;
  background: inherit;
  clip-path: polygon(100% 0, 100% 100%, 0 50%);
}

/* 我发的：蓝色气泡，尖角在右侧中部 */
.msg-line.mine .msg-bubble {
  background: #3aa2e8;
}
.msg-line.mine .msg-bubble::before {
  content: "";
  position: absolute;
  right: -7px;
  top: 50%;
  margin-top: -6px;
  width: 9px;
  height: 12px;
  background: inherit;
  clip-path: polygon(0 0, 0 100%, 100% 50%);
}
/* 时间标记：居中的一小条，把消息按时间段分开（不挂在气泡下面） */
.msg-time-sep {
  align-self: center;
  margin: 2px 0;
  padding: 3px 10px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.75);
  color: #a9b2c0;
  font-size: 11.5px;
  font-family: var(--font-mono);
  line-height: 1.6;
}

.chat-input {
  flex-shrink: 0;
  border-top: 1px solid #eef1f5;
  padding: 10px 14px 12px;
  display: flex;
  gap: 10px;
  align-items: flex-end;
  background: #fff;
}
.chat-input textarea {
  flex: 1;
  min-width: 0;
  border: 1px solid #e5e9f0;
  border-radius: 12px; /* 跟圆润的气泡配套 */
  padding: 8px 11px;
  font-size: 13.5px;
  line-height: 1.6;
  font-family: inherit;
  color: #2b3445;
  outline: none;
  resize: none;
  background: #fbfcfe;
}
.chat-input textarea:focus { border-color: #ff9ebb; background: #fff; }
.send-btn {
  border: none;
  background: #ff5a8a;
  color: #fff;
  border-radius: 12px;
  padding: 9px 20px;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
  font-family: inherit;
  flex-shrink: 0;
}
.send-btn:hover { filter: brightness(1.06); }
.send-btn:disabled { background: #e2e7ef; color: #a9b2c0; cursor: default; }

.chat-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a9b2c0;
  font-size: 13px;
  background: #fff;
}
</style>
