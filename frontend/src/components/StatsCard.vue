<script setup>
// ============================================================
// StatsCard.vue —— 全站统计卡片（放在公告卡片下方）
// 数字来自 GET /api/stats（已发布文章数 / 分类数 / 标签数）；
// 「本站已连续运行 …」原本在名片里，现按需求搬到这里。
// ============================================================

import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '../api'
import { useSite } from '../composables/useSite'

const { startedAt } = useSite()

// ---------- 全站统计数字 ----------
const stats = ref({ posts: 0, categories: 0, tags: 0 })

onMounted(async () => {
  try {
    const d = await api.getStats()
    stats.value = {
      posts: Number(d.posts) || 0,
      categories: Number(d.categories) || 0,
      tags: Number(d.tags) || 0
    }
  } catch {
    /* 读取失败时保持 0，不打断页面 */
  }
})

// ---------- 本站已连续运行 ----------
// startedAt 来自后端 /api/settings（本次进程启动时间），服务重启即归零。
const nowTs = ref(Date.now())
let timer = null
onMounted(() => { timer = setInterval(() => { nowTs.value = Date.now() }, 1000) })
onBeforeUnmount(() => { if (timer) clearInterval(timer) })

const uptime = computed(() => {
  if (!startedAt.value) return null // 后端尚未返回（例如旧版本后端还没重启）
  let s = Math.max(0, Math.floor((nowTs.value - startedAt.value) / 1000))
  const d = Math.floor(s / 86400); s -= d * 86400
  const h = Math.floor(s / 3600); s -= h * 3600
  const m = Math.floor(s / 60); s -= m * 60
  return { d, h, m, s }
})

// 240px 宽的侧栏放不下「0天0时10分2秒」这么多字：把为 0 的高位单位省掉，
// 刚重启时显示「10 分 02 秒」，运行久了自然变成「1 天 03 时」。
const uptimeText = computed(() => {
  const u = uptime.value
  if (!u) return ''
  const pad = (n) => String(n).padStart(2, '0')
  if (u.d > 0) return `${u.d} 天 ${pad(u.h)} 时`
  if (u.h > 0) return `${u.h} 时 ${pad(u.m)} 分`
  if (u.m > 0) return `${u.m} 分 ${pad(u.s)} 秒`
  return `${u.s} 秒`
})
</script>

<template>
  <section class="stats">
    <h3 class="stats-title"><span class="title-bar"></span> 全站统计</h3>

    <ul class="stats-list">
      <li><span class="k">文章</span><b>{{ stats.posts }}</b></li>
      <li><span class="k">分类</span><b>{{ stats.categories }}</b></li>
      <li><span class="k">标签</span><b>{{ stats.tags }}</b></li>
    </ul>

    <div class="stats-uptime">
      <template v-if="uptime">
        已连续运行 <b>{{ uptimeText }}</b>
      </template>
      <template v-else>运行时长获取中…</template>
    </div>
  </section>
</template>

<style scoped>
/* 与文章卡片同材质：半透明白底 + 不模糊 */
.stats {
  background: var(--card-bg);
  backdrop-filter: var(--card-blur);
  -webkit-backdrop-filter: var(--card-blur);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 18px;
}

.stats-title {
  font-family: var(--font-mono);
  font-size: 15px;
  font-weight: 700;
  margin-bottom: 10px;
  letter-spacing: 0.3px;
  /* 与前面的主题色竖线垂直居中对齐 */
  display: flex;
  align-items: center;
}

.stats-list { list-style: none; display: flex; flex-direction: column; gap: 7px; }
.stats-list li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13.5px;
}
.stats-list .k { color: var(--text-dim); }
.stats-list b {
  font-family: var(--font-mono);
  font-size: 15px;
  color: var(--accent-ink);
}

.stats-uptime {
  margin-top: 12px;
  padding-top: 11px;
  border-top: 1px dashed var(--border);
  text-align: center;
  font-size: 12.5px;
  line-height: 1.8;
  color: var(--text-dim);
}
/* 时长数字用等宽 + tabular-nums，避免每秒跳动时宽度抖 */
.stats-uptime b {
  color: var(--accent-ink);
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
  padding: 0 1px;
}
</style>
