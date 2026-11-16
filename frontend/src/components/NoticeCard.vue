<script setup>
// ============================================================
// NoticeCard.vue —— 首页公告卡片（放在毛玻璃区域右侧栏）
// 内容来自后台「设置」页保存的公告纯文本（最多 300 字），这里只按纯文本渲染。
// ============================================================

import { useSite } from '../composables/useSite'

const { notice } = useSite()
</script>

<template>
  <section class="notice">
    <h3 class="notice-title"><span class="title-bar"></span> 公告</h3>
    <p v-if="notice" class="notice-body">{{ notice }}</p>
    <p v-else class="notice-empty">暂无公告</p>
  </section>
</template>

<style scoped>
/* 与文章卡片同材质：半透明白底 + 不模糊 */
.notice {
  background: var(--card-bg);
  backdrop-filter: var(--card-blur);
  -webkit-backdrop-filter: var(--card-blur);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 20px 18px;
}

.notice-title {
  font-family: var(--font-mono);
  font-size: 15px;
  font-weight: 700;
  margin-bottom: 10px;
  letter-spacing: 0.3px;
  /* 与前面的主题色竖线垂直居中对齐 */
  display: flex;
  align-items: center;
}

/* 纯文本渲染，保留换行；长英文/URL 也强制换行，避免撑破卡片 */
.notice-body {
  font-size: 14px;
  line-height: 1.8;
  color: var(--text);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.notice-empty { font-size: 13.5px; color: var(--text-dim); }
</style>
