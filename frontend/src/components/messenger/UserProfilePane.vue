<script setup>
// 桃信右侧「资料卡」：大头像 + 昵称 + 一句话 + 桃信 ID + 加入时间，
// 下方两块信息区（好感等级属性加成 / 属性加成对象，当前均为空）+ 私信 / 加好友按钮。
import { computed, ref } from 'vue'
import { api } from '../../api'
import { useMessenger } from '../../composables/useMessenger'
import { useToast } from '../../composables/useToast'
import logoUrl from '../../others/icon.svg'
import LevelHeart from './LevelHeart.vue'

const { profile, startChat, addFriend, acceptFriend, removeFriend, realtimeOn } = useMessenger()
const { toast } = useToast()
const acting = ref(false)

// 资料卡上的状态文案（后端启用 Redis 实时层时才显示）
const statusText = computed(() => {
  if (!realtimeOn.value) return ''
  if (profile.value?.relation === 'self') return ''
  return profile.value?.online ? '在线' : '离线'
})

async function onPrimary() {
  if (!profile.value) return
  if (profile.value.relation === 'friend' || profile.value.relation === 'none') {
    await startChat(profile.value)
    return
  }
  if (profile.value.relation === 'pending_out') {
    toast.info('好友申请已发送，等待对方同意')
    return
  }
  // pending_in：对方申请过我 → 直接同意
  acting.value = true
  try {
    const d = await api.listFriendRequests()
    const hit = (d.incoming || []).find((r) => r.id === profile.value.id)
    if (!hit) return toast.info('请到「新的朋友」里同意')
    const r = await acceptFriend(hit)
    if (r.ok) { profile.value.relation = 'friend'; toast.success(r.message) } else toast.error(r.message)
  } finally {
    acting.value = false
  }
}

async function onAdd() {
  if (!profile.value) return
  acting.value = true
  const r = await addFriend(profile.value)
  acting.value = false
  r.ok ? toast.success(r.message) : toast.error(r.message)
}

async function onRemove() {
  if (!profile.value) return
  acting.value = true
  const r = await removeFriend(profile.value)
  acting.value = false
  r.ok ? toast.success(r.message) : toast.error(r.message)
}

// 主按钮文案
function primaryLabel() {
  const rel = profile.value?.relation
  if (rel === 'self') return '这是你自己'
  if (rel === 'pending_in') return '同意好友申请'
  if (rel === 'pending_out') return '等待对方同意'
  return '私信'
}
</script>

<template>
  <div v-if="profile" class="profile-pane">
    <div class="p-head">
      <span class="p-avatar">
        <img v-if="profile.avatar" :src="profile.avatar" alt="" />
        <span v-else class="p-avatar-letter">{{ (profile.nickname || '?').slice(0, 1) }}</span>
        <!-- 在线小绿点 -->
        <i v-if="realtimeOn && profile.relation !== 'self'" class="p-online" :class="{ on: profile.online }"></i>
        <!-- 等级爱心压在头像右下角：复用列表里那个组件，样式完全一致 -->
        <LevelHeart class="p-level" :level="profile.level || 1" :size="34" />
      </span>

      <div class="p-name">
        {{ profile.nickname || profile.userId }}
        <i v-if="profile.role === 'admin'" class="badge-admin">站长</i>
        <span v-if="statusText" class="p-status" :class="{ on: profile.online }">{{ statusText }}</span>
      </div>
      <div class="p-sign">
        {{ profile.relation === 'friend' ? '你们已经是好友，随时可以私信～' : '来自凛冬RinEyce 的 Momo Talk 用户' }}
      </div>

      <div class="p-tags">
        <span class="p-tag">Momo Talk ID：{{ profile.userId }}</span>
        <span class="p-tag">加入于 {{ (profile.createdAt || '').slice(0, 10) }}</span>
      </div>

      <div class="p-actions">
        <button class="p-btn primary" :disabled="acting || profile.relation === 'self'" @click="onPrimary">
          {{ profile.relation === 'friend' || profile.relation === 'none' ? '✉ 私信' : primaryLabel() }}
        </button>
        <button
          v-if="profile.relation === 'none'"
          class="p-btn"
          :disabled="acting"
          @click="onAdd"
        >＋ 加好友</button>
        <button
          v-if="profile.relation === 'friend'"
          class="p-btn danger"
          :disabled="acting"
          @click="onRemove"
        >删除好友</button>
      </div>
    </div>

    <!-- 页脚 -->
    <div class="p-foot">
      <!-- icon.svg 是「白色桃子剪影 + 透明底」，白底页面上要垫一块粉色底才看得见 -->
      <span class="p-foot-badge"><img class="p-foot-icon" :src="logoUrl" alt="" /></span>
      <span>Momo Talk · 与站内好友保持联系</span>
    </div>
  </div>

  <div v-else class="profile-empty">选择一位成员查看资料</div>
</template>

<style scoped>
.profile-pane {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  /* 内容整体下移并居中：上下留出较大的空白，视觉重心落在面板中部偏下，
     和参考图里「头像居中、下方信息成组」的观感一致 */
  padding: 48px 26px 32px;
  background: #fff;
}
/* 头像 + 昵称 + 标签 + 按钮整组水平居中，并限制最大宽度，
   避免在宽面板里文字拉得太散 */
.p-head {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  max-width: 360px;
  margin: 0 auto;
}

.p-avatar {
  position: relative;
  width: 96px;
  height: 96px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: #f2f5f9;
  overflow: visible;
}
.p-avatar img { width: 96px; height: 96px; border-radius: 50%; object-fit: cover; display: block; }
.p-avatar-letter { font-size: 38px; font-weight: 700; color: #8c96a6; }
/* 在线小绿点（左上角，右下角留给等级爱心） */
.p-online {
  position: absolute;
  left: 2px;
  top: 6px;
  width: 15px;
  height: 15px;
  border-radius: 50%;
  background: #c3cbd6;
  border: 3px solid #fff;
}
.p-online.on { background: #35c46a; }

/* 头像右下角的等级爱心：位置由这里定，外观全部交给 LevelHeart 组件
   （样式与列表里的爱心完全一致；不再自带 drop-shadow 荧光）。
   注意 LevelHeart 自身是 position: relative，这里必须用 :deep() 提高优先级，
   否则写不进 absolute，爱心会掉到头像右边去。 */
.p-head :deep(.p-level) {
  position: absolute;
  right: -2px;
  bottom: -2px;
}

.p-name {
  margin-top: 10px;
  font-size: 16px;
  font-weight: 800;
  color: #2b3445;
  display: flex;
  align-items: center;
  gap: 6px;
}
/* 在线状态小标签 */
.p-status {
  font-size: 11px;
  font-weight: 600;
  color: #8c96a6;
  background: #f2f5f9;
  border-radius: 999px;
  padding: 1px 8px;
}
.p-status.on { color: #1f9c52; background: #e8f8ee; }
.p-sign { margin-top: 3px; font-size: 12.5px; color: #8c96a6; }
.p-tags { display: flex; gap: 8px; flex-wrap: wrap; justify-content: center; margin-top: 8px; }
.p-tag {
  font-size: 11.5px;
  color: #7d8798;
  background: #f2f5f9;
  border-radius: 999px;
  padding: 2px 10px;
  font-family: var(--font-mono);
}
.p-actions { display: flex; gap: 10px; margin-top: 14px; flex-wrap: wrap; justify-content: center; }
.p-btn {
  border: 1px solid #e2e7ef;
  background: #fff;
  color: #4b5563;
  border-radius: 9px;
  padding: 7px 20px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  font-family: inherit;
  transition: all 0.18s ease;
}
.p-btn:hover { border-color: #c9d2de; color: #2b3445; }
.p-btn.primary { background: #ff5a8a; border-color: #ff5a8a; color: #fff; }
.p-btn.primary:hover { filter: brightness(1.06); color: #fff; }
.p-btn.danger { color: #e5484d; border-color: rgba(229, 72, 77, 0.35); }
.p-btn:disabled { opacity: 0.55; cursor: default; }

.p-foot {
  margin-top: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: #ff8ab0;
  font-size: 11.5px;
}
.p-foot-icon { width: 10px; height: 10px; display: block; }
/* 粉色小方块垫底：把白色桃子剪影「染」成粉色，和白底页面里的文字同一族 */
.p-foot-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 15px;
  height: 15px;
  border-radius: 4px;
  background: linear-gradient(160deg, #ff9db4, #e2567b);
  opacity: 0.85;
}

.profile-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a9b2c0;
  font-size: 13px;
  background: #fff;
}
</style>
