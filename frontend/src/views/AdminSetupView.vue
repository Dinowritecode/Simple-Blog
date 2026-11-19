<script setup>
// 后台「首次初始化」：上传头像（带裁剪）+ 填一个邮箱，完成之后不再出现。
// 不做邮箱验证 —— 只是给站长留一个联系地址，顺便让头像不为空。
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'
import { refreshAdminSetup, resetAdminSetupCache } from '../composables/useAdminSetup'
import ImageCropper from '../components/admin/ImageCropper.vue'

const router = useRouter()
const { user, displayName, avatarUrl, avatarLetter, setUser } = useAuth()
const { toast } = useToast()

const email = ref('')
const uploading = ref(false)
const saving = ref(false)
const error = ref('')

// 裁剪：方形，输出 512×512（头像各处都是圆形裁切，方图最省心）
const cropOpen = ref(false)
const pendingFile = ref(null)
const fileInput = ref(null)

// 头像是可选的：不上传就用站点默认头像，所以这里只校验邮箱
const canSubmit = computed(() => /.+@.+\..+/.test(email.value.trim()) && !saving.value)

onMounted(async () => {
  // 已经初始化过就直接进后台
  const ok = await refreshAdminSetup(true)
  if (ok) {
    router.replace('/admin')
    return
  }
  // 自动建的账号邮箱是 admin@localhost，这种占位值不预填 —— 免得站长不填就点完成
  const cur = user.value?.email || ''
  email.value = /@localhost$/i.test(cur) ? '' : cur
})

function pickFile() {
  fileInput.value && fileInput.value.click()
}
function onFile(e) {
  const f = e.target.files && e.target.files[0]
  e.target.value = ''
  if (!f) return
  if (!/^image\//.test(f.type)) return toast.error('请选择图片文件')
  pendingFile.value = f
  cropOpen.value = true
}

async function onCropped(file) {
  cropOpen.value = false
  pendingFile.value = null
  uploading.value = true
  try {
    const d = await api.uploadAvatar(file)
    setUser({ avatar: d.avatar })
    toast.success('头像已设置')
  } catch (err) {
    toast.error(err.message)
  } finally {
    uploading.value = false
  }
}

async function submit() {
  const mail = email.value.trim()
  if (!/.+@.+\..+/.test(mail)) return (error.value = '邮箱格式不正确')
  saving.value = true
  error.value = ''
  try {
    await api.completeOnboarding(mail)
    // 本地用户信息同步一下邮箱，顶栏 / 资料卡立刻能看到
    setUser({ email: mail })
    resetAdminSetupCache()
    toast.success('初始化完成，欢迎使用后台')
    router.replace('/admin')
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="setup-page">
    <div class="setup-card">
      <div class="setup-spark" aria-hidden="true">
        <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round">
          <path d="M12 3l1.6 5.4L19 10l-5.4 1.6L12 17l-1.6-5.4L5 10l5.4-1.6z" />
        </svg>
      </div>
      <h1>完善基础信息</h1>
      <p class="setup-sub">请完善您的个人信息喵~</p>

      <!-- ① 头像（可选） -->
      <section class="setup-block">
        <div class="block-head">
          <span class="step">1</span>
          <b>上传头像</b>
          <span class="block-hint">可选项</span>
        </div>
        <div class="avatar-row">
          <div class="avatar-preview">
            <img v-if="avatarUrl" :src="avatarUrl" alt="头像" />
            <span v-else class="avatar-empty">{{ avatarLetter }}</span>
          </div>
          <div class="avatar-actions">
            <input ref="fileInput" type="file" accept="image/*" hidden @change="onFile" />
            <button class="btn-outline" type="button" :disabled="uploading" @click="pickFile">
              {{ uploading ? '上传中…' : '更换头像' }}
            </button>
            <p class="avatar-tip">支持png、jpg、jpeg格式</p>
          </div>
        </div>
      </section>

      <!-- ② 邮箱 -->
      <section class="setup-block">
        <div class="block-head">
          <span class="step">2</span>
          <b>设置邮箱</b>
          <span class="block-hint">必填项</span>
        </div>
        <label class="setup-field">
          <input v-model="email" type="email" placeholder="you@example.com" autocomplete="email" />
        </label>
      </section>

      <p v-if="error" class="setup-error">{{ error }}</p>

      <button class="setup-submit" type="button" :disabled="!canSubmit" @click="submit">
        {{ saving ? '保存中…' : '开始使用' }}
      </button>
    </div>

    <ImageCropper
      :open="cropOpen"
      :file="pendingFile"
      :aspect="1"
      :out-width="512"
      title="裁剪头像"
      note="头像在各处按圆形显示，方图最容易对齐。"
      @done="onCropped"
      @close="cropOpen = false; pendingFile = null"
    />
  </div>
</template>

<style scoped>
.setup-page {
  min-height: 100svh;
  display: grid;
  place-items: center;
  padding: 32px 20px;
  background:
    radial-gradient(circle at 20% 0%, color-mix(in srgb, var(--accent) 12%, transparent), transparent 55%),
    var(--bg);
}
.setup-card {
  width: min(520px, 100%);
  background: var(--card);
  border: 1px solid var(--glass-border);
  border-radius: 18px;
  box-shadow: var(--shadow);
  padding: 34px 34px 26px;
  text-align: center;
}
.setup-spark { color: var(--accent); display: flex; justify-content: center; margin-bottom: 10px; }
.setup-card h1 { font-size: 1.35rem; margin-bottom: 6px; }
.setup-sub { color: var(--text-dim); font-size: 13.5px; margin-bottom: 26px; }

.setup-block { text-align: left; margin-bottom: 22px; }
.block-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
.block-head b { font-size: 14px; }
.step {
  width: 18px; height: 18px;
  display: inline-flex; align-items: center; justify-content: center;
  border-radius: 50%;
  background: var(--accent);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
}
.block-hint { color: var(--text-dim); font-size: 12px; }

.avatar-row {
  display: flex;
  align-items: center;
  gap: 28px;              /* 头像与按钮之间的间距（比默认更开一点） */
  margin-left: 8px;       /* 整组略往右，和上面标题的视觉重心对齐 */
}
.avatar-preview {
  width: 72px; height: 72px;
  flex: none;
  border-radius: 50%;
  overflow: hidden;
  display: grid;
  place-items: center;
  background: var(--bg-alt);
  border: 2px solid var(--glass-border);
}
.avatar-preview img { width: 100%; height: 100%; object-fit: cover; }
.avatar-empty { font-size: 26px; font-weight: 700; color: var(--accent); }
.avatar-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-start;
  margin-top: 7px;        /* 按钮和下方说明整体微微下移，重心和头像更协调 */
}
.avatar-tip { color: var(--text-dim); font-size: 12px; margin: 0; }

/* 更换头像：蓝线空心 + 蓝字，悬停才变实心 */
.btn-outline {
  min-width: 92px;                /* 比原来的整行按钮短很多 */
  padding: 7px 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;        /* 文字水平居中 */
  line-height: 1.2;
  border: 1.5px solid var(--accent);
  border-radius: 999px;
  background: transparent;
  color: var(--accent);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease, border-color 0.18s ease;
}
.btn-outline:hover:not(:disabled) {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}
.btn-outline:active:not(:disabled) { transform: translateY(1px); }
.btn-outline:disabled { opacity: 0.55; cursor: not-allowed; }

.setup-field input {
  width: 100%;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg);
  color: var(--text);
  font-size: 15px;
  padding: 11px 14px;
  outline: none;
  transition: border-color 0.18s ease;
}
.setup-field input:focus { border-color: var(--accent); }

.setup-error { color: #e5484d; font-size: 13px; text-align: left; margin: 0 0 12px; }

.setup-submit {
  width: 100%;
  margin-top: 6px;
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
.setup-submit:hover:not(:disabled) { filter: brightness(1.06); }
.setup-submit:active:not(:disabled) { transform: translateY(1px); }
.setup-submit:disabled { opacity: 0.55; cursor: not-allowed; }
</style>
