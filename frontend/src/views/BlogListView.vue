<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'
import PostCard from '../components/PostCard.vue'

const posts = ref([])
const page = ref(1)
const pageSize = 9
const total = ref(0)
const loading = ref(true)
const error = ref('')

const totalPages = () => Math.max(1, Math.ceil(total.value / pageSize))

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.listPosts({ page: page.value, pageSize })
    posts.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)

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

      <div v-if="loading" class="blog-state">加载中…</div>
      <div v-else-if="error" class="blog-state blog-error">{{ error }}</div>
      <div v-else-if="!posts.length" class="blog-state">还没有已发布的文章，敬请期待～</div>

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
  margin-top: 44px;
}
.pager .btn:disabled { opacity: 0.4; cursor: not-allowed; transform: none; }
.pager-info { font-family: var(--font-mono); color: var(--text-dim); font-size: 14px; }
</style>
