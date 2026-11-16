<script setup>
// 站长后台登录页（/admin/login）
// 站长账号部署时自动创建（随机初始密码打印在启动日志，或用 ADMIN_PASSWORD 固定），无需注册。
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'

const route = useRoute()
const router = useRouter()
const { login, isAdmin, logout } = useAuth()
const { toast } = useToast()

const account = ref('')
const password = ref('')
const submitting = ref(false)
const error = ref('')

async function submit() {
  if (!account.value || !password.value) {
    error.value = '请输入站长账号和密码'
    return
  }
  submitting.value = true
  error.value = ''
  try {
    const data = await login(account.value.trim(), password.value)
    if (!isAdmin.value) {
      // 登录成功但不是站长：清除登录态并提示（普通用户请走前台登录）
      logout()
      error.value = '该账号不是站长，普通用户请从右上角登录'
      return
    }
    toast.success(`站长你好，${data.user.nickname || data.user.username || 'admin'}！`)
    const next = typeof route.query.next === 'string' && route.query.next.startsWith('/admin') ? route.query.next : '/admin'
    router.push(next)
  } catch (e) {
    error.value = e.message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="admin-login-page">
    <form class="admin-login-card" @submit.prevent="submit">
      <router-link class="login-brand" to="/"><span class="accent">凛冬</span>RinEyce</router-link>
      <h1>站长后台</h1>
      <p class="login-sub">站长账号部署时自动创建，此页仅用于登录</p>

      <label class="field">
        <span>账号</span>
        <input v-model="account" type="text" autocomplete="username" placeholder="admin" />
      </label>
      <label class="field">
        <span>密码</span>
        <input v-model="password" type="password" autocomplete="current-password" placeholder="••••••" />
      </label>

      <p v-if="error" class="login-error">{{ error }}</p>

      <button class="btn btn-primary login-btn" type="submit" :disabled="submitting">
        {{ submitting ? '登录中…' : '进入后台' }}
      </button>

      <div class="login-tip">
        忘了密码？在运行 <code>go run .</code> 的终端输入
        <code>changepass 新密码</code> 即可重置。
      </div>
      <router-link class="login-back" to="/">← 返回首页</router-link>
    </form>
  </div>
</template>

<style scoped>
.admin-login-page {
  min-height: 100svh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--bg);
}
.admin-login-card {
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
.login-brand { font-weight: 700; font-size: 16px; color: var(--text); margin-bottom: 24px; }
.login-brand:hover { text-decoration: none; }
.admin-login-card h1 { font-size: 1.5rem; margin-bottom: 4px; }
.login-sub { color: var(--text-dim); font-size: 13.5px; margin-bottom: 24px; }

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

.login-tip {
  margin-top: 16px;
  font-size: 12.5px;
  color: var(--text-dim);
  background: var(--bg-alt);
  border-radius: 10px;
  padding: 10px 12px;
  line-height: 1.8;
}
.login-tip code {
  font-family: var(--font-mono);
  font-size: 11.5px;
  background: var(--accent-soft);
  color: var(--accent);
  padding: 1px 5px;
  border-radius: 5px;
}
.login-back { margin-top: 18px; text-align: center; font-size: 13px; color: var(--text-dim); }
</style>
