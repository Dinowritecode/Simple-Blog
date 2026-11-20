<script setup>
// ============================================================
// AdminEditorView.vue —— 写 / 改文章
//
// 布局：左侧编辑器吃满整页高度，右侧固定一栏放「元信息」——
//   标题 / 摘要 / 分类 / 标签 / 封面 / 发布状态，以及保存按钮。
// 这样正文区不用再被一堆表单挤在中间，写起来接近独立编辑器。
// ============================================================
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import '@wangeditor/editor/dist/css/style.css'
import { Editor, Toolbar } from '@wangeditor/editor-for-vue'
import { api } from '../api'
import ImageUrlField from '../components/admin/ImageUrlField.vue'
import TagInput from '../components/admin/TagInput.vue'
import downIcon from '../others/down.svg'

const route = useRoute()
const router = useRouter()

const postId = computed(() => route.params.id || null)
const isEdit = computed(() => !!postId.value)

const form = ref({ title: '', summary: '', cover: '', category: '', tags: '', content: '', status: 'draft' })
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
      category: data.category || '',
      tags: data.tags || '',
      content: data.content,
      status: data.status
    }
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

/* ---------- 分类候选：从已有文章汇总，输入框可直接选也可以输入新分类 ---------- */
const categories = ref([])
const catOpen = ref(false)   // 分类下拉是否展开

async function loadCategories() {
  try {
    const data = await api.adminListPosts({ page: 1, pageSize: 50 })
    const set = new Set()
    for (const it of data.items || []) if (it.category) set.add(it.category)
    categories.value = [...set].sort()
  } catch {
    /* 拉不到候选也无所谓，分类仍可手动输入 */
  }
}

onMounted(loadCategories)
onMounted(loadPost)
watch(postId, () => loadPost())

/* ---------- 封面裁剪比例 ---------- */
// 文章封面在详情页是通栏 banner：
//   min-height = clamp(300px, 36vh, 480px) + --nav-h(56px)，宽 = 视口宽
// 后台裁出来的画面和详情页顶部看到的一模一样。
const coverAspect = computed(() => {
  const w = typeof window !== 'undefined' ? window.innerWidth : 1440
  const h = typeof window !== 'undefined' ? window.innerHeight : 900
  const banner = Math.min(480, Math.max(300, h * 0.36)) + 56
  return Math.min(3.2, Math.max(1.4, w / Math.max(banner, 1)))
})

/* ---------- 保存 ---------- */
async function save(status) {
  if (typeof status === 'string') form.value.status = status
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
      const created = await api.createPost(data)
      savedTip.value = '✅ 已创建'
      // 新建后跳到编辑页，方便继续写 / 改封面
      if (created && created.id) {
        router.replace(`/admin/posts/${created.id}/edit`)
        return
      }
    }
    savedTip.value = '✅ 保存成功'
    setTimeout(() => (savedTip.value = ''), 2500)
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

const wordCount = computed(() => {
  const text = String(form.value.content || '').replace(/<[^>]+>/g, '')
  return text.replace(/\s/g, '').length
})

onBeforeUnmount(() => {
  editorRef.value?.destroy()
})
</script>

<template>
  <div class="ed">
    <!-- 顶部条：标题 + 保存状态（不占右侧面板的位置） -->
    <header class="ed-head">
      <h1>{{ isEdit ? '编辑文章' : '写文章' }}</h1>
      <div class="ed-head-right">
        <span v-if="savedTip" class="saved-tip">{{ savedTip }}</span>
        <span class="ed-count">{{ wordCount }} 字</span>
        <button class="btn btn-ghost" @click="router.push('/admin')">返回列表</button>
      </div>
    </header>

    <p v-if="error" class="ed-error">{{ error }}</p>
    <div v-if="loading" class="ed-state">加载中…</div>

    <div v-else class="ed-body">
      <!-- 左：编辑器，占满剩余高度 -->
      <div class="ed-main">
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

      <!-- 右：元信息面板 -->
      <aside class="ed-side">
        <div class="side-block">
          <label class="side-field">
            <span>标题 <i>*</i></span>
            <input v-model="form.title" type="text" placeholder="文章标题" />
          </label>

          <label class="side-field">
            <span>摘要</span>
            <textarea v-model="form.summary" rows="3" placeholder="列表页展示的简短介绍（可选）"></textarea>
          </label>

          <!-- 分类：只能有一个。可以直接输入，也可以点 ▾ 从已有分类里选 -->
          <div class="side-field cat-wrap">
            <span>分类</span>
            <div class="cat-row">
              <input
                v-model="form.category"
                type="text"
                list="category-options"
                placeholder="输入或选择分类"
                @focus="catOpen = false"
              />
              <!-- 箭头嵌在输入框里：图标用 others/down.svg，点一下展开已有分类 -->
              <button
                class="cat-toggle"
                type="button"
                :class="{ on: catOpen }"
                :disabled="!categories.length"
                :title="categories.length ? '选择已有分类' : '还没有已有分类'"
                @click="catOpen = !catOpen"
              >
                <img :src="downIcon" alt="" />
              </button>
              <datalist id="category-options">
                <option v-for="c in categories" :key="c" :value="c"></option>
              </datalist>
            </div>
            <ul v-if="catOpen" class="cat-menu">
              <li v-for="c in categories" :key="c">
                <button type="button" @click="form.category = c; catOpen = false">{{ c }}</button>
              </li>
            </ul>
          </div>

          <div class="side-field">
            <span>标签</span>
            <TagInput
              v-model="form.tags"
              placeholder="输入后回车，可加多个"
            />
          </div>
        </div>

        <div class="side-block">
          <div class="side-label">
            <span>封面图</span>
            <span class="side-hint">点 👁 看效果，📁 上传后可裁剪</span>
          </div>
          <ImageUrlField
            v-model="form.cover"
            :aspect="coverAspect"
            :out-width="1920"
            crop-title="裁剪文章封面"
            :crop-note="`裁剪框比例 ${coverAspect.toFixed(2)} : 1 与文章详情页顶部 banner 一致。`"
          />
        </div>

        <div class="side-block">
          <div class="side-label"><span>发布状态</span></div>
          <div class="status-row">
            <button
              class="status-btn"
              :class="{ on: form.status === 'draft' }"
              type="button"
              @click="form.status = 'draft'"
            >草稿</button>
            <button
              class="status-btn"
              :class="{ on: form.status === 'published' }"
              type="button"
              @click="form.status = 'published'"
            >公开</button>
          </div>
          <p class="status-tip">
            {{ form.status === 'draft' ? '仅自己可见，不会出现在首页' : '保存后立即对所有人可见' }}
          </p>
        </div>

        <div class="side-actions">
          <button class="btn btn-primary block" :disabled="saving" @click="save()">
            {{ saving ? '保存中…' : form.status === 'published' ? '保存并公开' : '保存草稿' }}
          </button>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.ed {
  display: flex;
  flex-direction: column;
  /* 后台内容区高度：视口减去 admin-main 的上下 padding */
  height: calc(100svh - 64px);
  min-height: 520px;
}

.ed-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.ed-head h1 { font-size: 1.35rem; }
.ed-head-right { display: flex; align-items: center; gap: 12px; }
.ed-count { font-size: 12.5px; color: var(--text-dim); font-variant-numeric: tabular-nums; }
.saved-tip { color: #16a34a; font-size: 13px; }
.ed-error { color: #e5484d; margin-bottom: 12px; }
.ed-state { text-align: center; color: var(--text-dim); padding: 60px 0; }

/* 两栏：编辑器自适应 + 右侧固定 340 */
.ed-body {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 18px;
}

/* ---------- 编辑器 ---------- */
.ed-main { min-width: 0; display: flex; }
.wangeditor {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--glass-border);
  border-radius: 12px;
  overflow: hidden;
  background: var(--glass-solid-bg);
  backdrop-filter: var(--glass-solid-blur);
  -webkit-backdrop-filter: var(--glass-solid-blur);
}
.wangeditor-toolbar { border-bottom: 1px solid var(--border); flex: none; }
.wangeditor-body { flex: 1; min-height: 0; overflow-y: auto; }
.wangeditor-body :deep(.w-e-text-container) { height: 100% !important; }
.wangeditor-body :deep(.w-e-scroll) { min-height: 100%; }

/* ---------- 右侧面板 ---------- */
.ed-side {
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-right: 2px;
}
.side-block {
  background: var(--card);
  border: 1px solid var(--glass-border);
  border-radius: 12px;
  box-shadow: var(--shadow);
  padding: 14px 15px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.side-field { display: flex; flex-direction: column; gap: 6px; }
.side-field > span { font-size: 12.5px; font-weight: 600; color: var(--text-dim); }
.side-field > span i { color: #e5484d; font-style: normal; }
.side-field input,
.side-field textarea {
  padding: 9px 12px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--bg);
  color: var(--text);
  font-size: 14px;
  font-family: inherit;
  outline: none;
  transition: border-color 0.18s ease;
}
.side-field input:focus, .side-field textarea:focus { border-color: var(--accent); }
.side-field textarea { resize: vertical; }

/* 分类：输入框 + 下拉按钮（一个值，不做成标签） */
.cat-wrap { position: relative; }
.cat-row { position: relative; }
.cat-row input { width: 100%; padding-right: 34px; }
/* 箭头按钮：贴在输入框内部右侧，不额外占一行宽度 */
.cat-toggle {
  position: absolute;
  right: 6px;
  top: 50%;
  transform: translateY(-50%);
  width: 24px;
  height: 24px;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  transition: background 0.16s ease;
}
.cat-toggle img {
  width: 14px;
  height: 14px;
  display: block;
  opacity: 0.55;
  /* 图标本身是 #333 描边，这里用滤镜压成跟随主题的灰/蓝 */
  transition: opacity 0.16s ease, transform 0.16s ease;
}
.cat-toggle:hover:not(:disabled) { background: var(--accent-soft); }
.cat-toggle:hover:not(:disabled) img { opacity: 1; }
.cat-toggle.on img { transform: rotate(180deg); opacity: 1; }
.cat-toggle:disabled { opacity: 0.35; cursor: not-allowed; }
.cat-menu {
  position: absolute;
  z-index: 30;
  left: 0;
  right: 0;
  top: calc(100% + 4px);
  max-height: 220px;
  overflow-y: auto;
  margin: 0;
  padding: 4px;
  list-style: none;
  background: var(--card);
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  box-shadow: var(--shadow-lg, 0 12px 30px rgba(0, 0, 0, 0.18));
}
.cat-menu button {
  display: block;
  width: 100%;
  text-align: left;
  padding: 7px 10px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
}
.cat-menu button:hover { background: var(--accent-soft); color: var(--accent-ink); }

.side-label { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; }
.side-label > span:first-child { font-size: 12.5px; font-weight: 600; color: var(--text-dim); }
.side-hint { font-size: 11.5px; color: var(--text-dim); }

/* 发布状态：两个胶囊按钮 */
.status-row { display: flex; gap: 8px; }
.status-btn {
  flex: 1;
  padding: 8px 0;
  border: 1.5px solid var(--border);
  border-radius: 999px;
  background: transparent;
  color: var(--text-dim);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.16s ease;
}
.status-btn:hover { border-color: var(--accent); color: var(--accent); }
.status-btn.on { border-color: var(--accent); background: var(--accent); color: #fff; }
.status-tip { font-size: 11.5px; color: var(--text-dim); margin: 0; }

.side-actions .block { width: 100%; }

@media (max-width: 1080px) {
  .ed { height: auto; }
  .ed-body { grid-template-columns: minmax(0, 1fr); }
  .ed-main { min-height: 60vh; }
}
</style>
