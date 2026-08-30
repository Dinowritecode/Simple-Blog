<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'

// 侧边栏导航分组：组名 → 章节链接列表
const groups = [
  {
    label: '开始',
    items: [
      { to: '/intro', text: '1. 项目简介' },
      { to: '/arch', text: '2. 系统架构与原理' },
      { to: '/files', text: '3. 目录结构与文件作用' }
    ]
  },
  {
    label: '数据库',
    items: [
      { to: '/db-design', text: '4. 数据库设计' },
      { to: '/db-access', text: '5. 数据库访问方法' }
    ]
  },
  {
    label: '部署与使用',
    items: [
      { to: '/deploy', text: '6. 完整部署流程' },
      { to: '/admin', text: '7. 后台访问方法' },
      { to: '/usage', text: '8. 使用说明' }
    ]
  },
  {
    label: '接口',
    items: [
      { to: '/api-overview', text: '9. API 总览与约定' },
      { to: '/api-public', text: '10. 公开接口详解' },
      { to: '/api-admin', text: '11. 管理接口详解' }
    ]
  },
  {
    label: '开发',
    items: [
      { to: '/faq', text: '12. 常见问题 FAQ' },
      { to: '/dev', text: '13. 参与开发指南' }
    ]
  }
]

// 回到顶部按钮
const showTop = ref(false)
function onScroll() {
  showTop.value = window.scrollY > 400
}
function toTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}
onMounted(() => window.addEventListener('scroll', onScroll))
onBeforeUnmount(() => window.removeEventListener('scroll', onScroll))
</script>

<template>
  <div class="layout">
    <!-- 左侧导航 -->
    <aside class="sidebar">
      <div class="brand">凛冬<span>RinEyce</span> · 文档</div>
      <nav>
        <template v-for="group in groups" :key="group.label">
          <div class="group">{{ group.label }}</div>
          <router-link v-for="item in group.items" :key="item.to" :to="item.to">{{ item.text }}</router-link>
        </template>
      </nav>
    </aside>

    <!-- 正文区域：当前章节渲染在这里（无刷新切换） -->
    <main class="content">
      <h1>凛冬博客 · 项目在线文档</h1>
      <p class="lead">本文档面向所有人：只看不写的小白、日常使用的管理员、想参与开发的程序员。从「这项目是干嘛的」到「每个文件干什么、每个接口怎么调」，一步到位。</p>

      <router-view />

      <hr class="divider" />
      <p class="center comment">凛冬RinEyce · 项目在线文档 · 如有疑问请先看「常见问题」或 README</p>
    </main>
  </div>

  <button v-if="showTop" class="totop" title="回到顶部" @click="toTop">↑</button>
</template>

<style scoped>
.layout { display: flex; min-height: 100vh; }

.sidebar {
  width: 260px; flex-shrink: 0; position: sticky; top: 0; height: 100vh;
  background: var(--card); border-right: 1px solid var(--border);
  overflow-y: auto; padding: 24px 0;
}
.sidebar .brand { padding: 0 22px 16px; font-weight: 700; font-size: 16px; border-bottom: 1px solid var(--border); margin-bottom: 12px; }
.sidebar .brand span { color: var(--accent); }
.sidebar nav a {
  display: block; padding: 7px 22px; color: var(--dim); font-size: 13.8px;
  border-left: 3px solid transparent; transition: all .15s ease;
}
.sidebar nav a:hover { color: var(--text); background: var(--accent-soft); text-decoration: none; }
/* 当前章节高亮（vue-router 自动加 active 类） */
.sidebar nav a.router-link-active { color: var(--accent); border-left-color: var(--accent); background: var(--accent-soft); font-weight: 600; }
.sidebar nav .group { padding: 14px 22px 4px; font-size: 11.5px; color: #9aa3af; letter-spacing: 1px; text-transform: uppercase; }

.content { flex: 1; min-width: 0; padding: 48px 56px 80px; max-width: 960px; }
.content h1 { font-size: 2rem; margin-bottom: 8px; }
.content .lead { color: var(--dim); font-size: 1.05rem; margin-bottom: 24px; }

.totop {
  position: fixed; right: 28px; bottom: 28px; width: 40px; height: 40px; border-radius: 50%;
  background: var(--accent); color: #fff; border: none; cursor: pointer; font-size: 18px;
  box-shadow: 0 4px 14px rgba(37,99,235,.35); z-index: 99;
}

@media (max-width: 900px) {
  .sidebar { display: none; }
  .content { padding: 28px 20px 60px; }
}
</style>
