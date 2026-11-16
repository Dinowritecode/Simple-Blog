<script setup>
// ============================================================
// BlogSection.vue —— 首页「博客」区块
// 块状文章网格 + 页码分页；每页条数由后台「设置」页决定（默认 20），
// 文章数超过一页才显示页码。
// 个人名片不在这里（已移到页面最左端的固定栏，见 HomeView.vue）。
// ============================================================

import { computed, onMounted, ref, watch } from 'vue'
import { api } from '../api'
import PostCard from './PostCard.vue'
import CategoryBar from './CategoryBar.vue'
import { useSite } from '../composables/useSite'

const { pageSize } = useSite() // 每页条数：后台可改，默认 20

const posts = ref([])
const page = ref(1)
const total = ref(0)
const loading = ref(true)
const error = ref('')
const category = ref('') // 当前选中的分类，空串 = 全部

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const showPager = computed(() => totalPages.value > 1)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.listPosts({
      page: page.value,
      pageSize: pageSize.value,
      category: category.value
    })
    posts.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)

// 切换分类：回到第一页重新拉
function selectCategory(name) {
  if (category.value === name) return
  category.value = name
  page.value = 1
  load()
}

// 设置是异步取回来的：拿到真正的每页条数后回到第一页重新拉一次
watch(pageSize, () => {
  page.value = 1
  load()
})

// 翻页：重新拉数据后把区块顶部对齐到视口（偏移掉导航栏高度），
// 而不是像独立博客页那样跳回页面最顶端。
function goPage(p) {
  const target = Math.min(Math.max(1, p), totalPages.value)
  if (target === page.value) return
  page.value = target
  load()
  const el = document.getElementById('blog')
  if (el) {
    const top = el.getBoundingClientRect().top + window.scrollY - 76
    window.scrollTo({ top: Math.max(0, top), behavior: 'smooth' })
  }
}

// 页码：页数少时全部列出；页数多时折叠中间，只保留首尾与当前页附近
const pageList = computed(() => {
  const tp = totalPages.value
  const cur = page.value
  if (tp <= 7) return Array.from({ length: tp }, (_, i) => i + 1)
  const nums = [...new Set([1, 2, cur - 1, cur, cur + 1, tp - 1, tp])]
    .filter((n) => n >= 1 && n <= tp)
    .sort((a, b) => a - b)
  const out = []
  let prev = 0
  for (const n of nums) {
    if (prev && n - prev > 1) out.push('...')
    out.push(n)
    prev = n
  }
  return out
})
</script>

<template>
  <section class="section blog" id="blog">
    <div class="section-inner">
      <!-- 分类横栏：去掉「博客」标题后，这条横栏就是列表的顶栏 -->
      <CategoryBar boxed :active="category" @select="selectCategory" />

      <div v-if="loading" class="blog-state">加载中…</div>
      <div v-else-if="error" class="blog-state blog-error">{{ error }}</div>
      <div v-else-if="!posts.length" class="blog-state">
        {{ category ? `「${category}」分类下还没有文章` : '还没有已发布的文章，敬请期待～' }}
      </div>

      <div v-else class="blog-grid">
        <PostCard v-for="post in posts" :key="post.id" :post="post" />
      </div>

      <nav v-if="showPager" class="pager" aria-label="文章分页">
        <button class="pager-btn" :disabled="page <= 1" @click="goPage(page - 1)">上一页</button>
        <template v-for="(p, i) in pageList" :key="i">
          <span v-if="p === '...'" class="pager-gap">…</span>
          <button
            v-else
            class="pager-num"
            :class="{ active: p === page }"
            :aria-current="p === page ? 'page' : undefined"
            @click="goPage(p)"
          >{{ p }}</button>
        </template>
        <button class="pager-btn" :disabled="page >= totalPages" @click="goPage(page + 1)">下一页</button>
      </nav>
    </div>
  </section>
</template>

<style scoped>
.blog-state { color: var(--text-dim); padding: 40px 0; }
.blog-error { color: #e5484d; }

/* 块状布局：等宽卡片自动铺排（960px 内容区下为 3 列） */
.blog-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 22px;
}

.pager {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 36px;
}
.pager-btn,
.pager-num {
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-dim);
  font-family: var(--font-mono);
  font-size: 14px;
  border-radius: 9px;
  padding: 7px 11px;
  cursor: pointer;
  transition: all 0.2s ease;
}
.pager-btn:hover:not(:disabled),
.pager-num:hover { color: var(--accent-ink); border-color: var(--accent-ink); }
.pager-num.active {
  background: var(--accent-fill);
  border-color: var(--accent-fill);
  color: var(--on-accent);
  font-weight: 700;
}
.pager-btn:disabled { opacity: 0.35; cursor: not-allowed; }
.pager-gap { color: var(--text-dim); padding: 0 2px; }
</style>
