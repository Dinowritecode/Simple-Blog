<script setup>
// ============================================================
// AuthView.vue —— 登录 / 注册同页（左右分栏 + 横向滑动切换）
//
//   /login    → 右半屏显示登录表单
//   /register → 右半屏显示注册表单
//
// 两个路由共用这一个组件，所以来回切换时组件**不会重新挂载**，
// 由 CSS transform 做「整块左滑 / 右滑」的过渡：
//   track 宽 200%，两个 panel 各占 50%；
//   注册时 track 平移 -50%（= 一屏宽），登录面板滑出、注册面板滑入。
//
// 左半屏是背景图区（后台可换），没配图就用主题色渐变底板。
// ============================================================
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'
import { useSite } from '../composables/useSite'
import { useUi } from '../composables/useUi'

const route = useRoute()
const router = useRouter()
const { login, register } = useAuth()
const { toast } = useToast()
const { loginBg } = useSite()
const { openProfileEditor } = useUi()

// ---------- 当前是哪一面 ----------
const isRegister = computed(() => route.name === 'register')

// ---------- 登录 ----------
const account = ref('')
const password = ref('')
const showPwd = ref(false)
const remember = ref(false)
const loginBusy = ref(false)
const loginError = ref('')

const REMEMBER_KEY = 'spblog_remember_account'

// ---------- 注册 ----------
const email = ref('')
const regPwd = ref('')
const regPwd2 = ref('')
const regBusy = ref(false)
const regError = ref('')

onMounted(() => {
  // 「记住 30 天」= 记住账号（邮箱），密码永远不落本地
  const saved = localStorage.getItem(REMEMBER_KEY)
  if (saved) {
    account.value = saved
    remember.value = true
  }
})

// 左侧：有图用图，没图用主题色底板
const leftStyle = computed(() => (loginBg.value ? { backgroundImage: `url(${loginBg.value})` } : {}))

// 切换面板时保留 ?next=
const nextQuery = computed(() =>
  typeof route.query.next === 'string' && route.query.next.startsWith('/')
    ? { next: route.query.next }
    : {}
)

function goRegister() {
  router.push({ path: '/register', query: nextQuery.value })
}
function goLogin() {
  router.push({ path: '/login', query: nextQuery.value })
}

// 左上角：回退到上一页（直接打开本页、没有历史时回首页）
function goBack() {
  if (window.history.length > 1) router.back()
  else router.push('/')
}

function forgot() {
  loginError.value = '暂不支持自助找回密码，请通过「联系」里的方式找站长重置'
}

async function submitLogin() {
  if (!account.value || !password.value) {
    loginError.value = '请输入邮箱（或用户名）和密码'
    return
  }
  loginBusy.value = true
  loginError.value = ''
  try {
    const data = await login(account.value.trim(), password.value)
    if (remember.value) localStorage.setItem(REMEMBER_KEY, account.value.trim())
    else localStorage.removeItem(REMEMBER_KEY)
    toast.success(`欢迎回来，${data.user.nickname || data.user.email.split('@')[0]}！`)
    router.push(typeof route.query.next === 'string' && route.query.next.startsWith('/') ? route.query.next : '/')
  } catch (e) {
    loginError.value = e.message
  } finally {
    loginBusy.value = false
  }
}

async function submitRegister() {
  if (!email.value || !regPwd.value) {
    regError.value = '请填写邮箱和密码'
    return
  }
  if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email.value.trim())) {
    regError.value = '邮箱格式不正确'
    return
  }
  if (regPwd.value.length < 6) {
    regError.value = '密码至少需要 6 位'
    return
  }
  if (regPwd.value !== regPwd2.value) {
    regError.value = '两次输入的密码不一致'
    return
  }
  regBusy.value = true
  regError.value = ''
  try {
    await register(email.value.trim(), regPwd.value)
    toast.success('注册成功，欢迎加入！', {
      duration: 8000,
      actionLabel: '去上传头像',
      onAction: openProfileEditor
    })
    router.push(typeof route.query.next === 'string' && route.query.next.startsWith('/') ? route.query.next : '/')
  } catch (e) {
    regError.value = e.message
  } finally {
    regBusy.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <!-- 左：图片区（没有背景图时用主题色底板） -->
    <aside class="auth-left" :class="{ 'has-image': !!loginBg }" :style="leftStyle">
      <button class="back-btn" type="button" @click="goBack">
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M15 18l-6-6 6-6" />
        </svg>
        <span>返回</span>
      </button>
    </aside>

    <!-- 右：滑动区（两屏宽，平移切换） -->
    <main class="auth-right">
      <div class="auth-viewport">
        <div class="auth-track" :class="{ 'show-register': isRegister }">
          <!-- ① 登录 -->
          <section class="auth-panel">
            <div class="auth-form">
              <div class="brand-spark" aria-hidden="true">
                <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round">
                  <path d="M12 3l1.6 5.4L19 10l-5.4 1.6L12 17l-1.6-5.4L5 10l5.4-1.6z" />
                </svg>
              </div>
              <h1>欢迎回来！</h1>
              <p class="auth-sub">请填写您的详细信息</p>

              <form @submit.prevent="submitLogin">
                <label class="field">
                  <span>电子邮件</span>
                  <input v-model="account" type="text" autocomplete="username" placeholder="you@example.com" />
                </label>

                <label class="field pwd-field">
                  <span>密码</span>
                  <input
                    v-model="password"
                    :type="showPwd ? 'text' : 'password'"
                    autocomplete="current-password"
                    placeholder="••••••••"
                  />
                  <button class="pwd-eye" type="button" :aria-label="showPwd ? '隐藏密码' : '显示密码'" @click="showPwd = !showPwd">
                    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
                      <path d="M2 12s3.6-6.5 10-6.5S22 12 22 12s-3.6 6.5-10 6.5S2 12 2 12z" />
                      <circle cx="12" cy="12" r="2.6" />
                      <path v-if="showPwd" d="M4 20L20 4" />
                    </svg>
                  </button>
                </label>

                <div class="auth-aux">
                  <label class="remember">
                    <input v-model="remember" type="checkbox" />
                    <span>记住30天</span>
                  </label>
                  <button class="link-btn" type="button" @click="forgot">忘记密码？</button>
                </div>

                <p v-if="loginError" class="auth-error">{{ loginError }}</p>

                <button class="auth-btn" type="submit" :disabled="loginBusy">
                  {{ loginBusy ? '登录中…' : '登录' }}
                </button>
              </form>

              <p class="auth-switch">
                还没有账号？
                <button class="link-btn strong" type="button" @click="goRegister">立即注册</button>
              </p>
            </div>
          </section>

          <!-- ② 注册 -->
          <section class="auth-panel">
            <div class="auth-form">
              <div class="brand-spark" aria-hidden="true">
                <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round">
                  <path d="M12 3l1.6 5.4L19 10l-5.4 1.6L12 17l-1.6-5.4L5 10l5.4-1.6z" />
                </svg>
              </div>
              <h1>创建账号</h1>
              <p class="auth-sub">注册后即可评论互动（暂未开通邮箱验证）</p>

              <form @submit.prevent="submitRegister">
                <label class="field">
                  <span>电子邮件</span>
                  <input v-model="email" type="email" autocomplete="email" placeholder="you@example.com" />
                </label>

                <label class="field">
                  <span>设置密码（至少 6 位）</span>
                  <input v-model="regPwd" type="password" autocomplete="new-password" placeholder="••••••••" />
                </label>

                <label class="field">
                  <span>确认密码</span>
                  <input v-model="regPwd2" type="password" autocomplete="new-password" placeholder="••••••••" />
                </label>

                <p v-if="regError" class="auth-error">{{ regError }}</p>

                <button class="auth-btn" type="submit" :disabled="regBusy">
                  {{ regBusy ? '注册中…' : '注册' }}
                </button>
              </form>

              <p class="auth-switch">
                已经有账号了？
                <button class="link-btn strong" type="button" @click="goLogin">立即登录</button>
              </p>
            </div>
          </section>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100svh;
  display: grid;
  grid-template-columns: 1fr 1fr;
  background: var(--bg);
}

/* ---------- 左：图片区 ---------- */
.auth-left {
  position: relative;
  /* 没配背景图时的底板：主题色渐层，避免一大片死色 */
  background:
    radial-gradient(circle at 30% 20%, color-mix(in srgb, #fff 45%, transparent), transparent 60%),
    linear-gradient(150deg, color-mix(in srgb, var(--accent) 88%, #fff) 0%, var(--accent) 55%, color-mix(in srgb, var(--accent) 65%, #000) 100%);
  background-size: cover;
  background-position: center;
  display: flex;
  align-items: flex-start;
  padding: 24px;
}
.auth-left.has-image::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.22), transparent 32%);
  pointer-events: none;
}

.back-btn {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px 8px 10px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.42);
  background: rgba(255, 255, 255, 0.18);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  color: #fff;
  font-size: 13.5px;
  cursor: pointer;
  transition: background 0.2s ease, border-color 0.2s ease;
}
.back-btn:hover {
  background: rgba(255, 255, 255, 0.3);
  border-color: rgba(255, 255, 255, 0.7);
}

/* ---------- 右：滑动区 ---------- */
.auth-right {
  background: var(--bg);
  display: flex;
  align-items: stretch;
  overflow: hidden;
}
.auth-viewport {
  flex: 1;
  overflow: hidden;
}
.auth-track {
  display: flex;
  width: 200%;
  height: 100%;
  /* 滑动过渡：left 出去的同时 right 进来 */
  transition: transform 0.52s cubic-bezier(0.65, 0, 0.35, 1);
  transform: translateX(0);
  will-change: transform;
}
.auth-track.show-register {
  transform: translateX(-50%);
}
.auth-panel {
  flex: 0 0 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 24px;
}

.auth-form {
  width: min(400px, 100%);
  text-align: center;
}
.brand-spark {
  color: var(--text);
  display: flex;
  justify-content: center;
  margin-bottom: 14px;
}
.auth-form h1 {
  font-size: 1.6rem;
  font-weight: 700;
  color: var(--accent);
  margin-bottom: 6px;
}
.auth-sub {
  color: var(--text-dim);
  font-size: 13.5px;
  margin-bottom: 30px;
}

/* ---------- 输入框：只有一条底部横线（去圆角） ---------- */
.field {
  display: block;
  text-align: left;
  margin-bottom: 22px;
  position: relative;
}
.field span {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 8px;
  transition: color 0.18s ease;
}
.field input {
  width: 100%;
  border: 0;
  border-bottom: 1px solid var(--border);
  /* 全局 input 有圆角，这里必须显式清零，否则横线两端是圆的 */
  border-radius: 0;
  box-shadow: none;
  background: transparent;
  color: var(--text);
  font-size: 15px;
  padding: 8px 0 10px;
  outline: none;
  transition: border-color 0.18s ease;
}
.field input::placeholder {
  color: color-mix(in srgb, var(--text-dim) 62%, transparent);
}
/* 聚焦：文字标签 + 下划线一起转主题色 */
.field:focus-within span {
  color: var(--accent);
}
.field:focus-within input {
  border-bottom-color: var(--accent);
}

.pwd-field input {
  padding-right: 34px;
}
.pwd-eye {
  position: absolute;
  right: 0;
  bottom: 8px;
  border: 0;
  background: transparent;
  color: var(--text-dim);
  cursor: pointer;
  padding: 4px;
  display: flex;
}
.pwd-eye:hover {
  color: var(--accent);
}

/* ---------- 记住 30 天：自绘勾选框（勾是白色） ---------- */
.auth-aux {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 2px 0 20px;
  font-size: 13px;
}
.remember {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--text-dim);
  cursor: pointer;
  user-select: none;
}
.remember input {
  appearance: none;
  -webkit-appearance: none;
  width: 16px;
  height: 16px;
  margin: 0;
  flex: none;
  border: 1.5px solid color-mix(in srgb, var(--text) 28%, transparent);
  border-radius: 4px;
  background: transparent;
  cursor: pointer;
  position: relative;
  transition: background 0.15s ease, border-color 0.15s ease;
}
.remember input:checked {
  background: var(--accent);
  border-color: var(--accent);
}
/* 勾：两条白边构成 ✓ */
.remember input:checked::after {
  content: '';
  position: absolute;
  left: 50%;
  top: 50%;
  width: 8px;
  height: 4.5px;
  border-left: 2px solid #fff;
  border-bottom: 2px solid #fff;
  transform: translate(-50%, -62%) rotate(-45deg);
  border-radius: 0.5px;
}

.link-btn {
  border: 0;
  background: transparent;
  color: var(--accent);
  font-size: 13px;
  cursor: pointer;
  padding: 0;
}
.link-btn:hover {
  text-decoration: underline;
}
.link-btn.strong {
  font-weight: 600;
}

.auth-error {
  color: #e5484d;
  font-size: 13px;
  text-align: left;
  margin: -8px 0 14px;
}

.auth-btn {
  width: 100%;
  padding: 13px 0;
  border: 0;
  border-radius: 999px;
  background: var(--accent);
  color: #fff;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  transition: filter 0.18s ease, transform 0.18s ease;
}
.auth-btn:hover:not(:disabled) {
  filter: brightness(1.06);
}
.auth-btn:active:not(:disabled) {
  transform: translateY(1px);
}
.auth-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.auth-switch {
  margin-top: 26px;
  font-size: 13px;
  color: var(--text-dim);
}

/* ---------- 小屏：只留表单 ---------- */
@media (max-width: 860px) {
  .auth-page {
    grid-template-columns: 1fr;
  }
  .auth-left {
    display: none;
  }
  .auth-panel {
    padding: 32px 20px;
  }
}
</style>
