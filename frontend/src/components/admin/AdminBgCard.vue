<script setup>
// ============================================================
// AdminBgCard.vue —— 后台「一张背景图」的通用设置卡片
//
// 首页背景图与文章页背景图除了标题、说明和保存接口以外完全一样，
// 所以抽成一个组件复用：预览 → 填 URL / 上传本地图片 → 清除 → 应用。
//
// 父组件通过 saveFn 把「保存到哪个设置项」传进来（api.saveHomeBg / api.savePostBg），
// 保存成功后 emit('saved', 新地址)，父组件据此同步更新 useSite 里的值，
// 这样在同一浏览器里切到前台立刻就能看到新背景，不用刷新。
// ============================================================

import { ref, watch } from 'vue'
import { api } from '../../api'
import { useToast } from '../../composables/useToast'

const props = defineProps({
  title: { type: String, required: true },
  desc: { type: String, default: '' },
  current: { type: String, default: '' }, // 已保存的地址
  saveFn: { type: Function, required: true }, // (url) => Promise<{...}>
  emptyText: { type: String, default: '未设置背景（使用默认纯色网格背景）' },
  placeholder: { type: String, default: 'https://… 或 /uploads/xxx.jpg' }
})
const emit = defineEmits(['saved'])

const { toast } = useToast()

const input = ref(props.current)
const saving = ref(false)
const uploading = ref(false)
const fileInput = ref(null)

// 外部（例如重新拉设置）改了 current，输入框跟着走
watch(
  () => props.current,
  (v) => {
    input.value = v || ''
  }
)

function pickFile() {
  fileInput.value && fileInput.value.click()
}

async function onFile(e) {
  const f = e.target.files && e.target.files[0]
  e.target.value = ''
  if (!f) return
  if (!/^image\//.test(f.type)) return toast.error('请选择图片文件')
  uploading.value = true
  try {
    const d = await api.uploadImage(f) // 上传后得到站内 /uploads/xxx 地址
    input.value = d.url
  } catch (err) {
    toast.error(err.message)
  } finally {
    uploading.value = false
  }
}

async function save() {
  const v = input.value.trim()
  if (v && !/^(https?:\/\/|\/)/.test(v)) {
    return toast.error('仅支持 http(s):// 或站内 /uploads 路径')
  }
  saving.value = true
  try {
    const d = await props.saveFn(v)
    const saved = (d && (d.homeBg ?? d.postBg)) ?? v
    input.value = saved || ''
    emit('saved', saved || '')
    toast.success(saved ? '背景图已应用' : '已清除背景图')
  } catch (err) {
    toast.error(err.message)
  } finally {
    saving.value = false
  }
}

function clear() {
  input.value = ''
}
</script>

<template>
  <div class="settings-card">
    <h2>{{ title }}</h2>
    <p class="settings-desc" v-html="desc"></p>

    <!-- 预览 -->
    <div class="bg-preview" :class="{ 'bg-empty': !input }">
      <div v-if="input" class="bg-preview-img" :style="{ backgroundImage: `url(${input})` }"></div>
      <div v-else class="bg-preview-empty">
        <span class="bg-empty-icon">🖼️</span>
        <span>{{ emptyText }}</span>
      </div>
      <div class="bg-preview-tag">{{ input ? '预览效果' : '预览区' }}</div>
    </div>

    <!-- 输入 / 上传 -->
    <div class="bg-form">
      <label class="m-field bg-url-field">
        <span>背景图 URL</span>
        <input v-model="input" type="text" :placeholder="placeholder" />
      </label>
      <div class="bg-buttons">
        <input ref="fileInput" type="file" accept="image/*" hidden @change="onFile" />
        <button class="btn-sm ghost" :disabled="uploading" @click="pickFile">
          {{ uploading ? '上传中…' : '📁 上传图片' }}
        </button>
        <button class="btn-sm ghost" :disabled="!input" @click="clear">清除</button>
        <button class="btn-sm" :disabled="saving || input === current" @click="save">
          {{ saving ? '保存中…' : '💾 应用背景' }}
        </button>
      </div>
      <p class="settings-hint">
        {{ current ? `当前已应用：${current}` : '当前未设置背景图' }}
      </p>
    </div>
  </div>
</template>

<style scoped>
/* ---------- 卡片外壳 ----------
   .settings-card / .settings-desc / .settings-hint 在 AdminSettingsView 里是
   scoped 样式，子组件拿不到，所以这里照抄一份（数值与那边保持一致）。
   .m-field 与 .btn-sm 是全局的，不用重复定义。 */
.settings-card {
  max-width: 680px;
  background: var(--glass-solid-bg);
  backdrop-filter: var(--glass-solid-blur);
  -webkit-backdrop-filter: var(--glass-solid-blur);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 28px;
}
.settings-card h2 { font-size: 1.15rem; margin-bottom: 8px; }
.settings-desc { color: var(--text-dim); font-size: 14px; line-height: 1.8; margin-bottom: 20px; }
.settings-hint {
  margin-top: 14px;
  font-size: 12.5px;
  color: var(--text-dim);
  font-family: var(--font-mono);
  word-break: break-all;
}

/* ---------- 预览区 ---------- */
.bg-preview {
  position: relative;
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid var(--border);
  /* 16:6 左右的比例，接近首屏裁切后的观感 */
  aspect-ratio: 16 / 6;
  background: var(--bg-alt);
  margin-bottom: 16px;
}
.bg-preview-img {
  position: absolute;
  inset: 0;
  background-size: cover;
  background-position: center;
  background-repeat: no-repeat;
}
.bg-preview-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--text-dim);
  font-size: 13px;
}
.bg-empty-icon { font-size: 26px; opacity: 0.6; }
.bg-preview-tag {
  position: absolute;
  right: 10px;
  bottom: 10px;
  font-size: 11px;
  padding: 3px 8px;
  border-radius: 6px;
  color: #fff;
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(4px);
}

/* ---------- 表单 ---------- */
.bg-form { display: flex; flex-direction: column; gap: 12px; }
.bg-url-field { margin-bottom: 0; }
.bg-buttons { display: flex; flex-wrap: wrap; gap: 8px; }
</style>
