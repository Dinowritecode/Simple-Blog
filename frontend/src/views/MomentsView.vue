<script setup>
// ============================================================
// MomentsView.vue —— 说说（类似 Twitter 的短动态）
//
// 和博客文章完全分开：独立路由 /moments，不混进文章列表和搜索的「文章」结果。
//   · 发布：登录后可用，一段文字 + 最多 9 张图（先上传拿 URL，再随正文一起提交）
//   · 浏览数：进页面时按条上报，后端按访客 Cookie 去重，刷新不会把数字刷上去
//   · 点赞：匿名也能点，同一访客一次，再点取消
//   · 分享：优先调系统分享面板，不支持就复制链接
// ============================================================

import { computed, nextTick, onMounted, ref } from 'vue'
import { api } from '../api'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'
import CommentSection from '../components/comment/CommentSection.vue'
import photoUrl from '../others/photo.jpg'

const { isLoggedIn, isAdmin, displayName, avatarUrl } = useAuth()
const { toast } = useToast()

const PAGE_SIZE = 10
const MAX_IMAGES = 9
const MAX_CHARS = 2000

const items = ref([])
const loading = ref(true)
const loadingMore = ref(false)
const page = ref(1)
const hasMore = ref(false)
const error = ref('')

// ---------- 时间显示：刚刚 / X 分钟前 / 今天 18:20 / 10-24 18:20 ----------
function timeAgo(s) {
  if (!s) return ''
  const t = new Date(s)
  const now = new Date()
  const diff = now - t
  if (diff < 60 * 1000) return '刚刚'
  if (diff < 60 * 60 * 1000) return `${Math.floor(diff / 60000)} 分钟前`
  const hm = `${String(t.getHours()).padStart(2, '0')}:${String(t.getMinutes()).padStart(2, '0')}`
  if (t.toDateString() === now.toDateString()) return `今天 ${hm}`
  const md = `${String(t.getMonth() + 1).padStart(2, '0')}-${String(t.getDate()).padStart(2, '0')}`
  return `${md} ${hm}`
}

// ---------- 浏览数：同一浏览器只上报一次，避免每次刷新都发请求 ----------
const VIEWED_KEY = 'moments_viewed'
function viewedSet() {
  try {
    return new Set(JSON.parse(localStorage.getItem(VIEWED_KEY) || '[]'))
  } catch {
    return new Set()
  }
}
function markViewed(id) {
  const set = viewedSet()
  set.add(id)
  // 只留最近 300 条，别让 localStorage 无限膨胀
  const arr = [...set].slice(-300)
  try {
    localStorage.setItem(VIEWED_KEY, JSON.stringify(arr))
  } catch {
    /* 存不下就下次再报，不影响功能 */
  }
}
async function reportViews(list) {
  const seen = viewedSet()
  for (const m of list) {
    if (seen.has(m.id)) continue
    markViewed(m.id)
    try {
      const r = await api.viewMoment(m.id)
      if (typeof r.views === 'number') m.views = r.views
    } catch {
      /* 浏览数算附属信息，失败就忽略 */
    }
  }
}

// ---------- 列表 ----------
async function load(reset = true) {
  if (reset) {
    loading.value = true
    page.value = 1
    error.value = ''
  } else {
    loadingMore.value = true
  }
  try {
    const d = await api.listMoments({ page: page.value, pageSize: PAGE_SIZE })
    const list = d.items || []
    if (reset) items.value = list
    else items.value = items.value.concat(list)
    hasMore.value = !!d.hasMore
    // 注意传 items.value 而不是 list：前者是 Vue 的响应式代理，
    // 改它上面的 views 才会触发重新渲染（直接改原始对象不会）。
    reportViews(items.value.slice(reset ? 0 : items.value.length - list.length))
  } catch (e) {
    if (reset) error.value = e.message || '加载失败'
    else toast.error(e.message || '加载失败')
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

function loadMore() {
  if (loadingMore.value || !hasMore.value) return
  page.value += 1
  load(false)
}

onMounted(() => load(true))

// ---------- 发布 ----------
const draft = ref('')
const picked = ref([]) // [{ url, uploading }]
const posting = ref(false)
const fileInput = ref(null)

const canPost = computed(() => {
  if (posting.value) return false
  if (picked.value.some((p) => p.uploading)) return false
  return draft.value.trim().length > 0 || picked.value.length > 0
})
const leftChars = computed(() => MAX_CHARS - draft.value.length)

function pickImages() {
  if (picked.value.length >= MAX_IMAGES) {
    toast.error(`最多 ${MAX_IMAGES} 张图片`)
    return
  }
  fileInput.value && fileInput.value.click()
}

async function onFiles(e) {
  const files = [...(e.target.files || [])]
  e.target.value = '' // 允许连续选同一张
  for (const file of files) {
    if (picked.value.length >= MAX_IMAGES) {
      toast.error(`最多 ${MAX_IMAGES} 张图片`)
      break
    }
    if (!file.type.startsWith('image/')) {
      toast.error('只能选图片')
      continue
    }
    const slot = { url: '', uploading: true }
    picked.value.push(slot)
    try {
      const r = await api.uploadMomentImage(file)
      slot.url = r.url
      slot.uploading = false
    } catch (err) {
      picked.value = picked.value.filter((p) => p !== slot)
      toast.error(err.message || '图片上传失败')
    }
  }
}

function removeImage(i) {
  picked.value.splice(i, 1)
}

async function publish() {
  if (!canPost.value) return
  posting.value = true
  try {
    const m = await api.createMoment({
      content: draft.value.trim(),
      images: picked.value.map((p) => p.url)
    })
    items.value = [m, ...items.value]
    draft.value = ''
    picked.value = []
    toast.success('发布成功')
    nextTick(() => window.scrollTo({ top: 0, behavior: 'smooth' }))
  } catch (e) {
    toast.error(e.message || '发布失败')
  } finally {
    posting.value = false
  }
}

// ---------- 点赞 ----------
const liking = ref({})
async function toggleLike(m) {
  if (liking.value[m.id]) return
  liking.value[m.id] = true
  try {
    const r = await api.likeMoment(m.id)
    m.liked = !!r.liked
    m.likes = r.likes
  } catch (e) {
    toast.error(e.message || '操作失败')
  } finally {
    liking.value[m.id] = false
  }
}

// ---------- 评论：默认收起，点一下才展开并加载（避免一次拉 N 个评论列表） ----------
const openComments = ref({})
function toggleComments(m) {
  openComments.value = { ...openComments.value, [m.id]: !openComments.value[m.id] }
}

// ---------- 分享 ----------
async function share(m) {
  const url = `${location.origin}/moments#m-${m.id}`
  const title = (m.content || '一条说说').slice(0, 40)
  if (navigator.share) {
    try {
      await navigator.share({ title, url })
      return
    } catch {
      /* 用户取消 */
    }
  }
  try {
    await navigator.clipboard.writeText(url)
    toast.success('链接已复制')
  } catch {
    toast.error('复制失败，请手动复制地址栏链接')
  }
}

// ---------- 删除 ----------
function canDelete(m) {
  return isLoggedIn.value && (isAdmin.value || m.author?.id === currentUserId())
}
function currentUserId() {
  try {
    return Number(JSON.parse(localStorage.getItem('user') || '{}').id || 0)
  } catch {
    return 0
  }
}
async function remove(m) {
  if (!confirm('删除这条说说？')) return
  try {
    await api.deleteMoment(m.id)
    items.value = items.value.filter((x) => x.id !== m.id)
    toast.success('已删除')
  } catch (e) {
    toast.error(e.message || '删除失败')
  }
}

function authorAvatar(m) {
  return m.author?.avatar || (m.author?.id === 1 ? photoUrl : '')
}
function authorLetter(m) {
  return (m.author?.nickname || '?').slice(0, 1)
}
function myAvatar() {
  return avatarUrl.value || photoUrl
}
</script>

<template>
  <section class="section moments-page">
    <div class="section-inner">
      <h2 class="section-title" v-reveal><span class="title-bar"></span> 说说 <span class="page-sub">Moments</span></h2>
      <p class="page-desc">随手记下的片段：一句话、一张图，不进文章列表。</p>

      <!-- 发布框：登录后可见 -->
      <div v-if="isLoggedIn" class="composer" v-reveal>
        <div class="composer-head">
          <span class="avatar" style="--avatar-size: 40px">
            <img :src="myAvatar()" alt="" />
          </span>
          <textarea
            v-model="draft"
            class="composer-input"
            rows="3"
            :maxlength="MAX_CHARS"
            placeholder="现在在想什么？"
          ></textarea>
        </div>

        <div v-if="picked.length" class="composer-thumbs">
          <div v-for="(p, i) in picked" :key="i" class="thumb">
            <img v-if="p.url" :src="p.url" alt="" />
            <span v-else class="thumb-loading">上传中…</span>
            <button class="thumb-del" title="移除" @click="removeImage(i)">×</button>
          </div>
        </div>

        <div class="composer-foot">
          <button class="composer-pick" :disabled="picked.length >= MAX_IMAGES" @click="pickImages">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="16" rx="3"/><circle cx="8.5" cy="9.5" r="1.5"/><path d="M21 16l-5-5-6 6"/></svg>
            图片<span v-if="picked.length" class="pick-n">{{ picked.length }}/{{ MAX_IMAGES }}</span>
          </button>
          <input ref="fileInput" type="file" accept="image/*" multiple hidden @change="onFiles" />
          <span class="composer-count" :class="{ warn: leftChars < 50 }">{{ draft.length }}/{{ MAX_CHARS }}</span>
          <button class="composer-send" :disabled="!canPost" @click="publish">
            {{ posting ? '发布中…' : '发布' }}
          </button>
        </div>
      </div>

      <!-- 列表 -->
      <div v-if="loading" class="moments-state">加载中…</div>
      <div v-else-if="error" class="moments-state is-error">{{ error }}</div>
      <div v-else-if="!items.length" class="moments-state">还没有说说，来发第一条吧。</div>

      <div v-else class="moments-list">
        <article v-for="m in items" :id="'m-' + m.id" :key="m.id" class="moment" v-reveal>
          <header class="m-head">
            <router-link v-if="m.author?.id" class="avatar" style="--avatar-size: 38px" :to="`/u/${m.author.id}`" :title="`看 ${m.author.nickname || 'TA'} 的主页`">
              <img v-if="authorAvatar(m)" :src="authorAvatar(m)" alt="" />
              <span v-else class="avatar-letter">{{ authorLetter(m) }}</span>
            </router-link>
            <span v-else class="avatar" style="--avatar-size: 38px">
              <img v-if="authorAvatar(m)" :src="authorAvatar(m)" alt="" />
              <span v-else class="avatar-letter">{{ authorLetter(m) }}</span>
            </span>
            <div class="m-who">
              <router-link v-if="m.author?.id" class="m-name" :to="`/u/${m.author.id}`">
                {{ m.author?.nickname || '匿名' }}
                <i v-if="m.author?.role === 'admin'" class="badge-admin">站长</i>
              </router-link>
              <span v-else class="m-name">
                {{ m.author?.nickname || '匿名' }}
                <i v-if="m.author?.role === 'admin'" class="badge-admin">站长</i>
              </span>
              <span class="m-time">{{ timeAgo(m.createdAt) }}</span>
            </div>
            <button v-if="canDelete(m)" class="m-del" title="删除这条说说" @click="remove(m)">删除</button>
          </header>

          <p v-if="m.content" class="m-text">{{ m.content }}</p>

          <div v-if="m.images && m.images.length" class="m-media" :class="'n' + Math.min(m.images.length, 5)">
            <img v-for="(src, i) in m.images" :key="i" :src="src" alt="" loading="lazy" />
          </div>

          <footer class="m-actions">
            <span class="m-stat" title="浏览量">
              <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6-10-6-10-6z"/><circle cx="12" cy="12" r="2.5"/></svg>
              {{ m.views }}
            </span>
            <button class="m-act m-like" :class="{ on: m.liked }" :disabled="liking[m.id]" @click="toggleLike(m)">
              <svg viewBox="0 0 24 24" width="15" height="15" :fill="m.liked ? 'currentColor' : 'none'" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1-1.1a5.5 5.5 0 0 0-7.8 7.8l8.8 8.8 8.8-8.8a5.5 5.5 0 0 0 0-7.8z"/></svg>
              {{ m.likes }}
            </button>
            <button class="m-act m-comment" :class="{ on: openComments[m.id] }" @click="toggleComments(m)">
              <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 11.5a8.4 8.4 0 0 1-9 8.4 9 9 0 0 1-3.8-.9L3 21l1.9-5a8.4 8.4 0 0 1-.9-3.5 8.5 8.5 0 0 1 8.5-8.5h.5a8.4 8.4 0 0 1 8 8z"/></svg>
              {{ m.comments || 0 }}
            </button>
            <button class="m-act m-share" @click="share(m)">
              <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><path d="M8.6 13.5l6.8 4M15.4 6.5l-6.8 4"/></svg>
              分享
            </button>
          </footer>

          <!-- 评论：与文章页同一个组件，样式完全一致 -->
          <div v-if="openComments[m.id]" class="m-comments">
            <CommentSection :moment-id="m.id" />
          </div>
        </article>
      </div>

      <div v-if="hasMore && !loading" class="moments-more">
        <button class="post-more-btn" :disabled="loadingMore" @click="loadMore">
          {{ loadingMore ? '加载中…' : '加载更多' }}
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.moments-page { min-height: 70vh; }

/* 说说正文比文章短，列再窄一点：744px 下每行约 47 个汉字偏长，
   收到 660px 后大约 40 字，读起来更顺 */
.moments-page .section-inner { max-width: 660px; }

.page-sub {
  font-size: 0.55em;
  letter-spacing: 1px;
  color: var(--accent-ink);
  margin-left: 6px;
}
.page-desc { color: var(--text-dim); font-size: 14px; margin-bottom: 22px; }

/* ---------- 发布框 ---------- */
.composer {
  background: var(--card);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 16px 18px 14px;
  margin-bottom: 22px;
}
.composer-head { display: flex; gap: 12px; align-items: flex-start; }
.composer-input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  resize: vertical;
  font-family: inherit;
  font-size: 15px;
  line-height: 1.7;
  color: var(--text);
  background: transparent;
  padding: 6px 0;
}
.composer-input::placeholder { color: var(--text-dim); }

.composer-thumbs {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(86px, 1fr));
  gap: 8px;
  margin: 8px 0 4px;
}
.thumb {
  position: relative;
  aspect-ratio: 1;
  border-radius: 10px;
  overflow: hidden;
  background: var(--bg-alt);
  border: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: center;
}
.thumb img { width: 100%; height: 100%; object-fit: cover; }
.thumb-loading { font-size: 11.5px; color: var(--text-dim); }
.thumb-del {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 20px;
  height: 20px;
  border: none;
  border-radius: 50%;
  background: rgba(8, 12, 20, 0.55);
  color: #fff;
  font-size: 14px;
  line-height: 1;
  cursor: pointer;
}
.thumb-del:hover { background: rgba(229, 72, 77, 0.9); }

.composer-foot {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border);
}
.composer-pick {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: none;
  background: transparent;
  color: var(--text-dim);
  font-size: 13.5px;
  padding: 6px 10px;
  border-radius: 9px;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}
.composer-pick:hover:not(:disabled) { background: var(--accent-soft); color: var(--accent-ink); }
.composer-pick:disabled { opacity: 0.45; cursor: not-allowed; }
.pick-n { font-family: var(--font-mono); font-size: 11.5px; }
.composer-count {
  margin-left: auto;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-dim);
}
.composer-count.warn { color: #e5484d; }
.composer-send {
  border: none;
  border-radius: 10px;
  background: var(--accent-fill);
  color: var(--on-accent);
  font-size: 14px;
  font-weight: 600;
  padding: 9px 22px;
  cursor: pointer;
  transition: filter 0.18s ease;
}
.composer-send:hover:not(:disabled) { filter: brightness(1.1); }
.composer-send:disabled { background: var(--bg-alt); color: var(--text-dim); cursor: not-allowed; }

/* ---------- 列表 ---------- */
.moments-state {
  text-align: center;
  color: var(--text-dim);
  font-size: 14px;
  padding: 48px 12px;
}
.moments-state.is-error { color: #e5484d; }

.moments-list { display: flex; flex-direction: column; gap: 14px; }

.moment {
  background: var(--card);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 16px 18px 12px;
  scroll-margin-top: 84px;
}

.m-head { display: flex; align-items: center; gap: 10px; }
.m-who { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.m-name { font-size: 14px; font-weight: 700; color: var(--text); display: inline-flex; align-items: center; }
/* 作者的头像与昵称都通向个人主页 */
a.m-name:hover { color: var(--accent-ink); text-decoration: none; }
a.avatar { flex-shrink: 0; transition: box-shadow 0.18s ease, transform 0.18s ease; }
a.avatar:hover { transform: translateY(-1px); box-shadow: 0 0 0 2px var(--accent-soft); }
.m-time { font-size: 12px; color: var(--text-dim); font-family: var(--font-mono); }
.m-del {
  border: none;
  background: transparent;
  color: var(--text-dim);
  font-size: 12.5px;
  padding: 4px 8px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}
.m-del:hover { background: rgba(229, 72, 77, 0.1); color: #e5484d; }

.m-text {
  margin: 10px 0 0;
  font-size: 15px;
  line-height: 1.75;
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-word;
}

/* 图片网格：1 张大图，2/4 张两列，其余三列 */
.m-media { display: grid; gap: 6px; margin-top: 12px; }
.m-media img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 10px;
  background: var(--bg-alt);
  display: block;
}
.m-media.n1 { grid-template-columns: minmax(0, 1fr); max-width: 420px; }
.m-media.n1 img { max-height: 380px; }
.m-media.n2,
.m-media.n4 { grid-template-columns: repeat(2, minmax(0, 1fr)); max-width: 520px; }
.m-media.n2 img,
.m-media.n4 img { aspect-ratio: 1; }
.m-media.n3,
.m-media.n5 { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.m-media.n3 img,
.m-media.n5 img { aspect-ratio: 1; }

.m-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 14px;
  padding-top: 10px;
  border-top: 1px solid var(--border);
}
.m-stat,
.m-act {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border: none;
  background: transparent;
  color: var(--text-dim);
  font-size: 12.5px;
  font-family: var(--font-mono);
  padding: 6px 10px;
  border-radius: 9px;
}
.m-act { cursor: pointer; transition: background 0.18s ease, color 0.18s ease; }
.m-act:hover:not(:disabled) { background: var(--accent-soft); color: var(--accent-ink); }
.m-like.on { color: #e5484d; }
.m-like.on:hover { background: rgba(229, 72, 77, 0.1); color: #e5484d; }
.m-comment.on { color: var(--accent-ink); background: var(--accent-soft); }
.m-share { margin-left: 0; }

/* 展开的评论区：上方一条分隔线，内部样式全部来自 CommentSection 本身，
   这里只调整“嵌在卡片里”时的边距（组件默认是文章页那种整块间距）。 */
.m-comments {
  margin-top: 4px;
  border-top: 1px solid var(--border);
  padding-top: 4px;
}
.m-comments :deep(.comments) {
  margin-top: 8px;
  padding-top: 0;
  border-top: none;
}
.m-comments :deep(.comments-title) { font-size: 1.05rem; }

.moments-more { display: flex; justify-content: center; margin-top: 20px; }
.post-more-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: none;
  background: transparent;
  color: var(--accent-ink);
  padding: 10px 20px;
  border-radius: 10px;
  font-size: 14px;
  cursor: pointer;
  transition: background 0.18s ease;
}
.post-more-btn:hover:not(:disabled) { background: var(--accent-soft); }
.post-more-btn:disabled { opacity: 0.5; cursor: default; }

@media (max-width: 760px) {
  .composer { padding: 14px; }
  .moment { padding: 14px 14px 10px; }
  .m-media.n3,
  .m-media.n5 { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
