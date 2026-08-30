import { createRouter, createWebHashHistory } from 'vue-router'

// 每个章节一个独立组件、一条路由（hash 模式：无需服务器配置，构建后可直接双击打开）
const routes = [
  { path: '/', redirect: '/intro' },
  { path: '/intro', name: 'intro', component: () => import('../chapters/Ch01Intro.vue') },
  { path: '/arch', name: 'arch', component: () => import('../chapters/Ch02Arch.vue') },
  { path: '/files', name: 'files', component: () => import('../chapters/Ch03Files.vue') },
  { path: '/db-design', name: 'db-design', component: () => import('../chapters/Ch04DbDesign.vue') },
  { path: '/db-access', name: 'db-access', component: () => import('../chapters/Ch05DbAccess.vue') },
  { path: '/deploy', name: 'deploy', component: () => import('../chapters/Ch06Deploy.vue') },
  { path: '/admin', name: 'admin', component: () => import('../chapters/Ch07Admin.vue') },
  { path: '/usage', name: 'usage', component: () => import('../chapters/Ch08Usage.vue') },
  { path: '/api-overview', name: 'api-overview', component: () => import('../chapters/Ch09ApiOverview.vue') },
  { path: '/api-public', name: 'api-public', component: () => import('../chapters/Ch10ApiPublic.vue') },
  { path: '/api-admin', name: 'api-admin', component: () => import('../chapters/Ch11ApiAdmin.vue') },
  { path: '/faq', name: 'faq', component: () => import('../chapters/Ch12Faq.vue') },
  { path: '/dev', name: 'dev', component: () => import('../chapters/Ch13Dev.vue') },
  { path: '/:pathMatch(.*)*', redirect: '/intro' }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
  // 切换章节时回到顶部
  scrollBehavior() {
    return { top: 0 }
  }
})

export default router
