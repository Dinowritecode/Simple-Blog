<script setup>
// 访客注册页：邮箱 + 密码（暂未开通邮箱验证，注册即自动登录）
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'
import { useUi } from '../composables/useUi'

const route = useRoute()
const router = useRouter()
const { register } = useAuth()
const { toast } = useToast()
const { openProfileEditor } = useUi()

const email = ref('')
const password = ref('')
const password2 = ref('')
const submitting = ref(false)
const error = ref('')

async function submit() {
  if (!email.value || !password.value) {
    error.value = '请填写邮箱和密码'
    return
  }
  if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email.value.trim())) {
    error.value = '邮箱格式不正确'
    return
  }
  if (password.value.length < 6) {
    error.value = '密码至少需要 6 位'
    return
  }
  if (password.value !== password2.value) {
    error.value = '两次输入的密码不一致'
    return
  }
  submitting.value = true
  error.value = ''
  try {
    await register(email.value.trim(), password.value)
    // 注册成功：顶部小提示，引导去设置头像（可裁剪，≤5MB）
    toast.success('注册成功，欢迎加入！', {
      duration: 8000,
      actionLabel: '去上传头像',
      onAction: openProfileEditor
    })
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
  <div class="register-page">
    <div class="register-card">
      <router-link class="register-brand" to="/"><span class="accent">凛冬</span>RinEyce</router-link>
      <h1>注 册</h1>
      <p class="register-sub">注册一个账号即可评论互动（暂未开通邮箱验证，注册即用）</p>

      <form @submit.prevent="submit">
        <label class="field">
          <span>邮箱</span>
          <input v-model="email" type="email" autocomplete="email" placeholder="you@example.com" />
        </label>
        <label class="field">
          <span>设置密码（至少 6 位）</span>
          <input v-model="password" type="password" autocomplete="new-password" placeholder="••••••" />
        </label>
        <label class="field">
          <span>确认密码</span>
          <input v-model="password2" type="password" autocomplete="new-password" placeholder="••••••" />
        </label>
        <p v-if="error" class="register-error">{{ error }}</p>
        <button class="btn btn-primary register-btn" type="submit" :disabled="submitting">
          {{ submitting ? '注册中…' : '注 册' }}
        </button>
      </form>

      <p class="register-switch">
        已有账号？
        <router-link :to="{ path: '/login', query: route.query }">去登录</router-link>
      </p>
      <router-link class="register-back" to="/">← 返回首页</router-link>
    </div>
  </div>
</template>

<style scoped>
.register-page {
  min-height: 100svh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--bg);
}
.register-card {
  width: 100%;
  max-width: 380px;
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 16px;
  box-shadow: var(--shadow);
  padding: 36px;
}
.register-brand { font-weight: 700; font-size: 16px; color: var(--text); display: inline-block; margin-bottom: 22px; }
.register-brand:hover { text-decoration: none; }
.register-card h1 { font-size: 1.4rem; margin-bottom: 4px; }
.register-sub { color: var(--text-dim); font-size: 13.5px; margin-bottom: 22px; line-height: 1.6; }

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

.register-error { color: #e5484d; font-size: 13.5px; margin: 4px 0 10px; }
.register-btn { width: 100%; margin-top: 6px; }
.register-btn:disabled { opacity: 0.6; cursor: not-allowed; }

.register-switch { margin-top: 18px; text-align: center; font-size: 13.5px; color: var(--text-dim); }
.register-switch a { font-weight: 600; }
.register-back { display: block; margin-top: 12px; text-align: center; font-size: 13px; color: var(--text-dim); }
</style>
