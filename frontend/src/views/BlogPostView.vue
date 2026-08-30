<script setup>
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api'
import { formatDate } from '../utils/format'

const route = useRoute()

const post = ref(null)
const loading = ref(true)
const error = ref('')

// 点赞：本地去重，防止同一浏览器重复点赞
const liked = ref(false)
const likeCount = ref(0)

function isLiked(id) {
  const list = JSON.parse(localStorage.getItem('liked_posts') || '[]')
  return list.includes(id)
}

function markLiked(id) {
  const list = JSON.parse(localStorage.getItem('liked_posts') || '[]')
  if (!list.includes(id)) {
    list.push(id)
    localStorage.setItem('liked_posts', JSON.stringify(list))
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.getPost(route.params.id)
    post.value = data
    likeCount.value = data.likes
    liked.value = isLiked(data.id)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function doLike() {
  if (liked.value || !post.value) return
  try {
    const data = await api.likePost(post.value.id)
    likeCount.value = data.likes
    liked.value = true
    markLiked(post.value.id)
  } catch (e) {
    error.value = e.message
  }
}

// 分享：优先系统分享，否则复制链接
const copied = ref(false)

async function doShare() {
  const url = window.location.href
  const title = post.value?.title || '凛冬RinEyce 的博客'
  if (navigator.share) {
    try {
      await navigator.share({ title, url })
    } catch {
      /* 用户取消分享 */
    }
    return
  }
  try {
    await navigator.clipboard.writeText(url)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch {
    error.value = '复制链接失败，请手动复制地址栏链接'
  }
}
</script>

<template>
  <section class="post-view section">
    <div class="post-wrap">
      <div v-if="loading" class="post-state">加载中…</div>
      <div v-else-if="error" class="post-state post-error">{{ error }}</div>

      <article v-else-if="post">
        <div v-if="post.cover" class="post-cover">
          <img :src="post.cover" :alt="post.title" />
        </div>

        <header class="post-head">
          <h1>{{ post.title }}</h1>
          <div class="post-meta">
            <span>🕐 {{ formatDate(post.createdAt) }}</span>
            <span>★ {{ likeCount }}</span>
          </div>
        </header>

        <!-- 文章正文（wangEditor 输出的 HTML） -->
        <div class="post-content" v-html="post.content"></div>

        <footer class="post-actions">
          <button class="btn like-btn" :class="{ liked }" :disabled="liked" @click="doLike">
            {{ liked ? '已点赞' : '👍 点赞' }} <span class="like-num">{{ likeCount }}</span>
          </button>
          <button class="btn btn-ghost" @click="doShare">
            {{ copied ? '✅ 链接已复制' : '🔗 分享' }}
          </button>
          <router-link class="btn btn-ghost" to="/blog">← 返回列表</router-link>
        </footer>
      </article>
    </div>
  </section>
</template>

<style scoped>
.post-view { min-height: 60vh; }

.post-wrap {
  max-width: 780px;
  margin: 0 auto;
}

.post-state { text-align: center; color: var(--text-dim); padding: 80px 0; }
.post-error { color: #e5484d; }

.post-cover {
  border-radius: var(--radius);
  overflow: hidden;
  margin-bottom: 28px;
  border: 1px solid var(--border);
}
.post-cover img { width: 100%; display: block; }

.post-head { margin-bottom: 28px; }
.post-head h1 {
  font-size: clamp(1.6rem, 4vw, 2.2rem);
  line-height: 1.35;
  margin-bottom: 12px;
}

.post-meta {
  display: flex;
  gap: 20px;
  color: var(--text-dim);
  font-family: var(--font-mono);
  font-size: 13.5px;
}

.post-actions {
  margin-top: 44px;
  padding-top: 24px;
  border-top: 1px dashed var(--border);
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.like-btn { background: var(--accent-soft); color: var(--accent); }
.like-btn:hover { transform: translateY(-2px); }
.like-btn.liked { opacity: 0.75; cursor: default; }
.like-num { font-family: var(--font-mono); margin-left: 4px; }
</style>
