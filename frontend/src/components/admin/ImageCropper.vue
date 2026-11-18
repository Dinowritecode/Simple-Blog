<script setup>
// ============================================================
// ImageCropper.vue —— 通用「选图 → 框选 → 输出」弹窗
//
// 交互（与常见「拖图片」的裁剪器不同）：
//   · 图片铺满画布后**不动**，由用户拖一个白色方框来选择保留区域
//   · 方框四角是加粗的蓝色手柄，拖动即可改变大小（始终跟随目标比例）
//   · 方框内画一个加粗的白色圆形，预览头像的圆形裁切效果（仅 1:1 时）
//   · 右侧是画布模式的实时预览：框住哪里就放大显示哪里，随选区缩放
//
// 用法：
//   <ImageCropper :open="cropOpen" :file="pendingFile" :aspect="1" :out-width="512"
//                 title="裁剪头像" @done="onCropped" @close="cropOpen = false" />
// ============================================================
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'

const props = defineProps({
  open: { type: Boolean, default: false },
  file: { type: Object, default: null },
  // 选区宽高比（页面上实际的显示比例）
  aspect: { type: Number, default: 16 / 9 },
  // 输出图片的最大宽度（px），高度按比例算
  outWidth: { type: Number, default: 1920 },
  title: { type: String, default: '裁剪图片' },
  note: { type: String, default: '' }
})
const emit = defineEmits(['done', 'close'])

const rawImg = ref(null)
const srcUrl = ref('')
const processing = ref(false)
const error = ref('')
const nat = ref({ w: 0, h: 0 })

const stageEl = ref(null)
const stage = ref({ w: 640, h: 360 })
const sel = ref({ x: 0, y: 0, w: 0, h: 0 })

// 头像是圆的：1:1 时在选区内画一个白色圆环预览
const isCircle = computed(() => Math.abs(props.aspect - 1) < 0.01)

// ---------- 画布：图片按 contain 铺进舞台（不超出、不裁切） ----------
const fit = computed(() => {
  const { w: sw, h: sh } = stage.value
  const { w: iw, h: ih } = nat.value
  if (!iw || !ih) return { x: 0, y: 0, w: sw, h: sh, scale: 1 }
  const scale = Math.min(sw / iw, sh / ih)
  const w = iw * scale
  const h = ih * scale
  return { x: (sw - w) / 2, y: (sh - h) / 2, w, h, scale }
})

const imgStyle = computed(() => ({
  left: fit.value.x + 'px',
  top: fit.value.y + 'px',
  width: fit.value.w + 'px',
  height: fit.value.h + 'px'
}))

const selStyle = computed(() => ({
  left: sel.value.x + 'px',
  top: sel.value.y + 'px',
  width: sel.value.w + 'px',
  height: sel.value.h + 'px'
}))

// ---------- 舞台尺寸：给「图片 + 右侧预览」留出空间 ----------
function computeStage() {
  const maxW = Math.min(window.innerWidth * 0.72, 900)
  const maxH = window.innerHeight * 0.58
  let w = maxW
  let h = w / props.aspect
  if (h > maxH) {
    h = maxH
    w = h * props.aspect
  }
  stage.value = { w: Math.round(w), h: Math.round(h) }
}

// 选区初始化：贴着图片，取能放下的最大比例框的 86%，居中
function resetSelection() {
  const f = fit.value
  const k = 0.86
  let w = Math.min(f.w * k, f.h * k * props.aspect)
  let h = w / props.aspect
  sel.value = {
    x: f.x + (f.w - w) / 2,
    y: f.y + (f.h - h) / 2,
    w,
    h
  }
}

const MIN_SIZE = 48

function clampSelection(s) {
  const f = fit.value
  return {
    x: Math.min(Math.max(s.x, f.x), f.x + f.w - s.w),
    y: Math.min(Math.max(s.y, f.y), f.y + f.h - s.h),
    w: s.w,
    h: s.h
  }
}

// ---------- 拖拽：移动选区 / 拖四角改大小 ----------
let dragMode = null
let dragStart = null
let stageBox = null

function beginDrag(ev, mode) {
  if (!rawImg.value) return
  ev.preventDefault()
  ev.stopPropagation()
  dragMode = mode
  stageBox = stageEl.value.getBoundingClientRect()
  dragStart = { px: ev.clientX, py: ev.clientY, sel: { ...sel.value } }
  window.addEventListener('pointermove', onDrag)
  window.addEventListener('pointerup', endDrag)
  window.addEventListener('pointercancel', endDrag)
}

function onDrag(ev) {
  if (!dragMode || !dragStart) return
  const dx = ev.clientX - dragStart.px
  const dy = ev.clientY - dragStart.py
  const s0 = dragStart.sel

  if (dragMode === 'move') {
    sel.value = clampSelection({ ...s0, x: s0.x + dx, y: s0.y + dy })
    return
  }

  // 四角：对角固定，按指针位置算新边长（保持比例）
  const px = ev.clientX - stageBox.left
  const py = ev.clientY - stageBox.top
  const f = fit.value
  const anchors = {
    nw: { x: s0.x + s0.w, y: s0.y + s0.h }, // 固定右下
    ne: { x: s0.x, y: s0.y + s0.h }, // 固定左下
    sw: { x: s0.x + s0.w, y: s0.y }, // 固定右上
    se: { x: s0.x, y: s0.y } // 固定左上
  }
  const a = anchors[dragMode]
  const rawW = Math.max(Math.abs(px - a.x), Math.abs(py - a.y) * props.aspect)
  // 上限：从锚点出发，图片内还能放多大
  const roomW = dragMode === 'nw' || dragMode === 'sw'
    ? a.x - f.x
    : f.x + f.w - a.x
  const roomH = dragMode === 'nw' || dragMode === 'ne'
    ? a.y - f.y
    : f.y + f.h - a.y
  const maxW = Math.max(MIN_SIZE, Math.min(roomW, roomH * props.aspect))
  const w = Math.min(Math.max(rawW, MIN_SIZE), maxW)
  const h = w / props.aspect
  const x = dragMode === 'nw' || dragMode === 'sw' ? a.x - w : a.x
  const y = dragMode === 'nw' || dragMode === 'ne' ? a.y - h : a.y
  sel.value = clampSelection({ x, y, w, h })
}

function endDrag() {
  dragMode = null
  dragStart = null
  window.removeEventListener('pointermove', onDrag)
  window.removeEventListener('pointerup', endDrag)
  window.removeEventListener('pointercancel', endDrag)
}

// ---------- 画布模式预览：框住哪里就放大显示哪里 ----------
const previewCanvas = ref(null)
const PREVIEW_W = 150

function drawPreview() {
  const cv = previewCanvas.value
  const img = rawImg.value
  if (!cv || !img) return
  const f = fit.value
  if (!f.scale) return
  // 选区 → 原图坐标
  const sx = (sel.value.x - f.x) / f.scale
  const sy = (sel.value.y - f.y) / f.scale
  const sw = sel.value.w / f.scale
  const sh = sel.value.h / f.scale
  const w = PREVIEW_W
  const h = Math.round(w / props.aspect)
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  cv.width = Math.round(w * dpr)
  cv.height = Math.round(h * dpr)
  cv.style.width = w + 'px'
  cv.style.height = h + 'px'
  const ctx = cv.getContext('2d')
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, w, h)
  ctx.imageSmoothingQuality = 'high'
  ctx.save()
  if (isCircle.value) {
    ctx.beginPath()
    ctx.arc(w / 2, h / 2, Math.min(w, h) / 2, 0, Math.PI * 2)
    ctx.clip()
  }
  ctx.drawImage(img, sx, sy, sw, sh, 0, 0, w, h)
  ctx.restore()
}

watch(sel, () => nextTick(drawPreview), { deep: true })

// ---------- 载入图片 ----------
let loadSeq = 0
function load(file) {
  const seq = ++loadSeq
  error.value = ''
  rawImg.value = null
  nat.value = { w: 0, h: 0 }
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
    if (seq !== loadSeq) return   // 已有更新的加载，丢弃这次结果
    rawImg.value = img
    nat.value = { w: img.naturalWidth, h: img.naturalHeight }
    if (!img.naturalWidth || !img.naturalHeight) {
      error.value = '无法读取图片喵~'
      return
    }
    nextTick(() => {
      resetSelection()
      drawPreview()
    })
  }
  // 解码失败：只给文字提示，不再让浏览器画那个「裂开的小图标」
  img.onerror = () => {
    if (seq !== loadSeq) return
    rawImg.value = null
    srcUrl.value = ''
    error.value = '图片加载失败，换一张试试'
  }
  srcUrl.value = URL.createObjectURL(file)
  img.src = srcUrl.value
}

// 只用一个 watcher：open 与 file 常常在同一 tick 里一起变，
// 分成两个 watcher 会 load 两次（第二次把第一次的 blob URL revoke 掉，
// <img> 于是报 ERR_FILE_NOT_FOUND，看起来像「图片加载失败」）。
watch([() => props.open, () => props.file], ([v]) => {
  if (v) {
    computeStage()
    load(props.file)
  } else {
    loadSeq++
    rawImg.value = null
    if (srcUrl.value) URL.revokeObjectURL(srcUrl.value)
    srcUrl.value = ''
  }
})

// ---------- 输出 ----------
const outH = computed(() => Math.round(props.outWidth / props.aspect))

async function confirm() {
  const img = rawImg.value
  if (!img || processing.value) return
  processing.value = true
  try {
    const f = fit.value
    const sx = (sel.value.x - f.x) / f.scale
    const sy = (sel.value.y - f.y) / f.scale
    const sw = sel.value.w / f.scale
    const sh = sel.value.h / f.scale
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
  endDrag()
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
            请拖动方框选择区域，实际效果见右侧预览窗口
          </p>

          <div class="crop-body">
            <!-- 画布：图片固定不动，上面盖一个可选中的方框 -->
            <div
              ref="stageEl"
              class="crop-stage"
              :style="{ width: stage.w + 'px', height: stage.h + 'px' }"
            >
              <!-- alt 留空：解码失败时浏览器不会画那个「裂开的图片」标记，
                   真正的失败提示由预加载的 Image() 给出 -->
              <img
                v-if="srcUrl"
                :src="srcUrl"
                alt=""
                class="crop-img"
                :style="imgStyle"
                draggable="false"
              />
              <div v-if="!srcUrl && !error" class="crop-loading">图片加载中…</div>

              <!-- 选区：白框 + 蓝色粗角 + 可选白色圆环 -->
              <div
                v-if="rawImg"
                class="crop-sel"
                :style="selStyle"
                @pointerdown="beginDrag($event, 'move')"
              >
                <div class="crop-sel-circle" v-if="isCircle"></div>
                <span class="crop-handle nw" @pointerdown.stop="beginDrag($event, 'nw')"></span>
                <span class="crop-handle ne" @pointerdown.stop="beginDrag($event, 'ne')"></span>
                <span class="crop-handle sw" @pointerdown.stop="beginDrag($event, 'sw')"></span>
                <span class="crop-handle se" @pointerdown.stop="beginDrag($event, 'se')"></span>
              </div>
            </div>

            <!-- 画布模式预览：跟随选区放大缩小 -->
            <div class="crop-preview" v-if="rawImg">
              <span class="pv-label">预览</span>
              <canvas ref="previewCanvas" class="pv-canvas" :class="{ round: isCircle }"></canvas>
              <span class="pv-size">{{ Math.round(sel.w) }} × {{ Math.round(sel.h) }} px</span>
            </div>
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
  display: grid;
  place-items: center;
  padding: 24px;
}
.crop-card {
  /* 宽度跟着画布走：竖图不会在旁边留一大片空白 */
  width: fit-content;
  max-width: min(1120px, 96vw);
  max-height: 92vh;
  overflow: auto;
  background: var(--card);
  border: 1px solid var(--glass-border);
  border-radius: 16px;
  box-shadow: var(--shadow-lg, 0 24px 60px rgba(0, 0, 0, 0.28));
  padding: 18px 22px 20px;
}
.crop-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px; }
.crop-head h3 { font-size: 15px; font-weight: 700; }
.crop-x {
  border: 0; background: transparent; color: var(--text-dim);
  font-size: 16px; cursor: pointer; line-height: 1; padding: 4px 6px; border-radius: 6px;
}
.crop-x:hover { background: var(--bg-alt); color: var(--text); }

.crop-tip { font-size: 12.5px; color: var(--text-dim); margin-bottom: 12px; }
.crop-ratio { color: var(--accent); font-weight: 600; }

.crop-body { display: flex; align-items: flex-start; gap: 18px; }
.crop-preview { min-width: 190px; }

/* 画布 */
.crop-stage {
  position: relative;
  flex: none;
  border-radius: 10px;
  overflow: hidden;
  background: repeating-conic-gradient(#2b323c 0% 25%, #242a33 0% 50%) 50% / 16px 16px;
  touch-action: none;
  user-select: none;
}
.crop-img { position: absolute; object-fit: fill; pointer-events: none; }
.crop-loading {
  position: absolute; inset: 0; display: grid; place-items: center;
  color: #cfd6e4; font-size: 13px;
}

/* 选区：白色描边 + 深色遮罩 */
.crop-sel {
  position: absolute;
  box-sizing: border-box;
  border: 2px solid #fff;
  box-shadow: 0 0 0 9999px rgba(0, 0, 0, 0.45);
  cursor: move;
  touch-action: none;
}
.crop-sel-circle {
  position: absolute;
  inset: 0;
  border: 3px solid #fff;
  border-radius: 50%;
  pointer-events: none;
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.25);
}
/* 四角：加粗蓝色直角（L 形），拖角改大小 */
.crop-handle {
  position: absolute;
  width: 26px;
  height: 26px;
  border: 0 solid var(--accent);
  border-radius: 1px;
  filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.45));
  touch-action: none;
}
.crop-handle.nw {
  left: -2px; top: -2px;
  border-top-width: 5px; border-left-width: 5px;
  border-top-left-radius: 3px;
  cursor: nwse-resize;
}
.crop-handle.ne {
  right: -2px; top: -2px;
  border-top-width: 5px; border-right-width: 5px;
  border-top-right-radius: 3px;
  cursor: nesw-resize;
}
.crop-handle.sw {
  left: -2px; bottom: -2px;
  border-bottom-width: 5px; border-left-width: 5px;
  border-bottom-left-radius: 3px;
  cursor: nesw-resize;
}
.crop-handle.se {
  right: -2px; bottom: -2px;
  border-bottom-width: 5px; border-right-width: 5px;
  border-bottom-right-radius: 3px;
  cursor: nwse-resize;
}

/* 右侧实时预览 */
.crop-preview {
  flex: none;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 10px;
  border-radius: 12px;
  background: var(--bg-alt);
  border: 1px solid var(--border);
}
.pv-label { font-size: 11.5px; color: var(--text-dim); letter-spacing: 0.4px; }
.pv-canvas { display: block; border-radius: 8px; background: #11161d; }
.pv-canvas.round { border-radius: 50%; }
.pv-size { font-size: 11.5px; color: var(--text-dim); font-variant-numeric: tabular-nums; }

.crop-err { color: #e5484d; font-size: 12.5px; margin: 12px 0 0; }
.crop-note { color: var(--text-dim); font-size: 12px; margin: 12px 0 0; }

.crop-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 16px; }

.fade-enter-active, .fade-leave-active { transition: opacity 0.18s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
