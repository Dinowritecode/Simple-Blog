<script setup>
// ============================================================
// AdminOverviewView.vue —— 后台「总览」
//
// 左侧（主区）：
//   ① 四个数字卡片：用户 / 文章 / 说说 / 评论
//   ② 四个环：负载 / CPU / 内存 / 磁盘（主机物理性能）
//   ③ 折线图：近 14 天访问量（PV 线 + 面积，UV 虚线）
// 右侧（窄栏）：
//   ④ 网站简略信息：名称 / 镜像版本 / 系统 / IP / 启动时间 / 运行时长
//   ⑤ 备忘录：自动保存到 settings
//
// 环和折线都是手写 SVG，没有引入图表库 —— 数据量小，省一个依赖。
// ============================================================
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '../api'

const loading = ref(true)
const error = ref('')
const counts = ref({ users: 0, posts: 0, moments: 0, comments: 0 })
const host = ref({ supported: false, loadOK: false, cpuOK: false, memOK: false, diskOK: false, load1: 0, cpuCount: 0, cpuPercent: 0, memUsed: 0, memTotal: 0, diskUsed: 0, diskTotal: 0 })
const traffic = ref([])

// 右侧：网站信息 + 备忘录
const info = ref({ name: '', version: '', os: '', ip: '', startedAt: '', uptime: '', memo: '' })
const memo = ref('')
const memoState = ref('idle') // idle | saving | saved | error
const memoAt = ref('')
let memoTimer = null
let memoLoaded = false

onMounted(async () => {
  try {
    const d = await api.getOverview()
    counts.value = d.counts || counts.value
    host.value = d.host || host.value
    traffic.value = d.traffic || []
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
  try {
    const s = await api.getSiteInfo()
    info.value = { ...info.value, ...s }
    memo.value = s.memo || ''
    memoLoaded = true
  } catch {
    /* 右侧信息拿不到不影响主区 */
  }
})

// 运行时长每分钟自增，不用反复请求接口
const uptimeTick = ref(0)
const uptimeText = computed(() => {
  const base = Number(info.value.uptimeSec || 0)
  if (!base) return info.value.uptime || '—'
  let total = base + uptimeTick.value * 60
  const d = Math.floor(total / 86400)
  const h = Math.floor((total % 86400) / 3600)
  const m = Math.floor((total % 3600) / 60)
  return [d ? d + ' 天' : '', d || h ? h + ' 小时' : '', m + ' 分'].filter(Boolean).join(' ')
})
const timer = setInterval(() => { uptimeTick.value += 1 }, 60000)

const infoRows = computed(() => [
  { label: '网站名称', value: info.value.name || '—' },
  { label: '当前版本', value: info.value.version || '—' },
  { label: '操作系统', value: info.value.os || '—' },
  { label: 'IP 地址', value: info.value.ip || '—' },
  { label: '启动时间', value: info.value.startedAt || '—' },
  { label: '运行时长', value: uptimeText.value }
])

async function saveMemo() {
  if (!memoLoaded) return
  memoState.value = 'saving'
  try {
    await api.saveMemo(memo.value)
    memoState.value = 'saved'
    memoAt.value = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  } catch {
    memoState.value = 'error'
  }
}

// 停止输入 1.2 秒后自动保存（也有手动保存按钮）
function onMemoInput() {
  memoState.value = 'idle'
  clearTimeout(memoTimer)
  memoTimer = setTimeout(saveMemo, 1200)
}

onBeforeUnmount(() => {
  clearInterval(timer)
  clearTimeout(memoTimer)
})

// ---------- ① 数字卡片 ----------
const cards = computed(() => [
  { key: 'users', label: '用户数', value: counts.value.users },
  { key: 'posts', label: '文章数', value: counts.value.posts },
  { key: 'moments', label: '说说数', value: counts.value.moments },
  { key: 'comments', label: '评论数', value: counts.value.comments }
])

// ---------- ② 环 ----------
const GB = (v) => (v >= 100 ? v.toFixed(0) : v.toFixed(v >= 10 ? 1 : 2))

const gauges = computed(() => {
  const h = host.value
  const pct = (used, total) => (total > 0 ? Math.min(100, (used / total) * 100) : 0)
  // 每一项单独判断可用性：Windows 没有 loadavg，但 CPU/内存/磁盘都是真实值
  const NA = '当前平台不支持'
  return [
    {
      key: 'load',
      label: '负载',
      ok: !!h.loadOK,
      // 负载没有「总量」，用核数当基准：load1 / 核数
      value: h.loadOK && h.cpuCount > 0 ? pct(h.load1, h.cpuCount) : 0,
      display: h.loadOK ? h.load1.toFixed(2) : '—',
      sub: h.loadOK ? `（${h.load1.toFixed(2)} / ${h.cpuCount} 核）` : 'Windows 无负载均值'
    },
    {
      key: 'cpu',
      label: 'CPU',
      ok: !!h.cpuOK,
      value: h.cpuOK ? h.cpuPercent : 0,
      display: h.cpuOK ? h.cpuPercent.toFixed(2) : '—',
      sub: h.cpuOK ? `（${h.cpuCount} 核）` : NA
    },
    {
      key: 'mem',
      label: '内存',
      ok: !!h.memOK,
      value: h.memOK ? pct(h.memUsed, h.memTotal) : 0,
      display: h.memOK ? pct(h.memUsed, h.memTotal).toFixed(2) : '—',
      sub: h.memOK ? `${GB(h.memUsed)} GB / ${GB(h.memTotal)} GB` : NA
    },
    {
      key: 'disk',
      label: '磁盘',
      ok: !!h.diskOK,
      value: h.diskOK ? pct(h.diskUsed, h.diskTotal) : 0,
      display: h.diskOK ? pct(h.diskUsed, h.diskTotal).toFixed(2) : '—',
      sub: h.diskOK ? `${GB(h.diskUsed)} GB / ${GB(h.diskTotal)} GB` : NA
    }
  ]
})

// 环的几何：半径 62，周长用于 dasharray
const R = 62
const CIRC = 2 * Math.PI * R
const ringDash = (pct) => `${(Math.max(0, Math.min(100, pct)) / 100) * CIRC} ${CIRC}`

// ---------- ③ 折线图 ----------
const W = 860
const H = 220
const PAD = { top: 16, right: 16, bottom: 30, left: 38 }

const chart = computed(() => {
  const data = traffic.value
  if (!data.length) return null
  const maxV = Math.max(1, ...data.map((d) => Math.max(d.views, d.visitors)))
  // 纵轴取整到「好看的刻度」：1/2/5 × 10^n
  const step = niceStep(maxV / 4)
  const top = Math.max(step * 4, niceStep(maxV))
  const innerW = W - PAD.left - PAD.right
  const innerH = H - PAD.top - PAD.bottom
  const x = (i) => PAD.left + (data.length === 1 ? innerW / 2 : (i / (data.length - 1)) * innerW)
  const y = (v) => PAD.top + innerH - (v / top) * innerH

  const pvPts = data.map((d, i) => [x(i), y(d.views)])
  const uvPts = data.map((d, i) => [x(i), y(d.visitors)])
  const line = (pts) => pts.map((p, i) => `${i ? 'L' : 'M'}${p[0].toFixed(1)} ${p[1].toFixed(1)}`).join(' ')
  // 面积：折线 + 回到基线闭合
  const area = `${line(pvPts)} L${x(data.length - 1).toFixed(1)} ${(PAD.top + innerH).toFixed(1)} L${x(0).toFixed(1)} ${(PAD.top + innerH).toFixed(1)} Z`

  // 纵轴刻度（0 / 1/4 / 1/2 / 3/4 / 满）
  const ticks = [0, 1, 2, 3, 4].map((k) => {
    const v = (top / 4) * k
    return { v: Math.round(v), y: y(v) }
  })

  return {
    area,
    pvLine: line(pvPts),
    uvLine: line(uvPts),
    pvPts,
    ticks,
    labels: data.map((d, i) => ({
      x: x(i),
      text: d.day.slice(5), // MM-DD
      // 14 天标签太密：只显示首尾和隔一个
      show: i === 0 || i === data.length - 1 || i % 2 === 0
    })),
    total: data.reduce((n, d) => n + d.views, 0),
    totalUv: data.reduce((n, d) => n + d.visitors, 0),
    max: top
  }
})

function niceStep(raw) {
  if (!(raw > 0)) return 1
  const p = Math.pow(10, Math.floor(Math.log10(raw)))
  const n = raw / p
  const m = n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10
  return m * p
}
</script>

<template>
  <div class="ov">
    <header class="ov-head">
      <h1>总览</h1>
      <p>站点数据与主机状态</p>
    </header>

    <p v-if="error" class="ov-error">{{ error }}</p>

    <div class="ov-cols">
      <!-- ==================== 左侧主区 ==================== -->
      <div class="ov-main">
    <!-- ① 数字卡片 -->
    <section class="cards">
      <div v-for="c in cards" :key="c.key" class="card stat">
        <span class="stat-label">{{ c.label }}</span>
        <strong class="stat-num">{{ loading ? '—' : c.value }}</strong>
      </div>
    </section>

    <!-- ② 主机性能环 -->
    <section class="panel">
      <h2 class="panel-title">状态</h2>
      <div class="gauges">
        <div v-for="g in gauges" :key="g.key" class="gauge">
          <svg class="ring" viewBox="0 0 160 160" aria-hidden="true">
            <circle class="ring-track" cx="80" cy="80" :r="R" />
            <!-- 旋转 -90° 让弧从 12 点开始，逆时针方向用 scale(-1,1) 之类不必要 -->
            <circle
              v-if="g.ok && g.value > 0"
              class="ring-bar"
              cx="80" cy="80" :r="R"
              :stroke-dasharray="ringDash(g.value)"
              transform="rotate(-90 80 80)"
            />
          </svg>
          <div class="gauge-center">
            <b>{{ g.display }}</b><i v-if="g.display !== '—'">%</i>
          </div>
          <div class="gauge-label">{{ g.label }}</div>
          <div class="gauge-sub">{{ g.sub }}</div>
        </div>
      </div>
    </section>

    <!-- ③ 访问量折线 -->
    <section class="panel">
      <div class="panel-head">
        <h2 class="panel-title">访问量</h2>
        <div class="legend">
          <span class="lg pv"></span>浏览量
          <span class="lg uv"></span>访客数
          <span class="lg-total">近 14 天：{{ chart ? chart.total : 0 }} / {{ chart ? chart.totalUv : 0 }}</span>
        </div>
      </div>

      <div v-if="!chart" class="empty">还没有访问数据</div>
      <svg v-else class="chart" :viewBox="`0 0 ${W} ${H}`" role="img" aria-label="近 14 天访问量折线图">
        <!-- 横向网格 + 纵轴刻度 -->
        <g class="grid">
          <template v-for="t in chart.ticks" :key="t.v">
            <line :x1="PAD.left" :x2="W - PAD.right" :y1="t.y" :y2="t.y" />
            <text :x="PAD.left - 10" :y="t.y + 4" text-anchor="end">{{ t.v }}</text>
          </template>
        </g>
        <!-- 面积 + 折线 -->
        <path class="area" :d="chart.area" />
        <path class="line-pv" :d="chart.pvLine" />
        <path class="line-uv" :d="chart.uvLine" />
        <!-- 数据点 -->
        <circle v-for="(p, i) in chart.pvPts" :key="i" class="dot" :cx="p[0]" :cy="p[1]" r="2.6" />
        <!-- 横轴日期 -->
        <text
          v-for="(l, i) in chart.labels"
          v-show="l.show"
          :key="'l' + i"
          class="xlabel"
          :x="l.x"
          :y="H - 8"
          text-anchor="middle"
        >{{ l.text }}</text>
      </svg>
    </section>
      </div><!-- /ov-main -->

      <!-- ==================== 右侧窄栏 ==================== -->
      <aside class="ov-side">
        <!-- ④ 网站简略信息 -->
        <section class="panel side-panel">
          <h2 class="panel-title">网站信息</h2>
          <dl class="info">
            <template v-for="r in infoRows" :key="r.label">
              <dt>{{ r.label }}</dt>
              <dd :title="r.value">{{ r.value }}</dd>
            </template>
          </dl>
        </section>

        <!-- ⑤ 备忘录 -->
        <section class="panel side-panel">
          <div class="panel-head">
            <h2 class="panel-title">备忘录</h2>
            <span class="memo-state">
              <template v-if="memoState === 'saving'">保存中…</template>
              <template v-else-if="memoState === 'saved'">已保存 {{ memoAt }}</template>
              <template v-else-if="memoState === 'error'">保存失败</template>
            </span>
          </div>
          <textarea
            v-model="memo"
            class="memo"
            rows="8"
            maxlength="5000"
            placeholder="随手记点什么：待办、想法、临时笔记…&#10;停止输入 1.2 秒后自动保存"
            @input="onMemoInput"
          ></textarea>
          <div class="memo-foot">
            <span class="memo-count">{{ memo.length }} / 5000</span>
            <button class="memo-save" type="button" @click="saveMemo">保存</button>
          </div>
        </section>
      </aside>
    </div><!-- /ov-cols -->
  </div>
</template>

<style scoped>
/* 不设 max-width：让内容撑满内容区，右栏才能贴到浏览器右边 */
.ov { max-width: none; }
/* 左主区 + 右窄栏（右侧 340px 固定，左边自适应） */
.ov-cols {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 18px;
  align-items: start;
  /* 抵消 .admin-main 的右内边距（32px）但故意留 14px：
     最右侧区块与浏览器边缘之间保持一点呼吸空间，不会显得被切掉 */
  margin-right: -18px;
}
.ov-main { min-width: 0; }
.ov-side { display: flex; flex-direction: column; gap: 18px; position: sticky; top: 24px; }
.side-panel { margin-bottom: 0; }

.ov-head { margin-bottom: 20px; }
.ov-head h1 { font-size: 1.45rem; margin-bottom: 4px; }
.ov-head p { color: var(--text-dim); font-size: 13.5px; }
.ov-error { color: #e5484d; font-size: 13px; margin-bottom: 12px; }

/* ④ 网站信息 */
.info { display: grid; grid-template-columns: 76px minmax(0, 1fr); gap: 10px 12px; margin-top: 12px; }
.info dt { color: var(--text-dim); font-size: 12.5px; }
.info dd {
  margin: 0;
  font-size: 12.5px;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

/* ⑤ 备忘录 */
.memo-state { font-size: 11.5px; color: var(--text-dim); }
.memo {
  width: 100%;
  margin-top: 12px;
  min-height: 148px;
  resize: vertical;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg);
  color: var(--text);
  font-size: 13px;
  line-height: 1.6;
  font-family: inherit;
  outline: none;
  transition: border-color 0.18s ease;
}
.memo:focus { border-color: var(--accent); }
.memo-foot { display: flex; align-items: center; justify-content: space-between; margin-top: 8px; }
.memo-count { font-size: 11.5px; color: var(--text-dim); font-variant-numeric: tabular-nums; }
.memo-save {
  padding: 5px 16px;
  border: 1.5px solid var(--accent);
  border-radius: 999px;
  background: transparent;
  color: var(--accent);
  font-size: 12.5px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}
.memo-save:hover { background: var(--accent); color: #fff; }

/* ① 卡片 */
.cards { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; margin-bottom: 18px; }
.card {
  background: var(--card);
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  box-shadow: var(--shadow);
  padding: 16px 18px;
}
.stat { display: flex; flex-direction: column; gap: 8px; }
.stat-label { color: var(--text-dim); font-size: 13px; }
.stat-num {
  font-size: 30px;
  line-height: 1;
  font-weight: 700;
  color: var(--accent);
  font-variant-numeric: tabular-nums;
}

/* ② 面板 */
.panel {
  background: var(--card);
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  box-shadow: var(--shadow);
  padding: 18px 20px 20px;
  margin-bottom: 18px;
}
.panel-title {
  font-size: 15px;
  padding-left: 10px;
  border-left: 3px solid var(--accent);
  line-height: 1.1;
  margin-bottom: 6px;
}
.panel-head { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; flex-wrap: wrap; }

.gauges {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
  margin-top: 12px;
}
.gauge { position: relative; display: flex; flex-direction: column; align-items: center; padding-top: 6px; }
.ring { width: 150px; height: 150px; }
.ring-track {
  fill: none;
  stroke: color-mix(in srgb, var(--accent) 12%, transparent);
  stroke-width: 9;
}
.ring-bar {
  fill: none;
  stroke: var(--accent);
  stroke-width: 9;
  stroke-linecap: round;
  transition: stroke-dasharray 0.6s ease;
}
.gauge-center {
  position: absolute;
  top: 6px;
  height: 150px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1px;
}
.gauge-center b { font-size: 21px; font-weight: 700; color: var(--text); font-variant-numeric: tabular-nums; }
.gauge-center i { font-style: normal; font-size: 12px; color: var(--text-dim); margin-top: 6px; }
.gauge-label { margin-top: 8px; font-size: 13.5px; color: var(--text); }
.gauge-sub { font-size: 12px; color: var(--text-dim); margin-top: 2px; text-align: center; }

/* ③ 图表 */
.legend { display: flex; align-items: center; gap: 8px; font-size: 12.5px; color: var(--text-dim); }
.lg { width: 14px; height: 3px; border-radius: 2px; display: inline-block; }
.lg.pv { background: var(--accent); }
.lg.uv { background: color-mix(in srgb, var(--accent) 45%, transparent); margin-left: 8px; }
.lg-total { margin-left: 10px; font-variant-numeric: tabular-nums; }

.chart { width: 100%; height: auto; margin-top: 10px; overflow: visible; }
.grid line { stroke: var(--border); stroke-width: 1; stroke-dasharray: 3 4; }
.grid text { fill: var(--text-dim); font-size: 11px; }
.area { fill: color-mix(in srgb, var(--accent) 12%, transparent); }
.line-pv { fill: none; stroke: var(--accent); stroke-width: 2.2; stroke-linejoin: round; stroke-linecap: round; }
.line-uv {
  fill: none;
  stroke: color-mix(in srgb, var(--accent) 45%, transparent);
  stroke-width: 1.6;
  stroke-dasharray: 4 4;
}
.dot { fill: #fff; stroke: var(--accent); stroke-width: 1.8; }
.xlabel { fill: var(--text-dim); font-size: 11px; }
.empty { color: var(--text-dim); font-size: 13px; padding: 26px 0; text-align: center; }

@media (max-width: 1000px) {
  /* 窄屏叠成单列时把负边距撤掉，靠 .admin-main 自己的内边距（16px）留白 */
  .ov-cols { grid-template-columns: minmax(0, 1fr); margin-right: -4px; }
  .ov-side { position: static; }
  .cards { grid-template-columns: repeat(2, 1fr); }
  .gauges { grid-template-columns: repeat(2, 1fr); }
}
</style>
