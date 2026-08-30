import { createRouter, createWebHistory } from 'vue-router'
import { useAuth } from '../composables/useAuth'

const routes = [
  { path: '/', name: 'home', component: () => import('../views/HomeView.vue') },
  { path: '/blog', name: 'blog', component: () => import('../views/BlogListView.vue') },
  { path: '/blog/:id', name: 'post', component: () => import('../views/BlogPostView.vue') },

  { path: '/admin/login', name: 'admin-login', component: () => import('../views/AdminLoginView.vue'), meta: { hideChrome: true } },

  {
    path: '/admin',
    component: () => import('../views/AdminLayout.vue'),
    meta: { hideChrome: true, requiresAuth: true },
    children: [
      { path: '', name: 'admin-dashboard', component: () => import('../views/AdminDashboardView.vue') },
      { path: 'posts/new', name: 'admin-new', component: () => import('../views/AdminEditorView.vue') },
      { path: 'posts/:id/edit', name: 'admin-edit', component: () => import('../views/AdminEditorView.vue') }
    ]
  },

  { path: '/:pathMatch(.*)*', redirect: '/' }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior(to, from, savedPosition) {
    if (to.hash) return { el: to.hash, behavior: 'smooth', top: 64 }
    if (savedPosition) return savedPosition
    return { top: 0 }
  }
})

router.beforeEach((to) => {
  const { token } = useAuth()
  if (to.meta.requiresAuth && !token.value) {
    return { name: 'admin-login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'admin-login' && token.value) {
    return { name: 'admin-dashboard' }
  }
})

export default router
