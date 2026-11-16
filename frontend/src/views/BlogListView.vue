<script setup>
// 博客列表页（/blog）—— 也是搜索结果的「落地点」：
//   /blog?category=Git     按分类筛选
//   /blog?tag=Firefly      按标签筛选
//   /blog?q=关键字         按关键字筛选（口径与 /search、首页搜索完全一致）
// 三个参数都从地址栏读，所以结果页可以直接分享 / 收藏 / 后退。
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import PostCard from '../components/PostCard.vue'
import CategoryBar from '../components/CategoryBar.vue'
import { useSite } from '../composables/useSite'

const { pageSize: perPage } = useSite() // 每页条数：后台可改，默认 20
const route = useRoute()
const router = useRouter()

const posts = ref([])
const page = ref(1)
const total = ref(0)
const loading = ref(true)
const error = ref('')

const category = ref('') // 当前选中的分类，空串 = 全部
const tag = ref('') // 当前标签筛选
const keyword = ref('') // 当前关键字筛选

// 从地址栏同步筛选条件（点搜索页的分类 / 标签结果会带着 query 跳到这里）
function readRoute() {
  const one = (v) => (Array.isArray(v) ? v[0] : v) || ''
  category.value = one(route.query.category)
  tag.value = one(route.query.tag)
  keyword.value = one(route.query.q)
}

const totalPages = () => Math.max(1, Math.ceil(total.value / perPage.value))

// 空列表文案：把当前生效的筛选条件说清楚，避免用户以为自己点坏了
const emptyText = computed(() => {
  if (keyword.value) return `没有找到和「${keyword.value}」相关的文章`
  if (tag.value) return `没有带「${tag.value}」标签的文章`
  if (category.value) return `「${category.value}」分类下还没有文章`
  return '还没有已发布的文章，敬请期待～'
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.listPosts({
      page: page.value,
      pageSize: perPage.value,
      category: category.value,
      tag: tag.value,
      q: keyword.value
    })
    posts.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  readRoute()
  load()
})

// 切换分类：回到第一页重新拉，并把筛选写进地址栏（刷新 / 分享后仍然生效）
function selectCategory(name) {
  if (category.value === name) return
  router.replace({ path: '/blog', query: cleanQuery({ category: name }) })
}

// 去掉空值参数，避免地址栏出现 ?category=&tag= 这种噪音
function cleanQuery(patch) {
  const next = {
    category: category.value,
    tag: tag.value,
    q: keyword.value,
    ...patch
  }
  Object.keys(next).forEach((k) => {
    if (!next[k]) delete next[k]
  })
  return next
}

// 清掉单个筛选条件（标签 / 关键字用胶囊上的 ✕）
function clearFilter(key) {
  router.replace({ path: '/blog', query: cleanQuery({ [key]: '' }) })
}

// 地址栏变了（分类切换、清筛选、从搜索页跳进来）→ 回第一页重拉
watch(
  () => [route.query.category, route.query.tag, route.query.q],
  () => {
    readRoute()
    page.value = 1
    load()
  }
)

// 设置是异步取回来的：拿到真正的每页条数后回到第一页重新拉一次
watch(perPage, () => {
  page.value = 1
  load()
})

function prev() {
  if (page.value > 1) {
    page.value--
    load()
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }
}
function next() {
  if (page.value < totalPages()) {
    page.value++
    load()
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }
}
</script>

<template>
  <section class="blog section">
    <div class="section-inner">
      <h2 class="blog-title">博客<span class="accent">Blog</span></h2>
      <p class="blog-sub">记录一些关于 Rust / Go / C++ 的思考与实践</p>

      <!-- 分类器：点分类只看该类文章 -->
      <CategoryBar :active="category" @select="selectCategory" />

      <!-- 正在生效的标签 / 关键字筛选：给一个明确的「✕ 取消」，否则用户不知道在看什么 -->
      <div v-if="tag || keyword" class="filters">
        <span class="filters-label">筛选中：</span>
        <span v-if="tag" class="filter-chip">
          #{{ tag }}
          <button class="filter-x" title="取消标签筛选" @click="clearFilter('tag')">✕</button>
        </span>
        <span v-if="keyword" class="filter-chip">
          关键字「{{ keyword }}」
          <button class="filter-x" title="取消关键字筛选" @click="clearFilter('q')">✕</button>
        </span>
      </div>

      <div v-if="loading" class="blog-state">加载中…</div>
      <div v-else-if="error" class="blog-state blog-error">{{ error }}</div>
      <div v-else-if="!posts.length" class="blog-state">
        {{ emptyText }}
      </div>

      <div v-else class="blog-grid">
        <PostCard v-for="post in posts" :key="post.id" :post="post" />
      </div>

      <div v-if="totalPages() > 1" class="pager">
        <button class="btn btn-ghost" :disabled="page <= 1" @click="prev">← 上一页</button>
        <span class="pager-info">{{ page }} / {{ totalPages() }}</span>
        <button class="btn btn-ghost" :disabled="page >= totalPages()" @click="next">下一页 →</button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.blog-title {
  font-size: clamp(1.8rem, 4vw, 2.4rem);
  margin-bottom: 6px;
}
.blog-title .accent { margin-left: 8px; }

.blog-sub { color: var(--text-dim); margin-bottom: 36px; }

/* 正在生效的标签 / 关键字筛选条（分类由上面的 CategoryBar 自己高亮，不在这里重复） */
.filters {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin: -8px 0 22px;
}
.filters-label { font-size: 12.5px; color: var(--text-dim); }
.filter-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 8px 5px 12px;
  border-radius: 8px;
  font-size: 12.5px;
  font-family: var(--font-mono);
  color: var(--accent);
  background: var(--accent-soft);
}
.filter-x {
  border: none;
  background: transparent;
  color: inherit;
  font-size: 11px;
  line-height: 1;
  padding: 2px 4px;
  border-radius: 4px;
  cursor: pointer;
  opacity: 0.75;
}
.filter-x:hover { opacity: 1; background: color-mix(in srgb, var(--accent) 18%, transparent); }

.blog-state {
  text-align: center;
  color: var(--text-dim);
  padding: 80px 0;
}
.blog-error { color: #e5484d; }

.blog-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 22px;
}

.pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 20px;
  /* 毛玻璃胶囊：与卡片、导航栏实底态同一套参数 */
  background: var(--glass-solid-bg);
  backdrop-filter: var(--glass-solid-blur);
  -webkit-backdrop-filter: var(--glass-solid-blur);
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  padding: 8px 18px;
  width: fit-content;
  margin: 44px auto 0;
}
.pager .btn:disabled { opacity: 0.4; cursor: not-allowed; transform: none; }
.pager-info { font-family: var(--font-mono); color: var(--text-dim); font-size: 14px; }
</style>
