<script setup>
// ============================================================
// UserProfileView.vue —— 个人主页（/u/:id）
//
// 结构：
//   ① 封面（约四分之一屏高，站长可上传，没传就用站点首页背景图）
//      左侧贴着封面下沿放头像 + 昵称 + 等级 + 9 位 ID；
//   ② 统计 + GitHub 式活动热力图（最近 53 周）；
//   ③ 他最近的说说（只读）。
//
// 入口：导航栏头像、评论区 / 说说里的头像与昵称。
// ============================================================

import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import { useAuth } from '../composables/useAuth'
import { useSite } from '../composables/useSite'
import { useToast } from '../composables/useToast'
import photoUrl from '../others/photo.jpg'

const route = useRoute()
const router = useRouter()
const { user: me, refresh: refreshMe } = useAuth()
const { homeBg } = useSite()
const { toast } = useToast()

const loading = ref(true)
const error = ref('')
const profile = ref(null)
const stats = ref({ moments: 0, likes: 0, comments: 0 })
const activity = ref([])
const moments = ref([])
const uploading = ref(false)
const fileInput = ref(null)

const isMe = computed(() => me.value && profile.value && me.value.id === profile.value.id)

// 封面：用户传过就用他自己的，否则退回站点首页背景图，再没有就用渐变
const coverUrl = computed(() => profile.value?.cover || homeBg.value || '')
const coverStyle = computed(() =>
  coverUrl.value
    ? { backgroundImage: `url("${coverUrl.value}"), linear-gradient(120deg, #1c2331, #2b3550)` }
    : { backgroundImage: 'linear-gradient(120deg, #1c2331, #2b3550)' }
)

function fmtDate(s) {
  if (!s) return ''
  const d = new Date(s)
  return `${d.getFullYear()} 年 ${d.getMonth() + 1} 月加入`
}

function timeAgo(s) {
  if (!s) return ''
  const t = new Date(s)
  const diff = Date.now() - t
  if (diff < 60 * 1000) return '刚刚'
  if (diff < 3600 * 1000) return `${Math.floor(diff / 60000)} 分钟前`
  if (diff < 86400 * 1000) return `${Math.floor(diff / 3600000)} 小时前`
  return `${t.getMonth() + 1} 月 ${t.getDate()} 日`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const d = await api.getUserProfile(route.params.id)
    profile.value = d.user
    stats.value = d.stats || stats.value
    activity.value = d.activity || []
    moments.value = d.moments || []
  } catch (e) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
    // 封面是异步渲染出来的：等它进 DOM 后通知导航栏重新量一次，
    // 否则导航栏还以为这是「没有大图的页面」，文字会压在封面上看不清。
    nextTick(() => window.dispatchEvent(new Event('resize')))
  }
}

onMounted(load)
watch(() => route.params.id, (id) => {
  if (id && route.name === 'user') load()
})

// ---------- 热力图 ----------
// 把 371 天排成「周为列、周日到周六为行」，和 GitHub 一样。
// 第一列可能不满（要看起始那天是周几），用 null 占位。
const heatWeeks = computed(() => {
  const days = activity.value
  if (!days.length) return []
  const weeks = []
  let week = []
  // 起始日期的星期几（0=周日）：前面补空格子，让同一行永远是同一个星期几
  const firstDow = new Date(days[0].date + 'T00:00:00').getDay()
  for (let i = 0; i < firstDow; i++) week.push(null)
  for (const d of days) {
    week.push(d)
    if (week.length === 7) {
      weeks.push(week)
      week = []
    }
  }
  if (week.length) {
    while (week.length < 7) week.push(null)
    weeks.push(week)
  }
  return weeks
})

// 月份标签：某一列里出现了「1 号」就标这个月
const heatMonths = computed(() =>
  heatWeeks.value.map((week, i) => {
    const hit = week.find((d) => d && d.date.endsWith('-01'))
    if (!hit) return ''
    const m = Number(hit.date.slice(5, 7))
    // 只在 1 月和每季度首月标一次，免得挤
    return m === 1 || i === 0 || m % 3 === 0 ? `${m}月` : ''
  })
)

function heatLevel(n) {
  if (!n) return 0
  if (n < 2) return 1
  if (n < 4) return 2
  if (n < 8) return 3
  return 4
}

const totalActivity = computed(() => activity.value.reduce((s, d) => s + d.count, 0))

// ---------- 封面 ----------
function pickCover() {
  fileInput.value && fileInput.value.click()
}

async function onCoverFile(e) {
  const file = (e.target.files || [])[0]
  e.target.value = ''
  if (!file) return
  if (!file.type.startsWith('image/')) return toast.error('只能选图片')
  uploading.value = true
  try {
    const up = await api.uploadMomentImage(file)
    await api.updateProfile({ cover: up.url })
    profile.value.cover = up.url
    refreshMe && refreshMe()
    toast.success('封面已更新')
  } catch (err) {
    toast.error(err.message || '上传失败')
  } finally {
    uploading.value = false
  }
}

// ---------- 其它 ----------
function goBack() {
  if (history.state && history.state.back) router.back()
  else router.push('/')
}

function openMoments() {
  router.push({ path: '/moments', hash: moments.value.length ? `#m-${moments.value[0].id}` : undefined })
}

function avatarOf(u) {
  return u?.avatar || (u?.id === 1 ? photoUrl : '')
}
</script>

<template>
  <div class="profile-page">
    <!-- 返回放在封面左上角：封面是通栏的，标题区不占额外高度 -->
    <button class="profile-back" @click="goBack">← 返回</button>

    <div v-if="loading" class="profile-state">加载中…</div>
    <div v-else-if="error" class="profile-state is-error">{{ error }}</div>

    <template v-else-if="profile">
      <!-- ① 封面：通栏，钻到导航栏下面，高度约三分之一屏 -->
      <header class="cover profile-cover" :style="coverStyle">
        <div class="cover-actions" v-if="isMe">
          <input ref="fileInput" type="file" accept="image/*" hidden @change="onCoverFile" />
          <button class="cover-btn" :disabled="uploading" @click="pickCover">
            {{ uploading ? '上传中…' : '更换封面' }}
          </button>
        </div>

        <div class="cover-info">
          <span class="cover-avatar">
            <img v-if="avatarOf(profile)" :src="avatarOf(profile)" alt="" />
            <span v-else class="avatar-letter">{{ (profile.nickname || '?')[0] }}</span>
          </span>
          <div class="cover-text">
            <h1 class="cover-name">
              {{ profile.nickname }}
              <i v-if="profile.role === 'admin'" class="badge-admin">站长</i>
              <i v-if="isMe" class="cover-me">这是你</i>
            </h1>
            <p class="cover-meta">
              <span class="cover-level">Lv.{{ profile.level }}</span>
              <span v-if="profile.userId" class="cover-dot">·</span>
              <span v-if="profile.userId" class="cover-id">ID {{ profile.userId }}</span>
              <span class="cover-dot">·</span>
              <span>{{ fmtDate(profile.createdAt) }}</span>
            </p>
          </div>
        </div>
      </header>

      <div class="profile-body">
        <!-- ② 统计 + 热力图 -->
        <section class="panel" v-reveal>
          <div class="panel-stats">
            <div class="panel-stat"><b>{{ stats.moments }}</b><span>说说</span></div>
            <div class="panel-stat"><b>{{ stats.likes }}</b><span>获赞</span></div>
            <div class="panel-stat"><b>{{ stats.comments }}</b><span>评论</span></div>
            <div class="panel-stat"><b>{{ totalActivity }}</b><span>近一年活跃</span></div>
          </div>

          <div class="heat">
            <div class="heat-head">
              <span class="heat-title">活动热力图</span>
              <span class="heat-sub">最近 53 周 · 说说 / 评论</span>
            </div>
            <div class="heat-scroll">
              <div class="heat-months">
                <span v-for="(m, i) in heatMonths" :key="i" class="heat-month">{{ m }}</span>
              </div>
              <div class="heat-grid">
                <div v-for="(week, wi) in heatWeeks" :key="wi" class="heat-col">
                  <span
                    v-for="(d, di) in week"
                    :key="di"
                    class="heat-cell"
                    :class="d ? 'lv' + heatLevel(d.count) : 'empty'"
                    :title="d ? `${d.date}　${d.count} 次` : ''"
                  ></span>
                </div>
              </div>
            </div>
            <div class="heat-legend">
              <span>少</span>
              <i class="heat-cell lv0"></i>
              <i class="heat-cell lv1"></i>
              <i class="heat-cell lv2"></i>
              <i class="heat-cell lv3"></i>
              <i class="heat-cell lv4"></i>
              <span>多</span>
            </div>
          </div>
        </section>

        <!-- ③ 说说 -->
        <div class="profile-sec-head">
          <h2 class="profile-sec-title"><span class="title-bar"></span> {{ isMe ? '我的说说' : 'TA 的说说' }}</h2>
          <button v-if="moments.length" class="profile-more" @click="openMoments">去说说页 →</button>
        </div>

        <div v-if="!moments.length" class="profile-state">还没有发过说说。</div>

        <div v-else class="profile-moments">
          <article v-for="m in moments" :key="m.id" class="pm">
            <div class="pm-head">
              <b>{{ timeAgo(m.createdAt) }}</b>
              <span class="pm-stats">
                <span>{{ m.views }} 浏览</span>
                <span>{{ m.likes }} 赞</span>
                <span>{{ m.comments }} 评论</span>
              </span>
            </div>
            <p v-if="m.content" class="pm-text">{{ m.content }}</p>
            <div v-if="m.images && m.images.length" class="pm-media">
              <img v-for="(src, i) in m.images.slice(0, 3)" :key="i" :src="src" alt="" loading="lazy" />
              <span v-if="m.images.length > 3" class="pm-media-more">+{{ m.images.length - 3 }}</span>
            </div>
          </article>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
/* 通栏：封面钻到导航栏下面，下面的内容与导航栏同宽（1328 容器） */
.profile-page {
  margin-top: calc(var(--nav-h) * -1);
  min-height: 70vh;
  padding-bottom: 60px;
}

/* 返回：浮在封面左上角，和导航栏内容区同一条左边缘 */
.profile-back {
  position: absolute;
  z-index: 3;
  top: calc(var(--nav-h) + 14px);
  left: max(24px, calc((100% - 1328px) / 2 + 24px));
  border: none;
  background: rgba(8, 12, 20, 0.35);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  color: #fff;
  font-size: 13.5px;
  padding: 7px 14px;
  border-radius: 999px;
  cursor: pointer;
  transition: background 0.18s ease;
}
.profile-back:hover { background: rgba(8, 12, 20, 0.55); }

.profile-state {
  text-align: center;
  color: var(--text-dim);
  font-size: 14px;
  padding: 40px 12px;
}
.profile-state.is-error { color: #e5484d; }

/* ---------- ① 封面（通栏） ---------- */
.cover {
  position: relative;
  width: 100%;
  /* 约三分之一屏高；通栏之后可以比之前高一些 */
  height: clamp(240px, 34vh, 420px);
  overflow: hidden;
  background-size: cover;
  background-position: center 32%;
  background-repeat: no-repeat;
  display: flex;
  align-items: flex-end;
  /* 顶部给导航栏让位：封面从屏幕最顶端铺下来 */
  padding-top: var(--nav-h);
}
/* 只在底部压一层很淡的渐变，保证白色文字在任何封面上都读得清。
   不做整图遮罩 —— 封面本身还是要能看清的。 */
.cover::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 58%;
  background: linear-gradient(to top, rgba(6, 10, 18, 0.62), rgba(6, 10, 18, 0));
  pointer-events: none;
}

.cover-actions {
  position: absolute;
  top: 12px;
  right: 12px;
  display: flex;
  gap: 8px;
}
.cover-btn {
  border: 1px solid rgba(255, 255, 255, 0.45);
  background: rgba(8, 12, 20, 0.35);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  color: #fff;
  font-size: 12.5px;
  padding: 7px 14px;
  border-radius: 999px;
  cursor: pointer;
  transition: background 0.18s ease, border-color 0.18s ease;
}
.cover-btn:hover:not(:disabled) { background: rgba(8, 12, 20, 0.55); border-color: #fff; }
.cover-btn:disabled { opacity: 0.6; cursor: default; }

/* 信息贴在封面左下：与导航栏内容区同一条左边缘（1328 容器 + 24 内边距） */
.cover-info {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 1328px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 0 24px 28px;
  min-width: 0;
}
.cover-avatar {
  width: 76px;
  height: 76px;
  flex-shrink: 0;
  border-radius: 50%;
  overflow: hidden;
  border: 3px solid rgba(255, 255, 255, 0.9);
  background: var(--bg-alt);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
  display: flex;
  align-items: center;
  justify-content: center;
}
.cover-avatar img { width: 100%; height: 100%; object-fit: cover; }

.cover-text { min-width: 0; }
.cover-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 1.55rem;
  font-weight: 800;
  letter-spacing: 0.5px;
  color: #fff;
  margin-bottom: 6px;
  text-shadow: 0 2px 4px rgba(0, 0, 0, 0.9), 0 4px 16px rgba(0, 0, 0, 0.7);
}
.cover-me {
  font-style: normal;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 1px;
  color: #fff;
  border: 1px solid rgba(255, 255, 255, 0.75);
  border-radius: 999px;
  padding: 1px 8px;
}
.cover-meta {
  display: flex;
  align-items: center;
  gap: 7px;
  flex-wrap: wrap;
  font-size: 12.5px;
  color: rgba(255, 255, 255, 0.92);
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.95), 0 2px 10px rgba(0, 0, 0, 0.8);
}
.cover-level { font-family: var(--font-mono); font-weight: 600; color: var(--accent); }
.cover-id { font-family: var(--font-mono); }
.cover-dot { opacity: 0.6; }

/* ---------- ② 统计 + 热力图 ---------- */
/* 内容区与导航栏同宽，页面因此是「通栏」而不是居中的窄栏 */
.profile-body {
  max-width: 1328px;
  margin: 0 auto;
  padding: 26px 24px 0;
}

.panel {
  margin-top: 0;
  background: var(--card-bg);
  backdrop-filter: var(--card-blur);
  -webkit-backdrop-filter: var(--card-blur);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 6px 18px 16px;
}

.panel-stats { display: flex; gap: 6px; }
.panel-stat {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  padding: 12px 0;
  border-radius: 10px;
  transition: background 0.18s ease;
}
.panel-stat:hover { background: var(--accent-soft); }
.panel-stat b { font-size: 1.15rem; font-family: var(--font-mono); color: var(--text); }
.panel-stat span { font-size: 12px; color: var(--text-dim); }

.heat { margin-top: 6px; border-top: 1px solid var(--border); padding-top: 14px; }
.heat-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 10px;
}
.heat-title { font-size: 13.5px; font-weight: 700; color: var(--text); }
.heat-sub { font-size: 11.5px; color: var(--text-dim); }

.heat-scroll { overflow-x: auto; padding-bottom: 4px; }
.heat-months {
  display: flex;
  gap: 3px;
  margin-bottom: 4px;
  min-width: max-content;
}
.heat-month {
  width: 11px;
  flex-shrink: 0;
  font-size: 10px;
  color: var(--text-dim);
  font-family: var(--font-mono);
  white-space: nowrap;
  overflow: visible;
}
.heat-grid { display: flex; gap: 3px; min-width: max-content; }
.heat-col { display: flex; flex-direction: column; gap: 3px; }
.heat-cell {
  width: 11px;
  height: 11px;
  border-radius: 2px;
  display: inline-block;
  background: color-mix(in srgb, var(--text) 7%, transparent);
}
.heat-cell.empty { background: transparent; }
.heat-cell.lv0 { background: color-mix(in srgb, var(--text) 7%, transparent); }
.heat-cell.lv1 { background: color-mix(in srgb, var(--accent) 28%, transparent); }
.heat-cell.lv2 { background: color-mix(in srgb, var(--accent) 52%, transparent); }
.heat-cell.lv3 { background: color-mix(in srgb, var(--accent) 76%, transparent); }
.heat-cell.lv4 { background: var(--accent); }

.heat-legend {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
  margin-top: 8px;
  font-size: 11px;
  color: var(--text-dim);
}

/* ---------- ③ 说说 ---------- */
.profile-sec-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 24px 0 14px;
}
.profile-sec-title {
  display: flex;
  align-items: center;
  font-size: 1.15rem;
  color: var(--text);
}
.profile-more {
  border: none;
  background: transparent;
  color: var(--accent-ink);
  font-size: 13px;
  cursor: pointer;
  padding: 6px 8px;
  border-radius: 8px;
}
.profile-more:hover { background: var(--accent-soft); }

.profile-moments {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: 12px;
  align-items: start;
}
.pm {
  background: var(--card-bg);
  backdrop-filter: var(--card-blur);
  -webkit-backdrop-filter: var(--card-blur);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 14px 16px;
}
.pm-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: var(--text-dim);
  margin-bottom: 8px;
}
.pm-head b { font-weight: 600; }
.pm-stats { display: flex; gap: 12px; font-family: var(--font-mono); }
.pm-text {
  font-size: 14.5px;
  line-height: 1.7;
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-word;
}
.pm-media { display: flex; gap: 6px; margin-top: 10px; position: relative; }
.pm-media img {
  width: 88px;
  height: 88px;
  object-fit: cover;
  border-radius: 8px;
  background: var(--bg-alt);
}
.pm-media-more {
  align-self: flex-end;
  font-size: 12px;
  color: var(--text-dim);
  font-family: var(--font-mono);
}

@media (max-width: 760px) {
  .cover { height: clamp(200px, 28vh, 300px); padding-top: var(--nav-h); }
  .cover-info { padding: 0 16px 18px; gap: 11px; }
  .cover-avatar { width: 60px; height: 60px; border-width: 2px; }
  .cover-name { font-size: 1.2rem; }
  .profile-body { padding: 18px 16px 0; }
  .profile-back { left: 16px; }
  .panel { padding: 4px 12px 14px; }
  .panel-stat b { font-size: 1rem; }
  .profile-moments { grid-template-columns: 1fr; }
  .pm-media img { width: 72px; height: 72px; }
}
</style>
