<script setup>
// ============================================================
// ProfileCard.vue —— 个人名片（常驻在首页毛玻璃区域左侧栏）
// 内容：头像 + 昵称（蓝黑配色）、星座 / 生日 / 年龄 / 性别、当月日历。
// 说明：「本站已连续运行」已按需求搬到下方的统计卡片（StatsCard.vue）。
// ============================================================

import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
// 站点头像统一用 src/others/photo.jpg
import { splitName } from '../utils/splitName'

// ---------- 资料：全部来自后台「设置 → 站长资料」，这里只做兜底 ----------
const props = defineProps({
  name: { type: String, default: '' },
  avatar: { type: String, default: '' },
  birthday: { type: String, default: '' }, // YYYY-MM-DD
  zodiac: { type: String, default: '' },
  gender: { type: String, default: '' },
  bio: { type: String, default: '' },
  github: { type: String, default: '' },
  contact: { type: String, default: '' },
  avatarLetter: { type: String, default: '凛' }
})

const BIRTHDAY_TEXT = computed(() => props.birthday || '未设置')
const ZODIAC = computed(() => props.zodiac || '—')
const GENDER = computed(() => props.gender || '—')
const NICK_PARTS = computed(() => splitName(props.name || '凛冬|RinEyce'))
const NICK_CN = computed(() => NICK_PARTS.value.pre)
const NICK_EN = computed(() => NICK_PARTS.value.main)
const BIO = computed(() => props.bio || '热爱底层、性能与优雅的代码')
const AVATAR = computed(() => props.avatar || '')

// 每秒走一次：年龄跨生日能自动更新，运行时长也会逐秒跳动
const nowTs = ref(Date.now())
let timer = null
onMounted(() => { timer = setInterval(() => { nowTs.value = Date.now() }, 1000) })
onBeforeUnmount(() => { if (timer) clearInterval(timer) })

const now = computed(() => new Date(nowTs.value))

// 年龄：按「今年生日是否已过」精确计算，而不是简单年份相减
const age = computed(() => {
  if (!props.birthday) return 0
  const b = new Date(props.birthday + 'T00:00:00')
  if (Number.isNaN(b.getTime())) return 0
  const t = now.value
  let a = t.getFullYear() - b.getFullYear()
  const m = t.getMonth() + 1
  if (m < b.getMonth() + 1 || (m === b.getMonth() + 1 && t.getDate() < b.getDate())) a--
  return a < 0 ? 0 : a
})

// ---------- 日历已拆成独立卡片：见 CalendarCard.vue ----------
</script>

<template>
  <div class="pc">
    <!-- 头像（静态圆形）+ 昵称 + 一句简介 + 社交账号（原来在首屏 Hero 里，已挪进名片） -->
    <div class="pc-top">
      <img v-if="AVATAR" class="pc-avatar" :src="AVATAR" :alt="(NICK_CN + NICK_EN) + ' 的头像'" />
      <span v-else class="pc-avatar pc-avatar-letter">{{ avatarLetter }}</span>
      <div class="pc-name">
        <span class="pc-name-cn">{{ NICK_CN }}</span><span class="pc-name-en">{{ NICK_EN }}</span>
      </div>
      <p class="pc-bio">{{ BIO }}</p>

      <div class="pc-social">
        <a href="https://github.com/Dinowritecode" target="_blank" rel="noopener" aria-label="GitHub" title="GitHub">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="currentColor"><path d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.91.58.11.79-.25.79-.56 0-.28-.01-1.02-.02-2-3.2.7-3.87-1.54-3.87-1.54-.52-1.33-1.28-1.68-1.28-1.68-1.04-.71.08-.7.08-.7 1.15.08 1.76 1.18 1.76 1.18 1.03 1.76 2.7 1.25 3.36.96.1-.75.4-1.25.72-1.54-2.55-.29-5.23-1.28-5.23-5.68 0-1.26.45-2.28 1.18-3.09-.12-.29-.51-1.46.11-3.05 0 0 .96-.31 3.15 1.18a10.9 10.9 0 0 1 5.74 0c2.19-1.49 3.15-1.18 3.15-1.18.62 1.59.23 2.76.11 3.05.74.81 1.18 1.83 1.18 3.09 0 4.41-2.69 5.38-5.25 5.66.41.36.78 1.06.78 2.14 0 1.54-.01 2.79-.01 3.17 0 .31.21.68.8.56A10.52 10.52 0 0 0 23.5 12C23.5 5.65 18.35.5 12 .5z"/></svg>
        </a>
        <a href="mailto:zoo.to@qq.com" aria-label="邮箱" title="邮箱">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="m22 7-10 6L2 7"/></svg>
        </a>
      </div>
    </div>

    <ul class="pc-facts">
      <li><span class="k">出生日期</span><span class="v">{{ BIRTHDAY_TEXT }}</span></li>
        <li><span class="k">星座</span><span class="v">{{ ZODIAC }}</span></li>
      
      <li><span class="k">年龄</span><span class="v">{{ age }} 岁</span></li>
      <li><span class="k">性别</span><span class="v">{{ GENDER }}</span></li>
    </ul>
  </div>
</template>

<style scoped>
.pc {
  /* 与文章卡片同材质：半透明白底 + 不模糊 */
  background: var(--card-bg);
  backdrop-filter: var(--card-blur);
  -webkit-backdrop-filter: var(--card-blur);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 20px 18px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* 头像 + 昵称 + 简介 + 社交账号：整块居中排布 */
.pc-top {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: 8px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--border);
}
/* 静态圆形头像：不加 hover 动效、不做任何位移 */
.pc-avatar {
  width: 76px;
  height: 76px;
  border-radius: 50%;
  object-fit: cover;
  border: 2px solid var(--border);
  flex-shrink: 0;
}
.pc-name { font-size: 17px; font-weight: 700; letter-spacing: 0.3px; white-space: nowrap; }
.pc-name-cn { color: var(--accent); } /* 蓝：跟导航栏的「凛冬」用同一个品牌色 */
.pc-name-en { color: var(--text); }   /* 黑（深色主题下为主题文字色） */
.pc-bio { font-size: 12.5px; color: var(--text-dim); line-height: 1.6; }

/* 社交账号：圆形图标按钮 */
.pc-social { display: flex; gap: 10px; margin-top: 4px; }
.pc-social a {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  color: var(--text-dim);
  background: color-mix(in srgb, var(--text) 7%, transparent);
  transition: all 0.2s ease;
}
.pc-social a:hover {
  color: var(--on-accent);
  background: var(--accent-fill);
  text-decoration: none;
  transform: translateY(-2px);
}

/* 资料行 */
.pc-facts { list-style: none; display: flex; flex-direction: column; gap: 8px; }
.pc-facts li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 14px;
  padding-bottom: 7px;
  border-bottom: 1px dashed var(--border);
}
.pc-facts li:last-child { border-bottom: none; padding-bottom: 0; }
.pc-facts .k { color: var(--text-dim); }
.pc-facts .v { color: var(--text); font-weight: 600; font-family: var(--font-mono); }
</style>
