// ============================================================
// useAuth.js —— 登录状态管理（组合式函数）
// 职责：维护「是否已登录」的全局状态，提供登录/登出方法。
//
// 原理：JWT 存 localStorage（浏览器本地存储），刷新页面后依然有效。
// 所有组件调用 useAuth() 拿到的都是同一份状态（模块级 ref 共享）。
// ============================================================

import { computed, ref } from 'vue'
import { api } from '../api'

// 模块级状态：整个应用共享（localStorage 持久化，刷新不丢失）
const token = ref(localStorage.getItem('token') || '')     // JWT
const username = ref(localStorage.getItem('username') || '') // 登录用户名
const isLoggedIn = computed(() => !!token.value)           // 是否已登录（派生值）

export function useAuth() {
  // 登录：调后端接口，成功后把 token 和用户名同时写进内存与 localStorage
  async function login(user, pass) {
    const data = await api.login(user, pass)
    token.value = data.token
    username.value = data.username
    localStorage.setItem('token', data.token)
    localStorage.setItem('username', data.username)
    return data
  }

  // 登出：清空内存与 localStorage（路由守卫会据此把用户送回登录页）
  function logout() {
    token.value = ''
    username.value = ''
    localStorage.removeItem('token')
    localStorage.removeItem('username')
  }

  return { token, username, isLoggedIn, login, logout }
}
