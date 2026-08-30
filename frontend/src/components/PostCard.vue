<script setup>
import { formatDate } from '../utils/format'

defineProps({
  post: { type: Object, required: true }
})
</script>

<template>
  <router-link class="post-card" :to="`/blog/${post.id}`">
    <div class="post-cover">
      <img v-if="post.cover" :src="post.cover" :alt="post.title" loading="lazy" />
      <span v-else class="post-cover-fallback">✍️</span>
    </div>
    <div class="post-body">
      <h3>{{ post.title }}</h3>
      <p>{{ post.summary || '……' }}</p>
      <div class="post-meta">
        <span class="post-date">{{ formatDate(post.createdAt) }}</span>
        <span class="post-likes">★ {{ post.likes }}</span>
      </div>
    </div>
  </router-link>
</template>

<style scoped>
.post-card {
  display: flex;
  flex-direction: column;
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  overflow: hidden;
  box-shadow: var(--shadow);
  color: var(--text);
  transition: transform 0.25s ease, border-color 0.25s ease, box-shadow 0.25s ease;
}
.post-card:hover {
  transform: translateY(-5px);
  border-color: var(--accent);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.08);
  text-decoration: none;
}

.post-cover {
  height: 160px;
  background: var(--bg-alt);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.post-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.post-cover-fallback {
  font-size: 2.4rem;
  opacity: 0.35;
}

.post-body { padding: 18px 20px 16px; display: flex; flex-direction: column; gap: 8px; flex: 1; }
.post-body h3 { font-size: 1.08rem; line-height: 1.4; }
.post-body p {
  color: var(--text-dim);
  font-size: 14px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  flex: 1;
}

.post-meta {
  display: flex;
  justify-content: space-between;
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--text-dim);
  border-top: 1px dashed var(--border);
  padding-top: 10px;
}
</style>
