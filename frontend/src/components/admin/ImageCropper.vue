<script setup>
// ============================================================
// ImageCropper.vue —— 通用「选图 → 裁剪 → 输出」弹窗
//
// 为什么要裁剪：背景图/封面在页面里是固定比例的容器 + object-fit: cover，
// 用户上传什么比例都会被裁，但**裁哪里**由浏览器决定，常常把头切掉。
// 这里让用户自己决定可视区域，并且裁剪框比例直接用「页面上的实际显示比例」，
// 所见即所得。
//
// 用法：
//   <ImageCropper :open="cropOpen" :file="pendingFile" :aspect="ratio" title="裁剪背景图"
//                 :out-width="1920" @done="onCropped" @close="cropOpen = false" />
//
// 交互：拖动移动图片，滚轮 / 滑块缩放，裁剪框外的部分会被裁掉。
// ============================================================
import { computed, onBeforeUnmount, ref, watch } from 'vue'

const props = defineProps({
  open: { type: Boolean, default: false },
  file: { type: Object, default: null },
  // 裁剪框宽高比（页面上实际的显示比例）
  aspect: { type: Number, default: 16 / 9 },
  // 输出图片的最大宽度（px），高度按比例算
  outWidth: { type: Number, default: 1920 },
  title: { type: String, default: '裁剪图片' },
  note: { type: String, default: '' }
})
const emit = defineEmits(['done', 'close'])

const rawImg = ref(null)
const srcUrl = ref('')
const zoom = ref(1)
const ox = ref(0)
const oy = ref(0)
const dragging = ref(false)
const dragFrom = ref({ x: 0, y: 0, ox: 0, oy: 0 })
const processing = ref(false)
const error = ref('')

// ---------- 裁剪框尺寸：按视口自适应，但保持目标比例 ----------
const stage = ref({ w: 640, h: 360 })
function computeStage() {
  const maxW = Math.min(window.innerWidth * 0.86, 1100)
  const maxH = window.innerHeight * 0.60
  let w = maxW
  let h = w / props.aspect
  if (h > maxH) {
    h = maxH
    w = h * props.aspect
  }
  stage.value = { w: Math.round(w), h: Math.round(h) }
}

const displayW = computed(() =>
  rawImg.value ? rawImg.value.naturalWidth * baseScale.value * zoom.value : 0
)
const displayH = computed(() =>
  rawImg.value ? rawImg.value.naturalHeight * baseScale.value * zoom.value : 0
)
// 基础缩放：先让图片铺满裁剪框（cover），此时 zoom = 1
const baseScale = computed(() => {
  if (!rawImg.value) return 1
  const { naturalWidth: w, naturalHeight: h } = rawImg.value
  return Math.max(stage.value.w / w, stage.value.h / h)
})

function clampOffset() {
  ox.value = Math.min(0, Math.max(ox.value, stage.value.w - displayW.value))
  oy.value = Math.min(0, Math.max(oy.value, stage.value.h - displayH.value))
}

function load(file) {
  error.value = ''
  rawImg.value = null
  zoom.value = 1
  if (srcUrl.value) URL.revokeObjectURL(srcUrl.value)
  srcUrl.value = ''
  if (!file) return
  if (!/^image\//.test(file.type)) {
    error.value = '请选择图片文件'
    return
  }
  computeStage()
  const img = new Image()
  img.onload = () => {
    rawImg.value = img
    // 居中
    requestAnimationFrame(() => {
      ox.value = (stage.value.w - displayW.value) / 2
      oy.value = (stage.value.h - displayH.value) / 2
      clampOffset()
    })
  }
  img.onerror = () => { error.value = '图片加载失败' }
  srcUrl.value = URL.createObjectURL(file)
  img.src = srcUrl.value
}

watch(() => props.file, (f) => { if (props.open) load(f) })
watch(
  () => props.open,
  (v) => {
    if (v) {
      computeStage()
      load(props.file)
    } else {
      rawImg.value = null
      if (srcUrl.value) URL.revokeObjectURL(srcUrl.value)
      srcUrl.value = ''
    }
  }
)

// ---------- 交互 ----------
function onZoom() { clampOffset() }

function onDown(ev) {
  if (!rawImg.value) return
  dragging.value = true
  dragFrom.value = { x: ev.clientX, y: ev.clientY, ox: ox.value, oy: oy.value }
  ev.currentTarget.setPointerCapture?.(ev.pointerId)
}
function onMove(ev) {
  if (!dragging.value) return
  ox.value = dragFrom.value.ox + (ev.clientX - dragFrom.value.x)
  oy.value = dragFrom.value.oy + (ev.clientY - dragFrom.value.y)
  clampOffset()
}
function onUp() { dragging.value = false }

function onWheel(ev) {
  if (!rawImg.value) return
  ev.preventDefault()
  const next = zoom.value * (ev.deltaY < 0 ? 1.08 : 0.92)
  zoom.value = Math.min(4, Math.max(1, Number(next.toFixed(3))))
  clampOffset()
}

// ---------- 输出 ----------
const outH = computed(() => Math.round(props.outWidth / props.aspect))

async function confirm() {
  const img = rawImg.value
  if (!img || processing.value) return
  processing.value = true
  try {
    const s = baseScale.value * zoom.value
    const sx = -ox.value / s
    const sy = -oy.value / s
    const sw = stage.value.w / s
    const sh = stage.value.h / s
    const canvas = document.createElement('canvas')
    canvas.width = props.outWidth
    canvas.height = outH.value
    const ctx = canvas.getContext('2d')
    ctx.imageSmoothingQuality = 'high'
    ctx.fillStyle = '#fff'
    ctx.fillRect(0, 0, canvas.width, canvas.height)
    ctx.drawImage(img, sx, sy, sw, sh, 0, 0, canvas.width, canvas.height)
    const blob = await new Promise((res) => canvas.toBlob(res, 'image/jpeg', 0.92))
    if (!blob) throw new Error('图片处理失败')
    if (blob.size > 10 * 1024 * 1024) throw new Error('裁剪后仍超过 10MB，请换一张图')
    const name = (props.file?.name || 'image').replace(/\.[^.]+$/, '') + '-crop.jpg'
    emit('done', new File([blob], name, { type: 'image/jpeg' }))
  } catch (e) {
    error.value = e.message || '裁剪失败'
  } finally {
    processing.value = false
  }
}

function close() {
  if (processing.value) return
  emit('close')
}

function onKey(e) {
  if (!props.open) return
  if (e.key === 'Escape') close()
}
watch(() => props.open, (v) => {
  if (v) {
    window.addEventListener('keydown', onKey)
    window.addEventListener('resize', computeStage)
  } else {
    window.removeEventListener('keydown', onKey)
    window.removeEventListener('resize', computeStage)
  }
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('resize', computeStage)
  if (srcUrl.value) URL.revokeObjectURL(srcUrl.value)
})
</script>

<template>
  <Teleport to="body">
    <transition name="fade">
      <div v-if="open" class="crop-mask" @click.self="close">
        <div class="crop-card">
          <div class="crop-head">
            <h3>{{ title }}</h3>
            <button class="crop-x" aria-label="关闭" @click="close">✕</button>
          </div>

          <p class="crop-tip">
            拖动图片选择位置，滚轮或滑块缩放。
            <span class="crop-ratio">{{ aspect.toFixed(2) }} : 1</span>
            · 输出 {{ outWidth }} × {{ outH }} px
          </p>

          <!-- 裁剪区：框内即最终保留的区域 -->
          <div
            class="crop-stage"
            :style="{ width: stage.w + 'px', height: stage.h + 'px' }"
            @pointerdown="onDown"
            @pointermove="onMove"
            @pointerup="onUp"
            @pointercancel="onUp"
            @wheel="onWheel"
          >
            <img
              v-if="srcUrl"
              :src="srcUrl"
              alt="待裁剪"
              class="crop-img"
              :class="{ dragging }"
              :style="{
                width: displayW + 'px',
                height: displayH + 'px',
                transform: `translate(${ox}px, ${oy}px)`
              }"
              draggable="false"
            />
            <div v-else class="crop-loading">图片加载中…</div>
            <!-- 三分线，帮助构图 -->
            <div class="crop-grid">
              <i></i><i></i><i></i><i></i>
            </div>
          </div>

          <div class="crop-zoom">
            <span>缩放</span>
            <input v-model.number="zoom" type="range" min="1" max="4" step="0.01" @input="onZoom" />
            <span class="crop-zoom-val">{{ zoom.toFixed(2) }}×</span>
          </div>

          <p v-if="error" class="crop-err">{{ error }}</p>
          <p v-else-if="note" class="crop-note">{{ note }}</p>

          <div class="crop-actions">
            <button class="btn-sm ghost" :disabled="processing" @click="close">取消</button>
            <button class="btn-sm" :disabled="processing || !rawImg" @click="confirm">
              {{ processing ? '处理中…' : '确认裁剪并上传' }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<style scoped>
.crop-mask {
  position: fixed;
  inset: 0;
  z-index: 300;
  background: rgba(8, 12, 20, 0.66);
  backdrop-filter: blur(6px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}
.crop-card {
  /* 用实底卡片色：--card-bg 可能带透明，压在深色蒙版上会发灰 */
  background: var(--card, #fff);
  color: var(--text);
  border: 1px solid var(--glass-border);
  border-radius: 16px;
  padding: 20px 22px 18px;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.28);
  max-width: 96vw;
}
.crop-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}
.crop-head h3 { font-size: 1rem; font-weight: 700; margin: 0; color: var(--text); }
.crop-x {
  border: 0;
  background: transparent;
  color: var(--text-dim);
  font-size: 15px;
  cursor: pointer;
  padding: 4px 6px;
  border-radius: 6px;
}
.crop-x:hover { background: var(--bg-alt); color: var(--text); }

.crop-tip { font-size: 12px; color: var(--text-dim); margin: 0 0 12px; line-height: 1.6; }
.crop-ratio {
  display: inline-block;
  padding: 1px 6px;
  margin: 0 2px;
  border-radius: 4px;
  background: var(--bg-alt);
  color: var(--text);
  font-variant-numeric: tabular-nums;
}

.crop-stage {
  position: relative;
  overflow: hidden;
  border-radius: 10px;
  background: #111;
  cursor: grab;
  touch-action: none;
  user-select: none;
  max-width: 86vw;
}
.crop-stage:active { cursor: grabbing; }
.crop-img {
  position: absolute;
  top: 0;
  left: 0;
  max-width: none;
  pointer-events: none;
  will-change: transform;
}
.crop-img.dragging { transition: none; }
.crop-loading {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  color: rgba(255, 255, 255, 0.7);
  font-size: 13px;
}
/* 三分线 */
.crop-grid { position: absolute; inset: 0; pointer-events: none; }
.crop-grid i {
  position: absolute;
  background: rgba(255, 255, 255, 0.28);
}
.crop-grid i:nth-child(1) { left: 33.333%; top: 0; bottom: 0; width: 1px; }
.crop-grid i:nth-child(2) { left: 66.666%; top: 0; bottom: 0; width: 1px; }
.crop-grid i:nth-child(3) { top: 33.333%; left: 0; right: 0; height: 1px; }
.crop-grid i:nth-child(4) { top: 66.666%; left: 0; right: 0; height: 1px; }

.crop-zoom {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 12px 0 4px;
  font-size: 12px;
  color: var(--text-dim);
}
.crop-zoom input[type='range'] { flex: 1; accent-color: var(--accent); }
.crop-zoom-val { font-variant-numeric: tabular-nums; min-width: 44px; text-align: right; }

.crop-err { color: #e5484d; font-size: 12px; margin: 6px 0 0; }
.crop-note { color: var(--text-dim); font-size: 12px; margin: 6px 0 0; line-height: 1.6; }

.crop-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 14px;
}

.fade-enter-active, .fade-leave-active { transition: opacity 0.18s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
