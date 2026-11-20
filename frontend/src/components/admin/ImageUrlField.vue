<script setup>
// ============================================================
// ImageUrlField.vue —— 「图片地址 + 预览 + 上传裁剪」的通用输入框
//
// 为什么不再直接铺一张大预览：
//   封面 / 背景这种图往往是通栏大图，内联预览会占掉半屏，编辑区被挤得没法写。
//   现在只显示地址，旁边放一个 👁 图标，点开才在**小窗**里看效果；
//   上传后同样是这个小窗式的裁剪器（和头像裁剪共用一个 ImageCropper）。
//
// 用法：
//   <ImageUrlField v-model="form.cover" :aspect="coverAspect" crop-title="裁剪文章封面" />
// ============================================================
import { ref } from 'vue'
import { api } from '../../api'
import ImageCropper from './ImageCropper.vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: '/uploads/xxx.jpg 或 https://…' },
  // 裁剪比例与输出宽度
  aspect: { type: Number, default: 16 / 9 },
  outWidth: { type: Number, default: 1920 },
  cropTitle: { type: String, default: '裁剪图片' },
  cropNote: { type: String, default: '' }
})
const emit = defineEmits(['update:modelValue'])

const fileInput = ref(null)
const uploading = ref(false)
const cropOpen = ref(false)
const pendingFile = ref(null)
const previewOpen = ref(false)
const error = ref('')

function pick() {
  fileInput.value?.click()
}

function onFileChange(e) {
  const file = e.target.files?.[0]
  e.target.value = ''
  if (!file) return
  if (!/^image\//.test(file.type)) {
    error.value = '请选择图片文件'
    return
  }
  error.value = ''
  pendingFile.value = file
  cropOpen.value = true
}

async function onCropped(file) {
  cropOpen.value = false
  pendingFile.value = null
  uploading.value = true
  try {
    const res = await api.uploadImage(file)
    emit('update:modelValue', res.url)
  } catch (err) {
    error.value = err.message
  } finally {
    uploading.value = false
  }
}

function clear() {
  emit('update:modelValue', '')
}
</script>

<template>
  <div class="iuf">
    <div class="iuf-row">
      <input
        class="iuf-input"
        :value="modelValue"
        type="text"
        :placeholder="placeholder"
        @input="emit('update:modelValue', $event.target.value)"
      />
      <button
        class="iuf-icon"
        type="button"
        title="预览"
        :disabled="!modelValue"
        @click="previewOpen = true"
      >👁</button>
      <button class="iuf-icon" type="button" title="上传图片" :disabled="uploading" @click="pick">
        {{ uploading ? '…' : '📁' }}
      </button>
      <button v-if="modelValue" class="iuf-icon danger" type="button" title="移除" @click="clear">✕</button>
      <input ref="fileInput" type="file" accept="image/*" hidden @change="onFileChange" />
    </div>
    <p v-if="error" class="iuf-error">{{ error }}</p>

    <!-- 预览小窗：只在这里看大图，不再占编辑区 -->
    <Teleport to="body">
      <transition name="fade">
        <div v-if="previewOpen" class="iuf-mask" @click.self="previewOpen = false">
          <div class="iuf-modal">
            <div class="iuf-modal-head">
              <h3>预览</h3>
              <button class="iuf-x" @click="previewOpen = false">✕</button>
            </div>
            <div class="iuf-preview">
              <img :src="modelValue" alt="预览" />
            </div>
            <p class="iuf-url" :title="modelValue">{{ modelValue }}</p>
          </div>
        </div>
      </transition>
    </Teleport>

    <!-- 裁剪：小窗模式，和头像裁剪同一套交互 -->
    <ImageCropper
      :open="cropOpen"
      :file="pendingFile"
      :aspect="aspect"
      :out-width="outWidth"
      :title="cropTitle"
      :note="cropNote"
      compact
      @done="onCropped"
      @close="cropOpen = false; pendingFile = null"
    />
  </div>
</template>

<style scoped>
.iuf-row { display: flex; align-items: center; gap: 6px; }
.iuf-row input {
  flex: 1;
  min-width: 0;
  padding: 9px 12px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--bg);
  color: var(--text);
  font-size: 13.5px;
  outline: none;
  transition: border-color 0.18s ease;
}
.iuf-row input:focus { border-color: var(--accent); }

.iuf-icon {
  flex: none;
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--bg);
  color: var(--text-dim);
  font-size: 14px;
  cursor: pointer;
  transition: color 0.16s ease, border-color 0.16s ease, background 0.16s ease;
}
.iuf-icon:hover:not(:disabled) { color: var(--accent); border-color: var(--accent); background: var(--accent-soft); }
.iuf-icon:disabled { opacity: 0.45; cursor: not-allowed; }
.iuf-icon.danger:hover:not(:disabled) { color: #e5484d; border-color: #e5484d; background: rgba(229, 72, 77, 0.08); }

.iuf-error { color: #e5484d; font-size: 12px; margin: 6px 0 0; }

/* 预览弹窗 */
.iuf-mask {
  position: fixed;
  inset: 0;
  z-index: 320;
  background: rgba(8, 12, 20, 0.66);
  backdrop-filter: blur(6px);
  display: grid;
  place-items: center;
  padding: 24px;
}
.iuf-modal {
  width: min(720px, 92vw);
  max-height: 86vh;
  overflow: auto;
  background: var(--card);
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  box-shadow: var(--shadow-lg, 0 24px 60px rgba(0, 0, 0, 0.28));
  padding: 14px 16px 16px;
}
.iuf-modal-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.iuf-modal-head h3 { font-size: 14.5px; }
.iuf-x { border: 0; background: transparent; color: var(--text-dim); cursor: pointer; font-size: 15px; padding: 2px 6px; border-radius: 6px; }
.iuf-x:hover { background: var(--bg-alt); color: var(--text); }
.iuf-preview { border-radius: 10px; overflow: hidden; background: var(--bg-alt); }
.iuf-preview img { display: block; width: 100%; height: auto; max-height: 64vh; object-fit: contain; }
.iuf-url { margin: 8px 0 0; font-size: 12px; color: var(--text-dim); word-break: break-all; }

.fade-enter-active, .fade-leave-active { transition: opacity 0.16s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
