<script setup>
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import '@wangeditor/editor/dist/css/style.css'
import { Editor, Toolbar } from '@wangeditor/editor-for-vue'
import { api } from '../api'

const route = useRoute()
const router = useRouter()

const postId = computed(() => route.params.id || null)
const isEdit = computed(() => !!postId.value)

const form = ref({ title: '', summary: '', cover: '', content: '', status: 'draft' })
const loading = ref(isEdit.value)
const saving = ref(false)
const error = ref('')
const savedTip = ref('')

/* ---------- wangEditor ---------- */
const editorRef = shallowRef()
const toolbarConfig = { excludeKeys: ['group-video', 'insertVideo'] }
const editorConfig = {
  placeholder: '开始写作…',
  MENU_CONF: {
    uploadImage: {
      // 自定义上传：走我们的接口（携带 JWT）
      async customUpload(file, insertFn) {
        const res = await api.uploadImage(file)
        insertFn(res.url, file.name, res.url)
      }
    }
  }
}

/* ---------- 数据加载 ---------- */
async function loadPost() {
  if (!isEdit.value) return
  loading.value = true
  error.value = ''
  try {
    const data = await api.adminGetPost(postId.value)
    form.value = {
      title: data.title,
      summary: data.summary,
      cover: data.cover,
      content: data.content,
      status: data.status
    }
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(loadPost)

// 从编辑页切换文章时重新加载
watch(postId, () => loadPost())

/* ---------- 封面图片上传 ---------- */
const coverInput = ref(null)
const uploadingCover = ref(false)

function pickCover() {
  coverInput.value?.click()
}

async function onCoverChange(e) {
  const file = e.target.files?.[0]
  if (!file) return
  uploadingCover.value = true
  try {
    const res = await api.uploadImage(file)
    form.value.cover = res.url
  } catch (err) {
    alert(err.message)
  } finally {
    uploadingCover.value = false
    e.target.value = ''
  }
}

/* ---------- 保存 ---------- */
async function save() {
  if (!form.value.title.trim()) {
    error.value = '请填写标题'
    return
  }
  saving.value = true
  error.value = ''
  savedTip.value = ''
  try {
    const data = { ...form.value }
    if (isEdit.value) {
      await api.updatePost(postId.value, data)
    } else {
      await api.createPost(data)
    }
    savedTip.value = '✅ 保存成功'
    setTimeout(() => (savedTip.value = ''), 2500)
    // 新建后跳转到编辑页，方便继续完善
    if (!isEdit.value) {
      router.replace('/admin')
    }
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

onBeforeUnmount(() => {
  editorRef.value?.destroy()
})
</script>

<template>
  <div class="editor-view">
    <div class="editor-head">
      <h1>{{ isEdit ? '编辑文章' : '写文章' }}</h1>
      <div class="editor-head-actions">
        <span v-if="savedTip" class="saved-tip">{{ savedTip }}</span>
        <button class="btn btn-ghost" @click="router.push('/admin')">取消</button>
        <button class="btn btn-primary" :disabled="saving" @click="save">
          {{ saving ? '保存中…' : '保存' }}
        </button>
      </div>
    </div>

    <p v-if="error" class="editor-error">{{ error }}</p>
    <div v-if="loading" class="editor-state">加载中…</div>

    <div v-else class="editor-form">
      <label class="field">
        <span>标题 *</span>
        <input v-model="form.title" type="text" placeholder="文章标题" />
      </label>

      <label class="field">
        <span>摘要</span>
        <textarea v-model="form.summary" rows="2" placeholder="列表页展示的简短介绍（可选）"></textarea>
      </label>

      <div class="field">
        <span>封面图（可选）</span>
        <div class="cover-row">
          <input v-model="form.cover" type="text" placeholder="/uploads/xxx.jpg 或 https://…" />
          <button class="btn btn-ghost" type="button" :disabled="uploadingCover" @click="pickCover">
            {{ uploadingCover ? '上传中…' : '上传' }}
          </button>
          <input ref="coverInput" type="file" accept="image/*" hidden @change="onCoverChange" />
        </div>
      </div>

      <div class="field">
        <span>正文（支持图片上传）</span>
        <div class="wangeditor">
          <Toolbar :editor="editorRef" :defaultConfig="toolbarConfig" mode="default" class="wangeditor-toolbar" />
          <Editor
            v-model="form.content"
            :defaultConfig="editorConfig"
            mode="default"
            class="wangeditor-body"
            @onCreated="editorRef = $event"
          />
        </div>
      </div>

      <label class="field field-status">
        <span>发布状态</span>
        <select v-model="form.status">
          <option value="draft">草稿（不公开）</option>
          <option value="published">发布（公开可见）</option>
        </select>
      </label>
    </div>
  </div>
</template>

<style scoped>
.editor-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 22px;
  gap: 16px;
  flex-wrap: wrap;
}
.editor-head h1 { font-size: 1.4rem; }
.editor-head-actions { display: flex; align-items: center; gap: 10px; }

.saved-tip { color: #16a34a; font-size: 14px; }
.editor-error { color: #e5484d; margin-bottom: 16px; }
.editor-state { text-align: center; color: var(--text-dim); padding: 60px 0; }

.editor-form { display: flex; flex-direction: column; gap: 18px; max-width: 960px; }

.field { display: flex; flex-direction: column; gap: 6px; }
.field > span { font-size: 13.5px; color: var(--text-dim); font-weight: 600; }

.field input[type="text"],
.field textarea,
.field select {
  padding: 11px 14px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg);
  color: var(--text);
  font-size: 15px;
  outline: none;
  transition: border-color 0.2s ease;
  font-family: inherit;
}
.field input:focus, .field textarea:focus, .field select:focus { border-color: var(--accent); }
.field textarea { resize: vertical; }

.cover-row { display: flex; gap: 10px; }
.cover-row input { flex: 1; }
.cover-row .btn { flex-shrink: 0; padding: 10px 18px; }

.field-status { max-width: 260px; }

/* wangEditor 容器 */
.wangeditor {
  border: 1px solid var(--border);
  border-radius: 12px;
  overflow: hidden;
  background: var(--card);
  z-index: 5;
}
.wangeditor-toolbar { border-bottom: 1px solid var(--border); }
.wangeditor-body { height: 460px; overflow-y: hidden; }
.wangeditor-body :deep(.w-e-text-container) { height: 460px !important; }
</style>
