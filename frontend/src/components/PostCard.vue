<script setup>
// 文章卡片：封面（可选）→ 标题 → 日期 + 分类 → 摘要 → 标签 → 浏览量 / 点赞量
// 图标放在 src/others/ 下，用 CSS mask 引用，这样颜色跟随文字（深色主题也正常）。
import { computed } from 'vue'
import { formatDate } from '../utils/format'

const props = defineProps({
  post: { type: Object, required: true }
})

// 标签在后端是一个逗号分隔的字符串，这里拆成数组渲染（中英文逗号都支持）
const tagList = computed(() =>
  (props.post.tags || '')
    .split(/[,，]/)
    .map((t) => t.trim())
    .filter(Boolean)
)
</script>

<template>
  <router-link class="post-card" :to="`/blog/${post.slug}`">
    <!-- 有封面才渲染这一段；没有封面时整块不存在，卡片自然变矮 -->
    <div v-if="post.cover" class="post-cover">
      <img :src="post.cover" :alt="post.title" loading="lazy" />
    </div>

    <div class="post-body">
      <h3 class="post-title">{{ post.title }}</h3>

      <!-- 日期 + 分类（分类用 book.svg 图标） -->
      <div class="post-meta">
        <span class="post-meta-item">
          <svg class="ico-svg" viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/></svg>
          {{ formatDate(post.createdAt) }}
        </span>
        <span v-if="post.category" class="post-meta-item">
          <i class="ico ico-cat"></i>
          {{ post.category }}
        </span>
      </div>

      <p class="post-summary">{{ post.summary || '……' }}</p>

      <!-- 标签：浅色小块 -->
      <div v-if="tagList.length" class="post-tags">
        <span v-for="t in tagList" :key="t" class="post-tag">#{{ t }}</span>
      </div>

      <!-- 浏览量 / 点赞量 / 评论数 -->
      <div class="post-stats">
        <span class="post-stat"><i class="ico ico-views"></i>{{ post.views || 0 }}</span>
        <span class="post-stat"><i class="ico ico-likes"></i>{{ post.likes || 0 }}</span>
        <span class="post-stat"><i class="ico ico-comments"></i>{{ post.comments || 0 }}</span>
      </div>
    </div>

    <!-- 悬停时右侧滑入的箭头 -->
    <span class="post-arrow" aria-hidden="true">
      <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>
    </span>
  </router-link>
</template>

<style scoped>
.post-card {
  position: relative;
  display: flex;
  flex-direction: column;
  background: var(--card);
  border: 1px solid var(--glass-border);
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

/* 封面：铺满卡片宽度 */
.post-cover { height: 150px; background: var(--bg-alt); overflow: hidden; }
.post-cover img { width: 100%; height: 100%; object-fit: cover; display: block; }

.post-body {
  padding: 16px 18px 18px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex: 1;
}

.post-title { font-size: 1.05rem; line-height: 1.45; font-weight: 700; }
.post-card:hover .post-title { color: var(--accent-ink); }

/* 日期 + 分类 */
.post-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 14px;
  font-size: 12.5px;
  color: var(--text-dim);
}
.post-meta-item { display: inline-flex; align-items: center; gap: 5px; }

/* 图标：用 mask 让 svg 跟随文字颜色（src/others 下的资源）。
   注意 mask 型图标必须靠 background-color 上色，所以它和内联 SVG 要分开两个类，
   否则内联 svg 也会被填成实心色块。 */
.ico {
  display: inline-block;
  width: 13px;
  height: 13px;
  flex-shrink: 0;
  background-color: currentColor;
}
.ico-cat {
  -webkit-mask: url('../others/book.svg') center / contain no-repeat;
  mask: url('../others/book.svg') center / contain no-repeat;
}
.ico-views {
  -webkit-mask: url('../others/preview.svg') center / contain no-repeat;
  mask: url('../others/preview.svg') center / contain no-repeat;
}
.ico-likes {
  -webkit-mask: url('../others/thumbs-up.svg') center / contain no-repeat;
  mask: url('../others/thumbs-up.svg') center / contain no-repeat;
}
.ico-comments {
  -webkit-mask: url('../others/comment.svg') center / contain no-repeat;
  mask: url('../others/comment.svg') center / contain no-repeat;
}

/* 内联 SVG 图标（日期）：只定尺寸，不能有 background-color */
.ico-svg { width: 13px; height: 13px; flex-shrink: 0; }

/* 摘要：最多两行 */
.post-summary {
  color: var(--text-dim);
  font-size: 13.5px;
  line-height: 1.7;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  flex: 1;
}

/* 标签：主题色淡底 + 主题色文字（对应参考图里的浅色标签） */
.post-tags { display: flex; flex-wrap: wrap; gap: 6px; }
.post-tag {
  font-size: 11.5px;
  line-height: 1;
  /* 淡底上的主题色文字用 --accent-ink（比品牌色深一档），11.5px 也看得清 */
  color: var(--accent-ink);
  background: color-mix(in srgb, var(--accent) 14%, transparent);
  border-radius: 6px;
  padding: 5px 9px;
}

/* 浏览量 / 点赞量 / 评论数（不加分隔线） */
.post-stats {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-top: 2px;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-dim);
}
.post-stat { display: inline-flex; align-items: center; gap: 5px; }

/* 悬停箭头：贴在卡片右侧中部，默认隐藏 */
.post-arrow {
  position: absolute;
  top: 50%;
  right: 0;
  transform: translate(6px, -50%);
  width: 26px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px 0 0 8px;
  background: color-mix(in srgb, var(--accent) 16%, transparent);
  color: var(--accent-ink);
  opacity: 0;
  transition: opacity 0.22s ease, transform 0.22s ease;
  pointer-events: none;
}
.post-card:hover .post-arrow { opacity: 1; transform: translate(0, -50%); }
</style>
