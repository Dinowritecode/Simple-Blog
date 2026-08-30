<script setup>
import { ref } from 'vue'
import { useTheme } from '../composables/useTheme'

const { theme, toggle } = useTheme()
const menuOpen = ref(false)

// 站内页面链接
const pageLinks = [
  { to: { path: '/' }, label: '首页' },
  { to: { path: '/blog' }, label: '博客' }
]
// 首页区块锚点链接
const sectionLinks = [
  { to: { path: '/', hash: '#about' }, label: '关于' },
  { to: { path: '/', hash: '#skills' }, label: '技能' },
  { to: { path: '/', hash: '#projects' }, label: '项目' },
  { to: { path: '/', hash: '#contact' }, label: '联系' }
]

function closeMenu() {
  menuOpen.value = false
}
</script>

<template>
  <header class="nav">
    <div class="nav-inner">
      <router-link class="nav-brand" to="/" @click="closeMenu">凛冬<span class="accent">RinEyce</span></router-link>

      <nav class="nav-links" :class="{ open: menuOpen }">
        <router-link
          v-for="link in pageLinks"
          :key="link.label"
          :to="link.to"
          @click="closeMenu"
        >{{ link.label }}</router-link>
        <router-link
          v-for="link in sectionLinks"
          :key="link.label"
          :to="link.to"
          @click="closeMenu"
        >{{ link.label }}</router-link>
      </nav>

      <div class="nav-actions">
        <button class="icon-btn" aria-label="切换主题" title="切换主题" @click="toggle">
          <svg v-if="theme === 'light'" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
          <svg v-else viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"/></svg>
        </button>
        <button class="icon-btn menu-btn" aria-label="打开菜单" title="菜单" @click="menuOpen = !menuOpen">
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 7h16M4 12h16M4 17h16"/></svg>
        </button>
      </div>
    </div>
  </header>
</template>

<style scoped>
.nav {
  position: sticky;
  top: 0;
  z-index: 100;
  background: color-mix(in srgb, var(--bg) 85%, transparent);
  backdrop-filter: blur(12px);
  border-bottom: 1px solid var(--border);
}

.nav-inner {
  max-width: 1080px;
  margin: 0 auto;
  padding: 0 24px;
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.nav-brand {
  font-weight: 700;
  font-size: 17px;
  color: var(--text);
  letter-spacing: 0.5px;
}
.nav-brand:hover { text-decoration: none; }

.nav-links { display: flex; gap: 32px; }
.nav-links a {
  color: var(--text-dim);
  font-size: 15px;
  transition: color 0.2s ease;
}
.nav-links a:hover { color: var(--text); text-decoration: none; }

.nav-actions { display: flex; align-items: center; gap: 8px; }

.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: transparent;
  color: var(--text-dim);
  cursor: pointer;
  transition: all 0.2s ease;
}
.icon-btn:hover { color: var(--text); border-color: var(--accent); }

.menu-btn { display: none; }

@media (max-width: 760px) {
  .menu-btn { display: inline-flex; }

  .nav-links {
    position: fixed;
    inset: 60px 0 auto 0;
    flex-direction: column;
    gap: 0;
    background: var(--bg);
    border-bottom: 1px solid var(--border);
    padding: 8px 24px 16px;
    transform: translateY(-12px);
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.25s ease, transform 0.25s ease;
  }
  .nav-links.open { opacity: 1; transform: none; pointer-events: auto; }
  .nav-links a { padding: 12px 0; border-bottom: 1px solid var(--border); }
  .nav-links a:last-child { border-bottom: none; }
}
</style>
