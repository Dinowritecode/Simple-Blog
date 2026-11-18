<script setup>
// ============================================================
// AdminAccountView.vue —— 后台「设置」：站长名称 / 邮箱 / 密码
//
// 三块互不影响，各自独立保存：
//   · 名称、邮箱 → PUT /api/user/profile（前端同步更新 useAuth 里的 user）
//   · 密码       → PUT /api/user/password（要验证原密码）
// ============================================================
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'

const { user, displayName, avatarUrl, setUser } = useAuth()
const { toast } = useToast()

const nickname = ref('')
const email = ref('')
const savingName = ref(false)
const savingEmail = ref(false)

const oldPwd = ref('')
const newPwd = ref('')
const newPwd2 = ref('')
const savingPwd = ref(false)
const pwdError = ref('')

onMounted(() => {
  nickname.value = displayName.value || ''
  email.value = user.value?.email === 'admin@localhost' ? '' : user.value?.email || ''
})

async function saveName() {
  const v = nickname.value.trim()
  if (!v) return toast.error('名称不能为空')
  if (v === displayName.value) return toast.info('名称没有变化')
  savingName.value = true
  try {
    const u = await api.updateProfile({ nickname: v })
    setUser({ nickname: u.nickname })
    toast.success('名称已保存')
  } catch (e) {
    toast.error(e.message)
  } finally {
    savingName.value = false
  }
}

async function saveEmail() {
  const v = email.value.trim()
  if (!/.+@.+\..+/.test(v)) return toast.error('邮箱格式不正确')
  savingEmail.value = true
  try {
    const u = await api.updateProfile({ email: v })
    setUser({ email: u.email })
    toast.success('邮箱已保存')
  } catch (e) {
    toast.error(e.message)
  } finally {
    savingEmail.value = false
  }
}

async function savePwd() {
  pwdError.value = ''
  if (!oldPwd.value) return (pwdError.value = '请输入原密码')
  if (newPwd.value.length < 6) return (pwdError.value = '新密码至少 6 位')
  if (newPwd.value !== newPwd2.value) return (pwdError.value = '两次输入的新密码不一致')
  savingPwd.value = true
  try {
    await api.changePassword(oldPwd.value, newPwd.value)
    oldPwd.value = newPwd.value = newPwd2.value = ''
    toast.success('密码已更新')
  } catch (e) {
    pwdError.value = e.message
  } finally {
    savingPwd.value = false
  }
}
</script>

<template>
  <div class="account">
    <header class="acc-head">
      <h1>设置</h1>
      <p>站长资料与登录密码</p>
    </header>

    <!-- 站长名称 -->
    <section class="card">
      <div class="card-head">
        <h2>站长名称</h2>
        <span class="hint">显示在评论区、资料卡和导航栏</span>
      </div>
      <div class="row">
        <span class="avatar" style="--avatar-size: 44px">
          <img v-if="avatarUrl" :src="avatarUrl" alt="" />
        </span>
        <input v-model="nickname" class="inp" type="text" maxlength="30" placeholder="站长名称" />
        <button class="btn" :disabled="savingName" @click="saveName">
          {{ savingName ? '保存中…' : '保存' }}
        </button>
      </div>
    </section>

    <!-- 邮箱 -->
    <section class="card">
      <div class="card-head">
        <h2>邮箱</h2>
        <span class="hint">用于登录（首页 / 后台都可以填邮箱）</span>
      </div>
      <div class="row">
        <input v-model="email" class="inp" type="email" placeholder="you@example.com" />
        <button class="btn" :disabled="savingEmail" @click="saveEmail">
          {{ savingEmail ? '保存中…' : '保存' }}
        </button>
      </div>
    </section>

    <!-- 密码 -->
    <section class="card">
      <div class="card-head">
        <h2>登录密码</h2>
        <span class="hint">至少 6 位，改完立即生效</span>
      </div>
      <div class="col">
        <input v-model="oldPwd" class="inp" type="password" autocomplete="current-password" placeholder="当前密码" />
        <input v-model="newPwd" class="inp" type="password" autocomplete="new-password" placeholder="新密码" />
        <input v-model="newPwd2" class="inp" type="password" autocomplete="new-password" placeholder="再输入一次新密码" />
        <p v-if="pwdError" class="err">{{ pwdError }}</p>
        <button class="btn" :disabled="savingPwd" @click="savePwd">
          {{ savingPwd ? '更新中…' : '更新密码' }}
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.account { max-width: 720px; }
.acc-head { margin-bottom: 22px; }
.acc-head h1 { font-size: 1.45rem; margin-bottom: 4px; }
.acc-head p { color: var(--text-dim); font-size: 13.5px; }

.card {
  background: var(--card);
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  padding: 18px 20px;
  margin-bottom: 16px;
  box-shadow: var(--shadow);
}
.card-head { display: flex; align-items: baseline; gap: 10px; margin-bottom: 14px; flex-wrap: wrap; }
.card-head h2 { font-size: 15px; }
.hint { font-size: 12.5px; color: var(--text-dim); }

.row { display: flex; align-items: center; gap: 12px; }
.col { display: flex; flex-direction: column; gap: 10px; align-items: flex-start; }
.col .inp { width: 100%; }
.col .btn { align-self: flex-start; }

.inp {
  flex: 1;
  min-width: 0;
  padding: 10px 13px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg);
  color: var(--text);
  font-size: 14.5px;
  outline: none;
  transition: border-color 0.18s ease;
}
.inp:focus { border-color: var(--accent); }

.btn {
  flex: none;
  padding: 10px 20px;
  border: 0;
  border-radius: 999px;
  background: var(--accent);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: filter 0.18s ease;
}
.btn:hover:not(:disabled) { filter: brightness(1.06); }
.btn:disabled { opacity: 0.55; cursor: not-allowed; }

.err { color: #e5484d; font-size: 12.5px; margin: 0; }
</style>
