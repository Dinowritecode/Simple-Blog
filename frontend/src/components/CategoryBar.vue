<script setup>
// ============================================================
// CategoryBar.vue —— 文章分类器
// 左边一个「全部」按钮（房子图标），后面是各分类 + 文章数。
// · 分类少时就是参考图那样的一行；
// · 一行放不下时才出现「全部」按钮，点它展开成多行 / 收起。
// 选中哪个分类由父组件通过 active 传入，点击时 emit('select', 分类名)。
// ============================================================

import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from '../api'

const props = defineProps({
  active: { type: String, default: '' }, // 当前选中的分类，空串 = 全部
  // 是否套一层「长条」外壳：首页博客区用它把分类做成顶部的一条横栏；
  // 搜索结果里保持裸胶囊（那里不是列表顶栏，加壳反而多一层容器）
  boxed: { type: Boolean, default: false }
})
const emit = defineEmits(['select'])

const cats = ref([])
const total = ref(0)
const expanded = ref(false) // 是否展开成多行
const overflowing = ref(false) // 单行是否放不下
const rowRef = ref(null)

async function load() {
  try {
    // 「全部」上的数字是「已发布文章总数」。
    // 分类接口只返回「每个分类的文章数」，没填分类的文章不在里面，直接相加会偏小；
    // 所以同时取一次全站统计，拿不到再退回求和。
    const [d, st] = await Promise.all([api.listCategories(), api.getStats().catch(() => null)])
    cats.value = d.items || []
    const posts = st && Number(st.posts)
    total.value = Number.isFinite(posts) && posts > 0
      ? posts
      : cats.value.reduce((sum, c) => sum + (Number(c.count) || 0), 0)
  } catch {
    /* 拉不到分类就只留「全部」按钮，不影响文章列表 */
  }
  await nextTick()
  measure()
}

// 用「内容宽度 > 可视宽度」判断是否溢出，比写死数量更稳
function measure() {
  const el = rowRef.value
  if (!el) return
  overflowing.value = el.scrollWidth > el.clientWidth + 1
}

onMounted(() => {
  load()
  window.addEventListener('resize', measure)
})
onBeforeUnmount(() => window.removeEventListener('resize', measure))

// 选中项变化后布局可能变（active 样式不同），重新量一次
watch(() => props.active, () => nextTick(measure))
watch(cats, () => nextTick(measure))
</script>

<template>
  <div class="catbar" :class="{ boxed }">
    <div ref="rowRef" class="cat-row" :class="{ expanded }">
      <!-- 全部：用文字而不是房子图标 —— 房子会被读成「回到首页」，和这里的
           「不限分类、看全部文章」是两回事 -->
      <button class="cat-all" :class="{ on: !active }" title="不限分类，看全部文章" @click="emit('select', '')">
        全部<span class="cat-count">{{ total }}</span>
      </button>

      <!-- 各分类 -->
      <button
        v-for="c in cats"
        :key="c.name"
        class="cat-chip"
        :class="{ on: active === c.name }"
        @click="emit('select', c.name)"
      >
        {{ c.name }}<span class="cat-count">{{ c.count }}</span>
      </button>
    </div>

    <button v-if="overflowing || expanded" class="cat-more" @click="expanded = !expanded">
      {{ expanded ? '收起' : '全部' }}
    </button>
  </div>
</template>

<style scoped>
/* 筛选行不再套一层卡片底：
   博客列表里它是一条白带、搜索结果里同样是胶囊却直接落在页面上，
   同一组控件两套外壳。去掉容器，靠间距和胶囊本身分组，两个页面就一致了。 */
.catbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 22px;
}

/* 长条模式（首页博客区）：分类整体做成顶部的一条横栏。
   底色必须用 --card-bg（= 50% 卡片色），不能直接用 --card 实底 ——
   下面的文章卡片走的是 style.css 里 `.section-inner .blog-grid .post-card → var(--card-bg)`，
   用实底就会在半透明的卡片上方突兀地白一条。模糊也跟着同一个变量，
   后台调「外观定制」时跟着一起变。 */
.catbar.boxed {
  background: var(--card-bg);
  backdrop-filter: var(--card-blur);
  -webkit-backdrop-filter: var(--card-blur);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 9px 12px;
  margin-bottom: 20px;
}

/* 分类行：默认单行、超出裁掉；展开后换行全显示 */
.cat-row {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  overflow: hidden;
}
.cat-row.expanded {
  flex-wrap: wrap;
  overflow: visible;
}

/* 「全部」按钮：主题色方块 + 房子图标 */
.cat-all {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border: none;
  border-radius: 8px;
  padding: 7px 10px;
  font-size: 12.5px;
  font-weight: 600;
  cursor: pointer;
  color: var(--text-dim);
  background: color-mix(in srgb, var(--text) 8%, transparent);
  transition: all 0.2s ease;
}
.cat-all.on {
  color: var(--on-accent);
  background: var(--accent-fill);
}

/* 分类小块 */
.cat-chip {
  flex-shrink: 0;
  border: none;
  border-radius: 8px;
  padding: 7px 12px;
  font-size: 13px;
  cursor: pointer;
  color: var(--text-dim);
  background: color-mix(in srgb, var(--text) 8%, transparent);
  transition: all 0.2s ease;
  white-space: nowrap;
}
.cat-chip:hover { color: var(--accent-ink); }
.cat-chip.on {
  color: var(--on-accent);
  background: var(--accent-fill);
  font-weight: 600;
}

/* 数量：跟在分类名后面的小号数字。
   以前靠 opacity 压淡，在浅色胶囊上只剩 3.1:1 —— 这是「有几篇」的信息，
   不是装饰，所以直接给足对比，只收小字号。 */
.cat-count {
  margin-left: 5px;
  font-family: var(--font-mono);
  font-size: 11px;
  vertical-align: 1px;
}
.cat-all.on .cat-count,
.cat-chip.on .cat-count { opacity: 0.85; }

/* 展开/收起按钮 */
.cat-more {
  flex-shrink: 0;
  border: none;
  background: transparent;
  color: var(--accent-ink);
  font-size: 12.5px;
  cursor: pointer;
  padding: 6px 4px;
}
.cat-more:hover { text-decoration: underline; }
</style>
