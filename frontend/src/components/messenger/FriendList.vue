<script setup>
// 桃信左侧第二列：
//   顶部搜索框（9 位 ID / 昵称找人）
//   ① 新的朋友：待处理的好友申请（红点计数，可同意 / 拒绝）
//   ② 成员：好友列表（点一行 → 右侧资料卡 → 私信按钮）
//   ③ 未读信息：会话列表（点一行 → 直接进聊天）
import { computed } from 'vue'
import { useMessenger } from '../../composables/useMessenger'
import { useToast } from '../../composables/useToast'
import UserRow from './UserRow.vue'
import UserSearchBox from './UserSearchBox.vue'

const {
  friends, incoming, conversations, listMode, activeConvId, unread,
  realtimeOn, peerTyping,
  showProfile, openConversation, acceptFriend, rejectFriend
} = useMessenger()
const { toast } = useToast()

const chatUnreadTotal = computed(() => unread.chat || 0)

// 在线小绿点只在后端启用了实时层（Redis）时才显示；否则传 null 让 UserRow 隐藏它
function onlineFlag(user) {
  return realtimeOn.value ? !!user.online : null
}

async function accept(req) {
  const r = await acceptFriend(req)
  r.ok ? toast.success(r.message) : toast.error(r.message)
}
async function reject(req) {
  const r = await rejectFriend(req)
  r.ok ? toast.info(r.message) : toast.error(r.message)
}

function preview(item) {
  if (!item.lastMessage) return '还没有聊过，打个招呼吧～'
  return item.lastMessage
}

function previewPrefix(item, meId) {
  return item.lastSenderId && item.lastSenderId === meId ? '我：' : ''
}

// 当前登录用户 ID：用于把「我发的」消息标成「我：」
const meId = Number(JSON.parse(localStorage.getItem('user') || '{}').id || 0)
</script>

<template>
  <div class="list-col">
    <UserSearchBox />

    <!-- 新的朋友：好友申请（未处理时置顶，按钮直接同意 / 拒绝） -->
    <div v-if="incoming.length" class="group">
      <div class="group-title">
        <span class="dot-red"></span>新的朋友
        <span class="group-count">{{ incoming.length }}</span>
      </div>
      <div v-for="r in incoming" :key="r.relationId" class="req-row">
        <UserRow
          :user="{ nickname: r.nickname, avatar: r.avatar, level: r.level, role: r.role, userId: r.userId }"
          secondary="请求添加你为好友"
          :clickable="false"
        >
          <template #right>
            <span class="req-actions">
              <button class="mini primary" @click.stop="accept(r)">同意</button>
              <button class="mini" @click.stop="reject(r)">拒绝</button>
            </span>
          </template>
        </UserRow>
      </div>
    </div>

    <!-- 成员（好友列表） -->
    <template v-if="listMode === 'friends'">
      <div class="group-title">
        成员({{ friends.length }})
      </div>
      <div v-if="!friends.length" class="list-empty">
        还没有好友。<br />用上面的搜索框输入对方的 9 位 Momo Talk ID 加好友吧～
      </div>
      <UserRow
        v-for="f in friends"
        :key="f.id"
        :user="f"
        :secondary="f.lastMessage || '点开看看 TA 的资料'"
        :unread="f.unread"
        :online="onlineFlag(f)"
        @select="showProfile(f.userId)"
      />
    </template>

    <!-- 未读信息（会话列表） -->
    <template v-else>
      <div class="group-title">
        未读信息({{ chatUnreadTotal }})
      </div>
      <div v-if="!conversations.length" class="list-empty">
        还没有私信记录。<br />点开一位成员，按「私信」开始聊天～
      </div>
      <UserRow
        v-for="c in conversations"
        :key="c.conversationId"
        :user="c.peer"
        :secondary="preview(c)"
        :prefix="previewPrefix(c, meId)"
        :unread="c.unread"
        :selected="c.conversationId === activeConvId"
        :online="onlineFlag(c.peer)"
        :typing="c.conversationId === activeConvId && peerTyping"
        :show-level="false"
        @select="openConversation(c)"
      />
    </template>
  </div>
</template>

<style scoped>
.list-col {
  width: 320px;
  flex-shrink: 0;
  background: #fff;
  border-right: 1px solid #eef1f5;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  padding-bottom: 10px;
}

.group { padding: 0 4px 4px; }
/* 分组标题不吸顶：列表很短，吸顶反而会盖住上面的搜索结果 */
.group-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  color: #7d8798;
  font-weight: 600;
  padding: 12px 14px 6px;
}
.group-count {
  font-family: var(--font-mono);
  font-size: 11px;
  color: #fff;
  background: #ff5a8a;
  border-radius: 999px;
  padding: 0 6px;
  line-height: 16px;
}
.dot-red {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #ff5a8a;
  box-shadow: 0 0 0 3px rgba(255, 90, 138, 0.16);
}

.req-row { padding: 0 6px; }
.req-actions { display: inline-flex; gap: 5px; }
.mini {
  border: 1px solid #e2e7ef;
  background: #fff;
  color: #6b7280;
  font-size: 11.5px;
  border-radius: 7px;
  padding: 3px 9px;
  cursor: pointer;
  font-family: inherit;
}
.mini:hover { border-color: #c9d2de; color: #2b3445; }
.mini.primary { background: #ff5a8a; border-color: #ff5a8a; color: #fff; }
.mini.primary:hover { filter: brightness(1.06); color: #fff; }

.list-empty {
  padding: 22px 18px;
  text-align: center;
  color: #9aa5b4;
  font-size: 12.5px;
  line-height: 1.9;
}
</style>
