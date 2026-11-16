<script setup>
const year = new Date().getFullYear()
</script>

<template>
  <footer class="footer">
    <p>© {{ year }} 凛冬RinEyce · Built with ❄️ and a lot of coffee</p>
    <!-- 备案号：按要求挂在页脚，链到工信部备案查询系统 -->
    <p class="footer-icp">
      <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer">
        蜀ICP备2026065293号
      </a>
    </p>
  </footer>
</template>

<style scoped>
.footer {
  border-top: 1px solid var(--glass-border);
  padding: 32px 24px;
  text-align: center;
  color: var(--text-dim);
  font-size: 13.5px;
  /* 与首页下方那块毛玻璃面板同材质：否则面板结束后会在最底部露出一条未经模糊的背景图 */
  background: var(--glass-panel-bg);
  backdrop-filter: var(--glass-panel-blur);
  -webkit-backdrop-filter: var(--glass-panel-blur);
}

.footer p { margin: 0; }
.footer p + p { margin-top: 8px; }

.footer-icp {
  font-size: 12px;
  letter-spacing: 0.02em;
  /* 备案号是法律要求必须清晰可见的信息，比版权行再实一点 */
  color: color-mix(in srgb, var(--text) 72%, transparent);
}
.footer-icp a {
  color: inherit;
  transition: color 0.18s ease;
}
.footer-icp a:hover { color: var(--accent-ink); }
</style>
