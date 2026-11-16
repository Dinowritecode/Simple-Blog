<script setup>
// 等级爱心：Momo Talk 列表每行行尾 / 资料卡头像上的等级标识。
//
// 视觉（对照参考图）：
//   浅粉实心 + 玫粉描边 + 深色数字带白色描边光晕（数字在任何底色上都清晰）
//
// filled 两种形态：
//   false（默认）：描边形态 —— 列表行里用的就是它（清爽、不抢眼）
//   true         ：实心形态 —— 需要更醒目的场合
// 两种形态共用同一段爱心路径，所以大小与重心完全一致。
defineProps({
  level: { type: Number, default: 1 },
  size: { type: Number, default: 30 },
  filled: { type: Boolean, default: false }
})
</script>

<template>
  <span
    class="level-heart"
    :class="{ filled }"
    :style="{ width: size + 'px', height: size + 'px' }"
    :title="`等级 ${level}`"
  >
    <svg viewBox="0 0 24 24" :width="size" :height="size" aria-hidden="true">
      <path
        d="M12 21s-8.4-5.1-8.4-11A4.9 4.9 0 0 1 12 7.4 4.9 4.9 0 0 1 20.4 10c0 5.9-8.4 11-8.4 11z"
        fill="currentColor"
        stroke="#f76fa8"
        stroke-width="1.1"
        stroke-linejoin="round"
      />
    </svg>
    <!-- 数字用 SVG 文本画：dominant-baseline=middle 会把「数字的实际墨迹中心」
         精确落到 (12, 14) —— 这是爱心路径的视觉重心。
         心形下方收尖、上方两瓣饱满，视觉质量中心明显低于几何中心（约 13.4），
         实测再往下压 0.6 才看起来正，所以取 y=14。
         白色描边（paint-order: stroke）制造光晕，数字压在爱心上不会糊。
         用 HTML 文本 + translate 对齐时行高与基线会随字号漂移，故用 SVG 文本。 -->
    <svg class="num" viewBox="0 0 24 24" :width="size" :height="size" aria-hidden="true">
      <text
        x="12"
        y="14"
        text-anchor="middle"
        dominant-baseline="middle"
        font-size="9"
        font-weight="700"
        fill="#2b3445"
        stroke="#fff"
        stroke-width="1.5"
        stroke-linejoin="round"
        paint-order="stroke"
      >{{ level }}</text>
    </svg>
  </span>
</template>

<style scoped>
.level-heart {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  /* 描边形态：浅粉实心 + 玫粉描边 */
  color: #f9aec8;
  flex-shrink: 0;
}
/* 实心形态：更饱和一点，数字仍是深色（保证可读） */
.level-heart.filled { color: #f9a0bf; }
/* 数字层叠在爱心之上 */
.num { position: absolute; left: 0; top: 0; }
</style>
