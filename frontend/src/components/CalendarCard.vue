<script setup>
// ============================================================
// CalendarCard.vue —— 当月日历（右侧栏「全站统计」下方的独立卡片）
// 头部是「‹ 2026年5月 ›」：左右按钮切换月份，中间标题点一下回到本月。
// 网格样式不变：周一起始，今天高亮（不是本月查看时不高亮）。
// ============================================================

import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

const WEEK = ['一', '二', '三', '四', '五', '六', '日'] // 周一作为一周第一天

// 真实「今天」：每秒走一次，跨零点会自动刷新
const nowTs = ref(Date.now())
let timer = null
onMounted(() => { timer = setInterval(() => { nowTs.value = Date.now() }, 1000) })
onBeforeUnmount(() => { if (timer) clearInterval(timer) })

const today = computed(() => new Date(nowTs.value))

// 正在查看的年月（默认就是本月），通过左右按钮切换
const viewYear = ref(new Date().getFullYear())
const viewMonth = ref(new Date().getMonth()) // 0~11

function prevMonth() {
  if (viewMonth.value === 0) {
    viewMonth.value = 11
    viewYear.value--
  } else {
    viewMonth.value--
  }
}
function nextMonth() {
  if (viewMonth.value === 11) {
    viewMonth.value = 0
    viewYear.value++
  } else {
    viewMonth.value++
  }
}
// 点中间标题：回到本月
function backToToday() {
  viewYear.value = today.value.getFullYear()
  viewMonth.value = today.value.getMonth()
}

// 只有查看本月时才高亮今天
const isCurrentMonth = computed(
  () => viewYear.value === today.value.getFullYear() && viewMonth.value === today.value.getMonth()
)

const calendar = computed(() => {
  const y = viewYear.value
  const m = viewMonth.value
  const days = new Date(y, m + 1, 0).getDate() // 下月 0 号 = 本月最后一天
  const lead = (new Date(y, m, 1).getDay() + 6) % 7 // getDay(): 0=周日 → 换算成「周一=0」
  const cells = []
  for (let i = 0; i < lead; i++) cells.push(null)
  for (let d = 1; d <= days; d++) cells.push(d)
  return { cells, todayDate: isCurrentMonth.value ? today.value.getDate() : 0 }
})
</script>

<template>
  <section class="cal-card">
    <!-- 月份切换头（原来的「# 日历」标题已去掉） -->
    <div class="cal-bar">
      <button class="cal-nav" type="button" aria-label="上个月" @click="prevMonth">‹</button>
      <button class="cal-now" type="button" title="回到本月" @click="backToToday">
        {{ viewYear }}年{{ viewMonth + 1 }}月
      </button>
      <button class="cal-nav" type="button" aria-label="下个月" @click="nextMonth">›</button>
    </div>

    <div class="cal-grid">
      <span v-for="w in WEEK" :key="w" class="cal-w">{{ w }}</span>
      <span
        v-for="(d, i) in calendar.cells"
        :key="i"
        class="cal-d"
        :class="{ 'is-today': d !== null && d === calendar.todayDate, 'is-empty': d === null }"
      >{{ d || '' }}</span>
    </div>
  </section>
</template>

<style scoped>
/* 与文章卡片同材质：半透明白底 + 不模糊 */
.cal-card {
  background: var(--card-bg);
  backdrop-filter: var(--card-blur);
  -webkit-backdrop-filter: var(--card-blur);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 16px 18px 18px;
}

/* 月份切换头 */
.cal-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.cal-nav {
  width: 26px;
  height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--text-dim);
  font-size: 17px;
  line-height: 1;
  cursor: pointer;
  transition: all 0.2s ease;
}
.cal-nav:hover { color: var(--accent); background: var(--accent-soft); }
.cal-now {
  flex: 1;
  border: none;
  background: transparent;
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
  padding: 4px 0;
}
.cal-now:hover { color: var(--accent); }

/* 网格样式保持不变 */
.cal-grid { display: grid; grid-template-columns: repeat(7, 1fr); gap: 2px; text-align: center; }
.cal-w { font-size: 11.5px; color: var(--text-dim); padding-bottom: 4px; }
.cal-d {
  height: 26px;
  line-height: 26px;
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--text);
  border-radius: 7px;
}
.cal-d.is-empty { visibility: hidden; }
/* 今天：圆形选中（原来是方形块），所以单独限宽 + 居中 */
.cal-d.is-today {
  width: 26px;
  margin: 0 auto;
  border-radius: 50%;
  background: var(--accent-fill);
  color: var(--on-accent);
  font-weight: 700;
}
</style>
