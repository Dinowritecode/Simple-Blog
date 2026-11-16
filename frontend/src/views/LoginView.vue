<script setup>
// 访客登录页（评论/互动用）：邮箱或用户名 + 密码
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'

const route = useRoute()
const router = useRouter()
const { login } = useAuth()
const { toast } = useToast()

const account = ref('')
const password = ref('')
const submitting = ref(false)
const error = ref('')

async function submit() {
  if (!account.value || !password.value) {
    error.value = '请输入邮箱（或用户名）和密码'
    return
  }
  submitting.value = true
  error.value = ''
  try {
    const data = await login(account.value.trim(), password.value)
    toast.success(`欢迎回来，${data.user.nickname || data.user.email.split('@')[0]}！`)
    const next = typeof route.query.next === 'string' && route.query.next.startsWith('/') ? route.query.next : '/'
    router.push(next)
  } catch (e) {
    error.value = e.message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <router-link class="login-brand" to="/"><span class="accent">凛冬</span>RinEyce</router-link>
      <h1>登 录</h1>
      <p class="login-sub">登录后即可评论互动</p>

      <form @submit.prevent="submit">
        <label class="field">
          <span>邮箱 / 用户名</span>
          <input v-model="account" type="text" autocomplete="username" placeholder="you@example.com" />
        </label>
        <label class="field">
          <span>密码</span>
          <input v-model="password" type="password" autocomplete="current-password" placeholder="••••••" />
        </label>
        <p v-if="error" class="login-error">{{ error }}</p>
        <button class="btn btn-primary login-btn" type="submit" :disabled="submitting">
          {{ submitting ? '登录中…' : '登 录' }}
        </button>
      </form>

      <p class="login-switch">
        还没有账号？
        <router-link :to="{ path: '/register', query: route.query }">立即注册</router-link>
      </p>
      <router-link class="login-back" to="/">← 返回首页</router-link>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100svh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--bg);
}
.login-card {
  width: 100%;
  max-width: 380px;
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 16px;
  box-shadow: var(--shadow);
  padding: 36px;
}
.login-brand { font-weight: 700; font-size: 16px; color: var(--text); display: inline-block; margin-bottom: 22px; }
.login-brand:hover { text-decoration: none; }
.login-card h1 { font-size: 1.4rem; margin-bottom: 4px; }
.login-sub { color: var(--text-dim); font-size: 13.5px; margin-bottom: 22px; }

.field { display: flex; flex-direction: column; gap: 6px; margin-bottom: 15px; }
.field span { font-size: 13.5px; color: var(--text-dim); }
.field input {
  padding: 11px 14px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg);
  color: var(--text);
  font-size: 15px;
  outline: none;
  transition: border-color 0.2s ease;
}
.field input:focus { border-color: var(--accent); }

.login-error { color: #e5484d; font-size: 13.5px; margin: 4px 0 10px; }
.login-btn { width: 100%; margin-top: 6px; }
.login-btn:disabled { opacity: 0.6; cursor: not-allowed; }

.login-switch { margin-top: 18px; text-align: center; font-size: 13.5px; color: var(--text-dim); }
.login-switch a { font-weight: 600; }
.login-back { display: block; margin-top: 12px; text-align: center; font-size: 13px; color: var(--text-dim); }
</style>
