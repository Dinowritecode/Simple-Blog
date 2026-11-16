<script setup>
// 搜索框：输入 9 位桃信 ID（或昵称）找到指定用户，并可直接加好友 / 发消息。
import { ref } from 'vue'
import { api } from '../../api'
import { useMessenger } from '../../composables/useMessenger'
import { useToast } from '../../composables/useToast'
import UserRow from './UserRow.vue'

const { showProfile, addFriend, acceptFriend, startChat } = useMessenger()
const { toast } = useToast()

const keyword = ref('')
const results = ref([])
const searching = ref(false)
const searched = ref(false)

async function doSearch() {
  const q = keyword.value.trim()
  if (!q) return toast.error('请输入对方的 9 位 Momo Talk ID 或昵称')
  searching.value = true
  try {
    const d = await api.searchUsers(q)
    results.value = d.items || []
    searched.value = true
    if (!results.value.length) toast.info('没有找到匹配的用户')
  } catch (e) {
    toast.error(e.message)
  } finally {
    searching.value = false
  }
}

// 搜索结果右侧按钮的文案：按「与我的关系」区分
const relationText = {
  friend: '发消息',
  pending_out: '已申请',
  pending_in: '同意',
  self: '我自己',
  none: '加好友'
}

async function onAction(u) {
  if (u.relation === 'self') return
  if (u.relation === 'friend') {
    await startChat(u)
    return
  }
  if (u.relation === 'pending_out') {
    toast.info('申请已发送，等待对方同意')
    return
  }
  if (u.relation === 'pending_in') {
    // 对方先申请了我：从申请列表里找到对应行再同意
    try {
      const d = await api.listFriendRequests()
      const hit = (d.incoming || []).find((r) => r.id === u.id)
      if (!hit) return toast.info('请到「新的朋友」里同意')
      const r = await acceptFriend(hit)
      if (r.ok) { u.relation = 'friend'; toast.success(r.message) } else toast.error(r.message)
    } catch (e) {
      toast.error(e.message)
    }
    return
  }
  const r = await addFriend(u)
  if (r.ok) {
    u.relation = r.status === 'accepted' ? 'friend' : 'pending_out'
    toast.success(r.message)
  } else {
    toast.error(r.message)
  }
}

function openProfile(u) {
  if (u.relation === 'self') return
  showProfile(u.userId)
}
</script>

<template>
  <div class="search-box">
    <div class="search-input">
      <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
        <circle cx="11" cy="11" r="7" /><path d="M20 20l-3.5-3.5" />
      </svg>
      <input
        v-model="keyword"
        type="text"
        maxlength="30"
        placeholder="输入 9 位 Momo Talk ID 或昵称"
        @keyup.enter="doSearch"
      />
      <button class="search-btn" :disabled="searching" @click="doSearch">
        {{ searching ? '…' : '搜索' }}
      </button>
    </div>

    <div v-if="searched" class="search-results">
      <div v-if="!results.length" class="search-empty">没有找到用户，确认一下对方的 Momo Talk ID？</div>
      <UserRow
        v-for="u in results"
        :key="u.id"
        :user="u"
        :secondary="u.relation === 'self' ? '这是你自己' : `Momo Talk ID：${u.userId}`"
        :show-level="false"
        @select="openProfile"
      >
        <template #right>
          <button
            class="act-btn"
            :class="{ friend: u.relation === 'friend', muted: u.relation === 'pending_out' || u.relation === 'self' }"
            @click.stop="onAction(u)"
          >{{ relationText[u.relation] || '加好友' }}</button>
        </template>
      </UserRow>
    </div>
  </div>
</template>

<style scoped>
.search-box { padding: 10px 12px; border-bottom: 1px solid #eef1f5; }
.search-input {
  display: flex;
  align-items: center;
  gap: 6px;
  background: #f2f5f9;
  border-radius: 9px;
  padding: 6px 8px 6px 10px;
  color: #9aa5b4;
}
.search-input input {
  flex: 1;
  min-width: 0;
  border: none;
  background: none;
  outline: none;
  font-size: 13px;
  color: #2b3445;
  font-family: inherit;
}
.search-input input::placeholder { color: #a9b2c0; }
.search-btn {
  border: none;
  background: #ff5a8a;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  border-radius: 7px;
  padding: 4px 12px;
  cursor: pointer;
  font-family: inherit;
}
.search-btn:disabled { opacity: 0.6; cursor: default; }

.search-results { margin-top: 8px; max-height: 210px; overflow-y: auto; }
.search-empty { text-align: center; color: #9aa5b4; font-size: 12.5px; padding: 16px 8px; }

.act-btn {
  border: none;
  background: #ff5a8a;
  color: #fff;
  border-radius: 7px;
  padding: 4px 11px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
  font-family: inherit;
}
.act-btn.friend { background: #4aa8e0; }
.act-btn.muted { background: #dfe4ec; color: #8c96a6; }
</style>
