<script setup>
// ============================================================
// PageBackdrop.vue —— 固定背景图层（首页与文章详情页共用组件，但图是两张）
// position: fixed + z-index: -1，页面滚动时它纹丝不动，
// 上层的毛玻璃区域才能「滑上来」盖住它。
//
// 图片地址由父组件通过 src 传进来：
//   首页     → homeBg（后台「首页背景图」）
//   文章详情 → postBg（后台「文章页背景图」）
// 两张图完全独立；src 为空就什么都不渲染（该页用默认纯色网格背景）。
// ============================================================

defineProps({
  src: { type: String, default: '' }
})
</script>

<template>
  <div
    v-if="src"
    class="bg-fixed"
    :style="{ backgroundImage: `url(${src})` }"
    aria-hidden="true"
  ></div>
</template>

<style scoped>
.bg-fixed {
  position: fixed;
  inset: 0;
  z-index: -1;
  background-position: center;
  background-size: cover;
  background-repeat: no-repeat;
}
</style>
