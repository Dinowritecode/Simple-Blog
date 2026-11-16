import { createRouter, createWebHistory, START_LOCATION } from 'vue-router'
import { useAuth } from '../composables/useAuth'

const routes = [
  { path: '/', name: 'home', component: () => import('../views/HomeView.vue') },
  { path: '/blog', name: 'blog', component: () => import('../views/BlogListView.vue') },
  { path: '/blog/:slug', name: 'post', component: () => import('../views/BlogPostView.vue') },
  // 说说：类似 Twitter 的短动态，和文章完全分开
  { path: '/moments', name: 'moments', component: () => import('../views/MomentsView.vue') },
  // 个人主页：导航栏头像、任意评论区 / 说说的头像与昵称都指向这里
  { path: '/u/:id', name: 'user', component: () => import('../views/UserProfileView.vue') },
  // 站内搜索：?q=关键字&type=all|post|category|tag|user
  { path: '/search', name: 'search', component: () => import('../views/SearchView.vue') },

  // 访客：登录 / 注册（右上角按钮进入，用于评论互动）
  { path: '/login', name: 'login', component: () => import('../views/LoginView.vue'), meta: { hideChrome: true, guestOnly: true } },
  { path: '/register', name: 'register', component: () => import('../views/RegisterView.vue'), meta: { hideChrome: true, guestOnly: true } },

  // 站长：独立后台登录页（仅登录，无注册 —— 站长账号部署时自动创建）
  { path: '/admin/login', name: 'admin-login', component: () => import('../views/AdminLoginView.vue'), meta: { hideChrome: true } },

  {
    path: '/admin',
    component: () => import('../views/AdminLayout.vue'),
    meta: { hideChrome: true, requiresAuth: true, requiresAdmin: true },
    children: [
      { path: '', name: 'admin-dashboard', component: () => import('../views/AdminDashboardView.vue') },
      { path: 'posts/new', name: 'admin-new', component: () => import('../views/AdminEditorView.vue') },
      { path: 'posts/:id/edit', name: 'admin-edit', component: () => import('../views/AdminEditorView.vue') },
      { path: 'comments', name: 'admin-comments', component: () => import('../views/AdminCommentsView.vue') },
      { path: 'users', name: 'admin-users', component: () => import('../views/AdminUsersView.vue') },
      { path: 'settings', name: 'admin-settings', component: () => import('../views/AdminSettingsView.vue') },
      { path: 'backup', name: 'admin-backup', component: () => import('../views/AdminBackupView.vue') }
    ]
  },

  { path: '/:pathMatch(.*)*', redirect: '/' }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior(to, from, savedPosition) {
    // 首次进入站点（含刷新）一律从顶部开始。
    // 否则：点过「博客」之后地址栏会留下 /#blog，再打开/刷新就会被浏览器
    // 一路滚到博客区块，首屏（封面 + 波浪 + 白色区域上沿）整个被跳过。
    // 站内点导航仍然照常平滑滚到对应区块 —— 这里只管「刚打开」这一次。
    if (from === START_LOCATION) return { top: 0 }
    // 返回/前进优先「回到离开时的位置」。
    // 必须排在 hash 判断前面：从文章页退回首页时，地址栏可能还带着 /#blog，
    // 先看 hash 就会把「回到刚才读到的位置」变成「滚到博客区块」——
    // 这就是「从文章页返回会往下滚」的原因。
    if (savedPosition) return savedPosition
    if (to.hash) return { el: to.hash, behavior: 'smooth', top: 64 }
    return { top: 0 }
  }
})

router.beforeEach((to) => {
  const { isLoggedIn, isAdmin } = useAuth()
  const needLogin = to.matched.some((r) => r.meta.requiresAuth)
  const needAdmin = to.matched.some((r) => r.meta.requiresAdmin)
  const guestOnly = to.matched.some((r) => r.meta.guestOnly)

  // 后台：未登录 → 站长登录页；非管理员（已登录普通用户）→ 回首页
  if (needLogin) {
    if (!isLoggedIn.value) {
      return { path: '/admin/login', query: { next: to.fullPath } }
    }
    if (needAdmin && !isAdmin.value) return { path: '/' }
  }
  // 站长登录页：已登录管理员 → 直接进后台；已登录普通用户 → 回首页
  if (to.name === 'admin-login' && isLoggedIn.value) {
    return isAdmin.value ? { path: '/admin' } : { path: '/' }
  }
  // 访客登录/注册页：已登录 → 回首页（站长也一样）
  if (guestOnly && isLoggedIn.value) return { path: '/' }
})

export default router
