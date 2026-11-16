// ============================================================
// useAuth.js —— 登录状态管理（组合式函数）
// 职责：维护「是否已登录 + 当前用户信息」的全局状态，提供登录/注册/登出。
//
// 原理：
//   - JWT 与用户信息存 localStorage（刷新页面后依然有效）；
//   - 启动时（App.vue 调用 init）会用 token 调 /api/user/me 刷新用户资料，
//     顺带校验 token 是否还有效（无效则自动登出）；
//   - 所有组件调用 useAuth() 拿到的都是同一份响应式状态。
// ============================================================

import { computed, ref } from 'vue'
import { api } from '../api'

function loadUser() {
  try {
    return JSON.parse(localStorage.getItem('user') || 'null')
  } catch {
    return null
  }
}

const token = ref(localStorage.getItem('token') || '')
const user = ref(loadUser())
const isLoggedIn = computed(() => !!token.value)
const isAdmin = computed(() => user.value?.role === 'admin')

function persist(t, u) {
  token.value = t || ''
  user.value = u || null
  if (t) localStorage.setItem('token', t)
  else localStorage.removeItem('token')
  if (u) localStorage.setItem('user', JSON.stringify(u))
  else localStorage.removeItem('user')
}

export function useAuth() {
  // 登录：账号 = 邮箱或用户名
  async function login(account, password) {
    const data = await api.login(account, password)
    persist(data.token, data.user)
    return data
  }

  // 注册（成功后自动登录）
  async function register(email, password) {
    const data = await api.register(email, password)
    persist(data.token, data.user)
    return data
  }

  // 登出
  function logout() {
    persist('', null)
  }

  // 启动时刷新用户信息（校验 token；失败自动登出）
  async function init() {
    if (!token.value) return
    try {
      const me = await api.getMe()
      user.value = me
      localStorage.setItem('user', JSON.stringify(me))
    } catch {
      persist('', null)
    }
  }

  // 局部更新用户信息（改昵称/头像后由弹窗调用）
  function setUser(patch) {
    if (!user.value) return
    user.value = { ...user.value, ...patch }
    localStorage.setItem('user', JSON.stringify(user.value))
  }

  // 展示名：昵称 → 邮箱前缀 → 兜底
  const displayName = computed(() => {
    const u = user.value
    if (!u) return ''
    if (u.nickname) return u.nickname
    if (u.email) return u.email.split('@')[0]
    return u.username || '用户'
  })

  // 头像 URL（空则返回空串，组件里渲染首字母占位）
  const avatarUrl = computed(() => user.value?.avatar || '')

  return {
    token,
    user,
    isLoggedIn,
    isAdmin,
    displayName,
    avatarUrl,
    login,
    register,
    logout,
    init,
    setUser
  }
}
