<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'
import PostCard from './PostCard.vue'

const posts = ref([])
const loadFailed = ref(false)

onMounted(async () => {
  try {
    const data = await api.listPosts({ page: 1, pageSize: 3 })
    posts.value = data.items || []
  } catch {
    loadFailed.value = true
  }
})
</script>

<template>
  <section v-if="!loadFailed" class="section latest" id="latest">
    <div class="section-inner">
      <h2 class="section-title" v-reveal><span class="accent">#</span> 最新文章</h2>

      <div v-if="posts.length" class="latest-grid">
        <PostCard v-for="post in posts" :key="post.id" :post="post" v-reveal />
      </div>
      <p v-else class="latest-empty" v-reveal>还没有文章，敬请期待～</p>

      <div class="latest-more" v-reveal>
        <router-link class="btn btn-ghost" to="/blog">查看全部文章 →</router-link>
      </div>
    </div>
  </section>
</template>

<style scoped>
.latest { padding-bottom: 40px; }

.latest-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 22px;
}

.latest-empty { color: var(--text-dim); }

.latest-more { margin-top: 32px; text-align: center; }
</style>
