<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'

const route = useRoute()
const router = useRouter()
const { login } = useAuth()

const username = ref('')
const password = ref('')
const submitting = ref(false)
const error = ref('')

async function submit() {
  if (!username.value || !password.value) {
    error.value = '请输入用户名和密码'
    return
  }
  submitting.value = true
  error.value = ''
  try {
    await login(username.value, password.value)
    router.push(route.query.redirect || '/admin')
  } catch (e) {
    error.value = e.message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <form class="login-card" @submit.prevent="submit">
      <router-link class="login-brand" to="/">凛冬<span class="accent">RinEyce</span></router-link>
      <h1>后台管理</h1>
      <p class="login-sub">登录以管理博客文章</p>

      <label class="field">
        <span>用户名</span>
        <input v-model="username" type="text" autocomplete="username" placeholder="admin" />
      </label>
      <label class="field">
        <span>密码</span>
        <input v-model="password" type="password" autocomplete="current-password" placeholder="••••••" />
      </label>

      <p v-if="error" class="login-error">{{ error }}</p>

      <button class="btn btn-primary login-btn" type="submit" :disabled="submitting">
        {{ submitting ? '登录中…' : '登 录' }}
      </button>

      <router-link class="login-back" to="/">← 返回首页</router-link>
    </form>
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
  padding: 40px 36px;
  display: flex;
  flex-direction: column;
}

.login-brand {
  font-weight: 700;
  font-size: 16px;
  color: var(--text);
  margin-bottom: 24px;
}
.login-brand:hover { text-decoration: none; }

.login-card h1 { font-size: 1.5rem; margin-bottom: 4px; }
.login-sub { color: var(--text-dim); font-size: 14px; margin-bottom: 28px; }

.field { display: flex; flex-direction: column; gap: 6px; margin-bottom: 16px; }
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

.login-error { color: #e5484d; font-size: 13.5px; margin: 4px 0 12px; }

.login-btn { margin-top: 8px; }
.login-btn:disabled { opacity: 0.6; cursor: not-allowed; }

.login-back {
  margin-top: 20px;
  text-align: center;
  font-size: 13.5px;
  color: var(--text-dim);
}
</style>
