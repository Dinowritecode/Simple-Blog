<script setup>
// Momo Talk 列表里的一行：头像 + 昵称 + 一句话 + 右侧等级爱心（数字在爱心里）。
// 好友列表、未读信息列表、搜索结果三种场景共用，用 slot 决定右侧显示什么。
import LevelHeart from './LevelHeart.vue'

const props = defineProps({
  user: { type: Object, required: true }, // { nickname, avatar, level, role, userId, online }
  secondary: { type: String, default: '' }, // 第二行文字（签名 / 最近一条私信）
  prefix: { type: String, default: '' }, // 第二行前缀（如「我：」）
  unread: { type: Number, default: 0 },
  selected: { type: Boolean, default: false },
  clickable: { type: Boolean, default: true },
  // 是否显示在线小绿点：null = 不显示（后端没启用 Redis 实时层）
  online: { type: Boolean, default: null },
  // 「正在输入…」替代第二行文字
  typing: { type: Boolean, default: false },
  // 是否在行尾显示等级爱心（搜索结果里右侧要给「加好友」按钮，所以传 false）
  showLevel: { type: Boolean, default: true }
})

const emit = defineEmits(['select'])

function onSelect() {
  if (props.clickable) emit('select', props.user)
}

function letter(name) {
  return (name || '?').slice(0, 1)
}
</script>

<template>
  <button
    type="button"
    class="m-row"
    :class="{ selected }"
    @click="onSelect"
  >
    <span class="m-avatar">
      <img v-if="user.avatar" :src="user.avatar" alt="" />
      <span v-else class="m-avatar-letter">{{ letter(user.nickname) }}</span>
      <!-- 在线小绿点：user.online 由后端 Redis 提供，未启用实时层时恒为 false -->
      <i v-if="online !== null" class="m-online" :class="{ on: online }"></i>
    </span>

    <span class="m-row-body">
      <span class="m-row-top">
        <span class="m-name">
          {{ user.nickname || user.userId || '用户' }}
          <i v-if="user.role === 'admin'" class="badge-admin">站长</i>
        </span>
        <!-- 行尾：未读气泡 + 等级爱心（爱心固定在行末，气泡往左排） -->
        <span class="m-right">
          <slot name="right">
            <span v-if="unread > 0" class="m-unread">{{ unread > 99 ? '99+' : unread }}</span>
          </slot>
          <LevelHeart v-if="showLevel" :level="user.level || 1" :size="34" />
        </span>
      </span>
      <span class="m-row-bottom">
        <span class="m-msg" :class="{ typing }">
          <template v-if="typing">正在输入…</template>
          <template v-else><em v-if="prefix">{{ prefix }}</em>{{ secondary || '还没有聊过，打个招呼吧～' }}</template>
        </span>
      </span>
    </span>
  </button>
</template>

<style scoped>
.m-row {
  position: relative; /* 行尾等级爱心绝对定位的锚点 */
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 10px;
  border: none;
  background: none;
  border-radius: 8px;
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease;
  font-family: inherit;
}
.m-row:hover { background: #f2f5f9; }
.m-row.selected { background: #e9f2fb; }

/* 头像 + 右下角等级爱心 */
.m-avatar {
  position: relative;
  width: 42px;
  height: 42px;
  border-radius: 50%;
  flex-shrink: 0;
  background: #eef1f5;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  overflow: visible;
}
.m-avatar img {
  width: 42px;
  height: 42px;
  border-radius: 50%;
  object-fit: cover;
  display: block;
}
.m-avatar-letter { font-size: 18px; font-weight: 700; color: #7a8699; }
/* 在线小绿点：左上角 */
.m-online {
  position: absolute;
  left: -2px;
  top: -2px;
  width: 11px;
  height: 11px;
  border-radius: 50%;
  background: #c3cbd6;
  border: 2px solid #fff;
}
.m-online.on { background: #35c46a; }

.m-row-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.m-row-top { display: flex; align-items: center; gap: 6px; }
.m-name {
  flex: 1;
  min-width: 0;
  font-size: 13.5px;
  font-weight: 700;
  color: #2b3445;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  /* 行尾等级爱心是绝对定位的，这里给它留出位置，避免长昵称压到爱心上：
     爱心 34px + 与文字的间距 6px */
  padding-right: 40px;
}
/* 行尾容器：等级爱心始终贴在最右，未读气泡排在它左边。
   爱心固定在行末并且相对整行垂直居中：
   .m-row-top 只是昵称那一小行，爱心若跟着它居中会明显偏上；
   这里用绝对定位把它挂到 .m-row（position: relative）上，
   再 top:50% + translateY(-50%)，无论一行还是两行列表都在 y 轴正中。
   再用 margin-top 做 1px 视觉微调（爱心的墨迹中心比外框中心略低）。 */
.m-right {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  position: absolute;
  right: 10px;
  top: 50%;
  transform: translateY(-50%);
  margin-top: 1px;
}
.m-unread {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 999px;
  background: #ff5a8a;
  color: #fff;
  font-size: 11px;
  line-height: 18px;
  text-align: center;
  font-family: var(--font-mono);
}
.m-row-bottom { display: flex; align-items: center; }
.m-msg {
  font-size: 12px;
  color: #8c96a6;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 1.5;
}
.m-msg em { font-style: normal; color: #b3bbc7; }
/* 「正在输入…」用桃信粉强调 */
.m-msg.typing { color: #ff5a8a; }
.m-avatar { overflow: visible; }
.m-avatar img { border-radius: 50%; }
</style>
