<script setup>
const year = new Date().getFullYear()
</script>

<template>
  <footer class="footer">
    <p>© {{ year }} 凛冬RinEyce · Built with ❄️ and a lot of coffee</p>
  </footer>
</template>

<style scoped>
.footer {
  border-top: 1px solid var(--border);
  padding: 32px 24px;
  text-align: center;
  color: var(--text-dim);
  font-size: 13.5px;
}
</style>
