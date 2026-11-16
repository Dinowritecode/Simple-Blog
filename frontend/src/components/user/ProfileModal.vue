<script setup>
import { ref, computed, watch } from 'vue'
import { api } from '../../api'
import { useAuth } from '../../composables/useAuth'
import { useToast } from '../../composables/useToast'

const props = defineProps({ open: Boolean })
const emit = defineEmits(['close'])

const { user, displayName, avatarUrl, setUser } = useAuth()
const { toast } = useToast()

// ---------- 昵称 ----------
const nickname = ref('')
const savingNick = ref(false)
watch(
  () => props.open,
  (v) => {
    if (v) nickname.value = user.value?.nickname || ''
  }
)

async function saveNickname() {
  const nick = nickname.value.trim()
  if (!nick) return toast.error('昵称不能为空')
  if (nick.length > 30) return toast.error('昵称最长 30 个字符')
  savingNick.value = true
  try {
    const me = await api.updateProfile(nick)
    setUser({ nickname: me.nickname })
    toast.success('昵称已更新')
  } catch (e) {
    toast.error(e.message)
  } finally {
    savingNick.value = false
  }
}

// ---------- 头像裁剪上传（≤5MB，纯 canvas 实现） ----------
const stageSize = 240 // 裁剪框 CSS 尺寸
const outSize = 256   // 输出尺寸
const maxAvatarSize = 5 * 1024 * 1024
const allowTypes = ['image/jpeg', 'image/png', 'image/webp', 'image/gif']

const rawImg = ref(null)      // HTMLImageElement
const srcUrl = ref('')        // objectURL
const zoom = ref(1)           // 1~4
const ox = ref(0)             // 图片显示偏移（px，左上为负）
const oy = ref(0)
const dragging = ref(false)
const dragFrom = ref({ x: 0, y: 0, ox: 0, oy: 0 })
const uploading = ref(false)
const fileInput = ref(null)

const displayW = computed(() => rawImg.value ? rawImg.value.naturalWidth * baseScale.value * zoom.value : 0)
const displayH = computed(() => rawImg.value ? rawImg.value.naturalHeight * baseScale.value * zoom.value : 0)
// 基础缩放：让图片至少铺满裁剪框（cover）
const baseScale = computed(() => {
  if (!rawImg.value) return 1
  const { naturalWidth: w, naturalHeight: h } = rawImg.value
  return Math.max(stageSize / w, stageSize / h)
})

function clampOffset() {
  ox.value = Math.min(0, Math.max(ox.value, stageSize - displayW.value))
  oy.value = Math.min(0, Math.max(oy.value, stageSize - displayH.value))
}

function chooseAvatar(e) {
  const f = e.target.files && e.target.files[0]
  e.target.value = '' // 允许重复选择同一文件
  if (!f) return
  if (f.size > maxAvatarSize) return toast.error('头像不能超过 5MB')
  if (!allowTypes.includes(f.type)) return toast.error('仅支持 jpg / png / webp / gif 图片')
  if (srcUrl.value) URL.revokeObjectURL(srcUrl.value)
  const img = new Image()
  img.onload = () => {
    rawImg.value = img
    zoom.value = 1
    // 居中
    ox.value = (stageSize - displayW.value) / 2
    oy.value = (stageSize - displayH.value) / 2
    clampOffset()
  }
  srcUrl.value = URL.createObjectURL(f)
  img.src = srcUrl.value
}

function onZoom() { clampOffset() }

function onDown(ev) {
  if (!rawImg.value) return
  dragging.value = true
  dragFrom.value = { x: ev.clientX, y: ev.clientY, ox: ox.value, oy: oy.value }
}
function onMove(ev) {
  if (!dragging.value) return
  ox.value = dragFrom.value.ox + (ev.clientX - dragFrom.value.x)
  oy.value = dragFrom.value.oy + (ev.clientY - dragFrom.value.y)
  clampOffset()
}
function onUp() { dragging.value = false }

function cancelCrop() {
  rawImg.value = null
  if (srcUrl.value) URL.revokeObjectURL(srcUrl.value)
  srcUrl.value = ''
}

function extOf(mime) {
  return { 'image/jpeg': '.jpg', 'image/png': '.png', 'image/webp': '.webp', 'image/gif': '.gif' }[mime] || '.jpg'
}

async function uploadAvatar() {
  const img = rawImg.value
  if (!img) return
  const s = baseScale.value * zoom.value
  const sx = -ox.value / s
  const sy = -oy.value / s
  const sw = stageSize / s
  uploading.value = true
  try {
    const canvas = document.createElement('canvas')
    canvas.width = outSize
    canvas.height = outSize
    const ctx = canvas.getContext('2d')
    ctx.imageSmoothingQuality = 'high'
    ctx.drawImage(img, sx, sy, sw, sw, 0, 0, outSize, outSize)
    const blob = await new Promise((res) => canvas.toBlob(res, 'image/jpeg', 0.92))
    if (!blob) throw new Error('图片处理失败')
    if (blob.size > maxAvatarSize) throw new Error('处理后的头像超过 5MB，请换一张图')
    const file = new File([blob], 'avatar' + extOf('image/jpeg'), { type: 'image/jpeg' })
    const data = await api.uploadAvatar(file)
    setUser({ avatar: data.avatar })
    toast.success('头像上传成功！')
    cancelCrop()
  } catch (e) {
    toast.error(e.message || '上传失败')
  } finally {
    uploading.value = false
  }
}

// 关闭时清理
function close() {
  if (uploading.value || savingNick.value) return
  cancelCrop()
  emit('close')
}
</script>

<template>
  <Teleport to="body">
    <transition name="fade">
      <div v-if="open" class="modal-mask" @click.self="close">
        <div class="modal-card profile-card">
          <button class="modal-close" aria-label="关闭" @click="close">✕</button>
          <h3>个人资料</h3>

          <!-- 头像 -->
          <div class="avatar-block">
            <div class="avatar avatar-lg" style="--avatar-size: 72px">
              <img v-if="avatarUrl" :src="avatarUrl" alt="头像" />
              <span v-else class="avatar-letter">{{ (displayName || '?')[0] }}</span>
            </div>
            <div class="avatar-tips">
              <div class="avatar-email">{{ user?.email }}</div>
              <div class="avatar-sub">正方形图片效果最佳，不超过 5MB</div>
            </div>
          </div>

          <!-- 裁剪区 -->
          <div v-if="rawImg" class="crop-wrap">
            <div class="crop-stage" @mousedown="onDown" @mousemove="onMove" @mouseup="onUp" @mouseleave="onUp">
              <img
                :src="srcUrl"
                :style="{ width: displayW + 'px', height: displayH + 'px', transform: `translate(${ox}px, ${oy}px)` }"
                draggable="false"
                alt="裁剪预览"
              />
              <div class="crop-frame"></div>
            </div>
            <div class="crop-controls">
              <label class="crop-zoom">
                <span>缩放</span>
                <input v-model.number="zoom" type="range" min="1" max="4" step="0.01" @input="onZoom" />
              </label>
              <div class="crop-actions">
                <button class="btn-sm ghost" :disabled="uploading" @click="cancelCrop">取消</button>
                <button class="btn-sm" :disabled="uploading" @click="uploadAvatar">
                  {{ uploading ? '上传中…' : '确认上传' }}
                </button>
              </div>
            </div>
          </div>

          <!-- 未裁剪时的操作 -->
          <template v-else>
            <label class="m-field">
              <span>昵称</span>
              <input v-model="nickname" maxlength="30" placeholder="展示在评论区" />
            </label>
            <div class="profile-actions">
              <button class="btn-sm ghost" :disabled="savingNick" @click="fileInput.click()">📷 更换头像</button>
              <button class="btn-sm" :disabled="savingNick" @click="saveNickname">
                {{ savingNick ? '保存中…' : '保存昵称' }}
              </button>
            </div>
          </template>

          <input ref="fileInput" type="file" accept="image/*" hidden @change="chooseAvatar" />
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<style scoped>
.profile-card { max-width: 460px; }
.avatar-block { display: flex; align-items: center; gap: 16px; margin-bottom: 18px; }
.avatar-lg { border: 2px solid var(--border); }
.avatar-tips { min-width: 0; }
.avatar-email { font-weight: 600; font-size: 14.5px; overflow: hidden; text-overflow: ellipsis; }
.avatar-sub { color: var(--text-dim); font-size: 12.5px; margin-top: 3px; }

.crop-wrap { margin-bottom: 16px; }
.crop-stage {
  position: relative;
  width: 240px;
  height: 240px;
  margin: 0 auto 14px;
  overflow: hidden;
  border-radius: 12px;
  background: var(--bg-alt);
  border: 1px solid var(--border);
  cursor: grab;
  touch-action: none;
}
.crop-stage:active { cursor: grabbing; }
.crop-stage img { position: absolute; left: 0; top: 0; max-width: none; user-select: none; pointer-events: none; }
.crop-frame {
  position: absolute;
  inset: 0;
  box-shadow: 0 0 0 100vmax rgba(0, 0, 0, 0.35);
  border: 2px solid rgba(255, 255, 255, 0.8);
  border-radius: 12px;
  pointer-events: none;
}
.crop-controls { display: flex; align-items: center; gap: 14px; }
.crop-zoom { display: flex; align-items: center; gap: 8px; flex: 1; font-size: 13px; color: var(--text-dim); }
.crop-zoom input[type='range'] { flex: 1; accent-color: var(--accent); }
.crop-actions { display: flex; gap: 8px; }

.profile-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 6px; }
</style>
