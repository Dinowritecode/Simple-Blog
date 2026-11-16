<script setup>
// ============================================================
// BlogPostView.vue —— 文章详情页
//
// 版式（不再用「一张圆角卡片套住整篇」的块状布局）：
//   ① 封面区：整幅「文章页背景图」，高度约视口 1/3；标题、分类、标签、元信息
//      左对齐压在图上（不居中），与导航栏内容区同一条左边缘。
//   ② 正文区：白底、通栏。正文列窄（680px）且居中于页面；
//      右侧留一列放「文章结构」目录，随滚动吸顶，点条目平滑跳到对应标题。
//   ③ 评论区放在正文列末尾 —— 它属于正文的延续，不单独成一个区域。
//
// 目录是在内容渲染完以后从 DOM 里扫 <h1>/<h2>/<h3> 生成的，
// 所以编辑器里写什么标题层级，页面右侧就有什么结构，不需要后端配合。
// ============================================================

import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import { formatDate } from '../utils/format'
import CommentSection from '../components/comment/CommentSection.vue'
import { useSite } from '../composables/useSite'
import { useToast } from '../composables/useToast'
import { highlightCode, normalizeLang } from '../utils/highlight'
// 站点头像（与首页首屏、左侧名片同一张）
import photoUrl from '../others/photo.jpg'

const { toast } = useToast()

const route = useRoute()
const router = useRouter()

// 返回上一页（不是固定回博客列表）。
// vue-router 4 会把来路写在 history.state.back 里：有值说明是站内跳转过来的，
// 直接 back() 回到读者原来的位置（列表页翻到第 2 页、搜索结果页都能回去）；
// 没有值说明是从外部链接 / 新标签直接打开的，这时才退回列表。
function goBack() {
  const back = window.history.state && window.history.state.back
  if (back) router.back()
  else router.push('/blog')
}
// 文章页背景图：与首页那张分开设置（后台「页面管理 → 文章页背景图」）
const { postBg, homeBg } = useSite()

const post = ref(null)
const loading = ref(true)
const error = ref('')

// 标签在后端是逗号分隔的字符串，这里拆成数组渲染（中英文逗号都支持）。
// 与分类同名的标签不重复显示 —— 封面上「博客搭建」出现两次（分类一次、标签一次）
// 看起来像出了 bug，去掉重复的那个。
const tagList = computed(() => {
  const cat = (post.value?.category || '').trim()
  return (post.value?.tags || '')
    .split(/[,，]/)
    .map((t) => t.trim())
    .filter((t) => t && t !== cat)
})

// ---------- 封面 ----------
// 优先级：作者为这篇文章上传的封面 → 后台设的「文章页背景图」 → **主页背景图**。
// 也就是：没单独传就用主页那张，传了就用作者自己的。
// 底部永远垫一层主题色渐变：图没加载出来（图床挂了 / 网络慢）时不会变成一块死黑，
// 文字仍然读得清 —— 和首页首屏那个「背景图失败」是同一类问题。
const coverUrl = computed(
  () => post.value?.cover || postBg.value || homeBg.value || ''
)
const coverStyle = computed(() => {
  const fallback =
    'linear-gradient(135deg, color-mix(in srgb, var(--accent) 42%, #0b1420), #0b1420 70%)'
  // 注意层的顺序：background-image 里**第一个在最上面**。
  // 图片必须排在渐变前面，否则不透明的渐变会把图整个盖掉（踩过）。
  return {
    backgroundImage: coverUrl.value ? `url("${coverUrl.value}"), ${fallback}` : fallback
  }
})

// ---------- 点赞 / 分享 ----------
const liked = ref(false)
const likeCount = ref(0)
const liking = ref(false)

async function doLike() {
  if (!post.value || liking.value) return
  liking.value = true
  try {
    // 注意：后端点赞接口按 **slug** 查文章（/api/posts/:slug/like），
    // 传 id 会查不到 → 报「文章不存在」。之前这里传的是 post.id。
    const data = await api.toggleLike(post.value.slug)
    likeCount.value = data.likes
    liked.value = !!data.liked
  } catch (e) {
    // 点赞失败不该把整篇文章切到错误页，弹个提示就行
    toast.error(e.message || '点赞失败，请稍后再试')
  } finally {
    liking.value = false
  }
}

const copied = ref(false)

async function doShare() {
  const url = window.location.href
  const title = post.value?.title || '凛冬RinEyece 的博客'
  if (navigator.share) {
    try {
      await navigator.share({ title, url })
    } catch {
      /* 用户取消分享 */
    }
    return
  }
  try {
    await navigator.clipboard.writeText(url)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch {
    toast.error('复制链接失败，请手动复制地址栏链接')
  }
}

// ---------- 文章结构（目录） ----------
const contentRef = ref(null)
const toc = ref([])
const activeId = ref('')
// 小屏目录面板的展开状态
const tocOpen = ref(false)

// 扫正文里的 h1/h2/h3，给没 id 的补一个，顺便记下层级
function buildToc() {
  const root = contentRef.value
  toc.value = []
  activeId.value = ''
  if (!root) return
  const list = []
  root.querySelectorAll('h1, h2, h3').forEach((el, i) => {
    const text = (el.textContent || '').trim()
    if (!text) return
    if (!el.id) el.id = `sec-${i}`
    list.push({ id: el.id, text, level: Number(el.tagName.slice(1)) })
  })
  toc.value = list
  updateActive()
}

function jump(id) {
  // 目标标题可能正被「长文缩略」藏着：先展开再滚，否则跳过去是一片空白
  if (collapsed.value) {
    expanded.value = true
    nextTick(() => scrollToId(id))
    return
  }
  scrollToId(id)
}

function scrollToId(id) {
  const el = document.getElementById(id)
  if (!el) return
  el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  activeId.value = id
}

// 小屏：跳完就把面板收起来，否则读者还得自己再关一次
function jumpMobile(id) {
  jump(id)
  tocOpen.value = false
}

// 滚动高亮：取「已经滚过阅读线上方」的最后一个标题作为当前节。
// 还没读到第一个标题时（开头那段引言）不高亮任何一条 —— 否则一进页面
// 第一条就是亮的，看起来像「已经读到这里」。
let ticking = false
function updateActive() {
  if (!toc.value.length) return
  // 阅读线放在 160px：跳转后标题落在 scroll-margin（84px）附近，
  // 展开长文时布局还会再抖一下，线太靠上会让高亮停在上一节。
  const line = 160
  let cur = ''
  for (const h of toc.value) {
    const el = document.getElementById(h.id)
    if (el && el.getBoundingClientRect().top <= line) cur = h.id
  }
  activeId.value = cur
}
function onScroll() {
  if (ticking) return
  ticking = true
  requestAnimationFrame(() => {
    ticking = false
    updateActive()
  })
}

// ---------- 代码块：行号 + 语法高亮 ----------
// 正文是 v-html 进来的，Vue 不会给里面加东西，所以渲染完再改 DOM：
//   ① 把 <pre> 包进一个 flex 容器，左边放行号列，右边放可横向滚动的代码；
//   ② 用 utils/highlight.js 把代码染上色。
// 行号用「一列等高的数字」而不是把代码按行切开 —— 切分会把跨行的 span 打断。
function enhanceCodeBlocks() {
  const root = contentRef.value
  if (!root) return

  root.querySelectorAll('pre').forEach((pre) => {
    if (pre.dataset.hl === '1') return
    const codeEl = pre.querySelector('code') || pre
    const raw = (codeEl.textContent || '').replace(/\s+$/, '')
    if (!raw) return

    // 语言：优先编辑器写的 language-xxx，没有就交给 highlight.js 自动识别
    const cls = `${codeEl.className || ''} ${pre.className || ''}`
    const m = /language-([\w+#-]+)/i.exec(cls)
    const lang = normalizeLang(m && m[1])

    codeEl.innerHTML = highlightCode(raw, lang)
    codeEl.classList.add('hljs')

    // 行号列：纯文本 + white-space: pre，行高与 pre 完全一致才对得齐
    const count = raw.split('\n').length
    const gutter = document.createElement('div')
    gutter.className = 'code-ln'
    gutter.setAttribute('aria-hidden', 'true')
    gutter.textContent = Array.from({ length: count }, (_, i) => i + 1).join('\n')

    const wrap = document.createElement('div')
    wrap.className = 'code-wrap'
    pre.parentNode.insertBefore(wrap, pre)
    wrap.appendChild(gutter)
    wrap.appendChild(pre)
    pre.dataset.hl = '1'
  })
}

// ---------- 长文缩略 ----------
// 正文超过这个高度（px）就先收成缩略，底部渐隐 + 「展开全文」。
// 1400 ≈ 两个屏幕多一点，短文章完全不触发。
const COLLAPSE_AT = 1400
const collapsible = ref(false)
const expanded = ref(false)
const collapsed = computed(() => collapsible.value && !expanded.value)

function measureCollapse() {
  const el = contentRef.value
  if (!el || collapsed.value) return // 收起来时量不到全文高度
  collapsible.value = el.scrollHeight > COLLAPSE_AT
}

function expandAll() {
  expanded.value = true
}

// ---------- 加载 ----------
async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.getPost(route.params.slug)
    post.value = data
    likeCount.value = data.likes
    liked.value = !!data.liked
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
  // 必须等 loading 关掉、正文真的进了 DOM 再扫标题 / 处理代码块。
  // （之前放在 try 里：那一刻 v-else-if="post" 还没渲染，contentRef 是空的，目录永远是空。）
  await nextTick()
  buildToc()
  enhanceCodeBlocks()
  measureCollapse()
  // 正文里的图片可能是加载完才撑开高度，稍后再量一次
  setTimeout(measureCollapse, 800)
}

onMounted(() => {
  load()
  window.addEventListener('scroll', onScroll, { passive: true })
  window.addEventListener('resize', onScroll)
})
onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScroll)
  window.removeEventListener('resize', onScroll)
})
// 同页切换文章时重新加载
watch(() => route.params.slug, load)
</script>

<template>
  <div class="post-page">
    <div v-if="loading" class="post-state">加载中…</div>
    <div v-else-if="error" class="post-state post-error">{{ error }}</div>

    <article v-else-if="post" class="post">
      <!-- ① 封面区：背景图 + 左对齐的标题信息 -->
      <header class="post-cover" :style="coverStyle">
        <!-- 返回：背景区左上角（让开吸顶导航栏的高度）。
             走浏览器历史回上一页；直接打开链接（没有站内历史）时才退回列表。 -->
        <button class="post-back" type="button" @click="goBack">
          <i class="ico ico-back" aria-hidden="true"></i>
          <span>返回</span>
        </button>

        <div class="post-cover-inner">
          <!-- 阅读顺序：分类（kicker）→ 标题 → 作者 → 元信息 → 标签 -->
          <span v-if="post.category" class="post-cat">
            <i class="ico ico-cat"></i>{{ post.category }}
          </span>

          <h1 class="post-title">{{ post.title }}</h1>

          <!-- 作者：标题和日期之间。本站是单人博客，作者就是站长本人 -->
          <div class="post-author">
            <img class="post-author-avatar" :src="photoUrl" alt="" />
            <span class="post-author-name"><span class="accent">凛冬</span>RinEyce</span>
          </div>

          <!-- 浏览量 / 评论数照旧；点赞数搬到右侧那颗按钮上 -->
          <div class="post-meta">
            <span class="post-meta-item">{{ formatDate(post.createdAt) }}</span>
            <span class="post-meta-item"><i class="ico ico-views"></i>{{ post.views || 0 }}</span>
            <span class="post-meta-item"><i class="ico ico-comments"></i>{{ post.comments || 0 }}</span>
          </div>

          <div v-if="tagList.length" class="post-tags">
            <span v-for="t in tagList" :key="t" class="post-tag">#{{ t }}</span>
          </div>
        </div>

        <!-- ② 背景区右侧：点赞（含点赞数）+ 分享 -->
        <div class="cover-actions">
          <button
            class="cover-btn cover-like"
            :class="{ liked }"
            :aria-pressed="liked"
            :title="liked ? '取消点赞' : '点赞'"
            @click="doLike"
          >
            <svg viewBox="0 0 24 24" width="17" height="17" :fill="liked ? 'currentColor' : 'none'" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1-1.1a5.5 5.5 0 0 0-7.8 7.8l8.8 8.8 8.8-8.8a5.5 5.5 0 0 0 0-7.8z" />
            </svg>
            <span class="cover-num">{{ likeCount }}</span>
          </button>

          <button class="cover-btn cover-share" :title="copied ? '链接已复制' : '复制链接分享'" @click="doShare">
            <i class="ico ico-share" aria-hidden="true"></i>
            <span class="cover-share-text">{{ copied ? '已复制' : '分享' }}</span>
          </button>
        </div>
      </header>

      <!-- ② 正文区：白底通栏，中间窄列正文 + 右侧目录 -->
      <div class="post-body">
        <div class="post-layout">
          <div class="post-main">
            <!-- 小屏：文章结构收成一块可折叠面板（宽屏由右侧固定栏承担） -->
            <div v-if="toc.length" class="post-toc-mobile">
              <button
                class="toc-toggle"
                :aria-expanded="tocOpen"
                aria-controls="post-toc-list"
                @click="tocOpen = !tocOpen"
              >
                <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 6h16M8 12h12M8 18h12M4 12h.01M4 18h.01"/></svg>
                <span class="toc-toggle-text">文章结构</span>
                <span class="toc-toggle-n">{{ toc.length }}</span>
                <span class="toc-chev" :class="{ open: tocOpen }" aria-hidden="true">▾</span>
              </button>
              <nav v-show="tocOpen" id="post-toc-list" class="toc-mobile-list">
                <button
                  v-for="h in toc"
                  :key="h.id"
                  class="toc-item"
                  :class="['lv' + h.level, { active: activeId === h.id }]"
                  @click="jumpMobile(h.id)"
                >{{ h.text }}</button>
              </nav>
            </div>

            <!-- 正文：过长时收窄成缩略，点「展开全文」再看全文 -->
            <div
              ref="contentRef"
              class="post-content"
              :class="{ 'is-collapsed': collapsed }"
              v-html="post.content"
            ></div>

            <div v-if="collapsed" class="post-more">
              <button class="post-more-btn" @click="expandAll">
                <span>展开全文</span>
                <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 9l6 6 6-6"/></svg>
              </button>
            </div>

            <!-- 评论区（登录后发表 / 回复） -->
            <CommentSection :slug="post.slug" />
          </div>

          <!-- 右侧：文章结构 -->
          <aside v-if="toc.length" class="post-toc" aria-label="文章结构">
            <p class="toc-title">文章结构</p>
            <nav class="toc-list">
              <button
                v-for="h in toc"
                :key="h.id"
                class="toc-item"
                :class="['lv' + h.level, { active: activeId === h.id }]"
                :title="h.text"
                @click="jump(h.id)"
              >{{ h.text }}</button>
            </nav>
          </aside>
        </div>
      </div>
    </article>
  </div>
</template>

<style scoped>
.post-page { min-height: 100svh; }

.post-state {
  text-align: center;
  color: var(--text-dim);
  padding: 120px 24px;
}
.post-error { color: #e5484d; }

/* ---------- ① 封面 ----------
   高度约视口的三分之一（用 clamp 兜住极矮/极高的窗口）。
   文字压在图上，底部留一层由下往上的渐隐，只为压住标题背后的高光。 */
.post-cover {
  position: relative;
  display: flex;
  align-items: flex-end;
  /* 封面从导航栏底下开始（上移一个导航栏的高度），导航栏就压在图上 ——
     和首页一样：玻璃材质能透出被模糊的画面。
     上移之后可视高度会被导航栏吃掉 60px，min-height 要把它补回来：
     可视背景带 ≈ 36vh。
     底部那道 5px 主题色线，就是「背景区 / 文章区」的分界。 */
  margin-top: calc(var(--nav-h) * -1);
  min-height: calc(clamp(300px, 36vh, 480px) + var(--nav-h));
  border-bottom: 5px solid var(--accent);
  background-color: #14181f;
  background-size: cover, cover;
  /* 这条带子又宽又扁，垂直方向只能露出一小段；略偏上取景，
     人物/主体的脸和上半身才不会被裁在边上。 */
  background-position: center 32%;
  background-repeat: no-repeat;
  color: #fff;
}
/* 封面不压任何遮罩，背景图原样呈现（站长要求）。
   白字的可读性交给下面的文字阴影，图片亮度一点不动。 */

/* 与导航栏内容区同宽的容器：标题左边缘和导航栏左侧对齐 */
.post-cover-inner {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 1328px;
  margin: 0 auto;
  /* 文字块整体收一点，给上方的背景留出更多画面 */
  padding: 0 24px 26px;
}

.post-cat,
.post-tag {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12.5px;
  line-height: 1;
  border-radius: 6px;
  padding: 5px 10px;
  /* 压在图上：半透明白底 + 一圈半透明白边，亮图上也能看出边界 */
  color: #fff;
  background: rgba(255, 255, 255, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.26);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
}
/* 分类当 kicker 用：单独一行、贴着标题上方；尺寸收小一档 */
.post-cat { display: flex; width: fit-content; margin-bottom: 9px; font-size: 12px; padding: 4px 9px; }

/* 标签退到元信息之后：手机上一行放不下也不会把标题往下顶 */
.post-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
  max-width: 720px;
}
.post-tags .post-tag { font-size: 12px; padding: 4px 9px; }

.post-title {
  /* 标题收小一档：文字块占的高度少了，图上就多留出背景 */
  font-size: clamp(1.5rem, 3.2vw, 2.1rem);
  line-height: 1.24;
  margin-bottom: 10px;
  max-width: 720px;
  /* 标题用主题色；压在图上，靠阴影把边缘勾出来 */
  color: var(--accent);
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9), 0 3px 12px rgba(0, 0, 0, 0.7);
  text-wrap: pretty;
}

/* 作者一行：头像 + 昵称，压在封面图上，跟标题同一条左边缘 */
.post-author {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  font-size: 13.5px;
  font-weight: 600;
  color: #fff;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.6);
}
.post-author-avatar {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  object-fit: cover;
  flex-shrink: 0;
  border: 1.5px solid rgba(255, 255, 255, 0.8);
  background: rgba(255, 255, 255, 0.2);
}
/* 作者昵称整行都是白色，只有前面的「凛冬」跟标题一样用主题色以外的处理：
   作者名字不标蓝 —— 保持和正文信息同一种白，避免一屏跳出三种颜色。 */
.post-author-name .accent { color: #fff; }

.post-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  color: rgba(255, 255, 255, 0.92);
  font-family: var(--font-mono);
  font-size: 12.5px;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.65);
}
.post-meta-item { display: inline-flex; align-items: center; gap: 5px; }

/* 图标：mask 引用 src/others 下的 svg，颜色跟随文字 */
.ico {
  display: inline-block;
  width: 13px;
  height: 13px;
  flex-shrink: 0;
  background-color: currentColor;
  transition: transform 0.18s ease;
}
.ico-cat {
  -webkit-mask: url('../others/book.svg') center / contain no-repeat;
  mask: url('../others/book.svg') center / contain no-repeat;
}
.ico-views {
  -webkit-mask: url('../others/preview.svg') center / contain no-repeat;
  mask: url('../others/preview.svg') center / contain no-repeat;
}
.ico-likes {
  -webkit-mask: url('../others/thumbs-up.svg') center / contain no-repeat;
  mask: url('../others/thumbs-up.svg') center / contain no-repeat;
}
.ico-comments {
  -webkit-mask: url('../others/comment.svg') center / contain no-repeat;
  mask: url('../others/comment.svg') center / contain no-repeat;
}

/* ---------- ② 正文区 ----------
   白底通栏；三列栅格：左侧弹性留白 / 中间 680 正文 / 右侧弹性留白，
   目录放在右列里，所以正文列始终居于页面正中。 */
.post-body {
  background: var(--card);
  min-height: 60vh;
}

.post-layout {
  display: grid;
  grid-template-columns: 1fr minmax(0, 680px) 1fr;
  gap: 40px;
  max-width: 1328px;
  margin: 0 auto;
  /* 正文和右侧目录一起下移：离上面那条主题色分界线留出呼吸（26 → 52） */
  padding: 52px 24px 72px;
}

/* ---------- 返回（背景区左上角） ----------
   没有底色、没有描边：就是一段压在图上白色文字。
   可读性靠文字阴影（跟着字形走，不会在图上看出一块矩形）；
   悬停只做一件事 —— 文字（含箭头）变主题蓝。 */
.post-back {
  position: absolute;
  top: calc(var(--nav-h) + 14px);
  left: max(24px, calc((100% - 1328px) / 2 + 24px));
  z-index: 2;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 38px;
  padding: 0 14px 0 10px;
  border: none;
  border-radius: 10px;
  color: #fff;
  font-family: inherit;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  background: none;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.85), 0 2px 8px rgba(0, 0, 0, 0.6);
  transition: color 0.18s ease;
}
.post-back:hover {
  /* 悬停：只有文字颜色变化（箭头是 currentColor 的 mask，一起变蓝） */
  color: var(--accent);
  text-decoration: none;
}
/* 箭头图标用 src/others/arrow-left.svg 做 mask，颜色跟随文字（深色主题自动反白） */
.ico-back {
  width: 17px;
  height: 17px;
  -webkit-mask: url('../others/arrow-left.svg') center / contain no-repeat;
  mask: url('../others/arrow-left.svg') center / contain no-repeat;
}
.post-back:hover .ico-back { transform: translateX(-2px); }

/* 分享图标用 src/others/share-two.svg，同样走 mask，颜色跟文字 */
.ico-share {
  width: 16px;
  height: 16px;
  -webkit-mask: url('../others/share-two.svg') center / contain no-repeat;
  mask: url('../others/share-two.svg') center / contain no-repeat;
  transform: translateY(0.5px);
}

/* ---------- 背景区右侧：点赞（含点赞数）+ 分享 ----------
   两个按钮排在同一水平面上，靠右对齐。 */
.cover-actions {
  position: absolute;
  right: max(24px, calc((100% - 1328px) / 2 + 24px));
  bottom: 26px;
  z-index: 2;
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 10px;
}
.cover-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 38px;
  padding: 0 14px;
  border-radius: 999px;
  /* 无边框：同样靠底色和悬停加深表达可点 */
  border: none;
  background: rgba(8, 12, 20, 0.22);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  color: #fff;
  font-size: 13.5px;
  font-weight: 600;
  cursor: pointer;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.5);
  transition: background 0.18s ease, transform 0.18s ease;
}
.cover-btn:hover {
  background: rgba(8, 12, 20, 0.55);
  transform: translateY(-1px);
}
.cover-num { font-family: var(--font-mono); font-variant-numeric: tabular-nums; }
/* 已点赞：实心桃心 + 粉色，和全站唯一的暖色保持一致 */
.cover-like.liked {
  color: #ff8aa8;
  background: rgba(229, 72, 77, 0.28);
}

.post-main { grid-column: 2; min-width: 0; }

/* 正文排版 */
.post-content { font-size: 16.5px; line-height: 1.85; color: var(--text); }
.post-content :deep(> :first-child) { margin-top: 0; }

/* 标题层级：h2 和 h3 原来都渲染成 ~20px/700，读起来是一串加粗段落，
   右侧目录的两级缩进在正文里没有对应。现在拉开字号、字重和上下间距：
   h2 ≈ 24px/700、h3 ≈ 18.4px/600，层级在正文里就能看出来。 */
.post-content :deep(h1),
.post-content :deep(h2),
.post-content :deep(h3),
.post-content :deep(h4) {
  /* 跳转落点留出吸顶导航的高度，标题不会被导航栏盖住 */
  scroll-margin-top: 84px;
  line-height: 1.38;
}
.post-content :deep(h1) { font-size: 1.6rem; margin: 2.4em 0 0.6em; }
/* 一级小节只靠字号/字重/间距立层级。
   也给 h2 加过 3px 主题竖条（全站分节语法），但正文里 9 个小节每节一道竖条，
   会变成一排重复的色块，反而比标题本身更抢眼 —— 这里是阅读正文，不是首页分节。 */
.post-content :deep(h2) {
  font-size: 1.5rem;          /* 24px */
  font-weight: 700;
  margin: 2.5em 0 0.62em;
  position: relative;
}
.post-content :deep(h3) {
  font-size: 1.15rem;         /* 18.4px */
  font-weight: 600;
  margin: 1.9em 0 0.5em;
}
.post-content :deep(h4) { font-size: 1rem; font-weight: 600; margin: 1.4em 0 0.5em; }
.post-content :deep(p) { margin: 0.9em 0; }
.post-content :deep(ul),
.post-content :deep(ol) { margin: 0.9em 0 0.9em 1.4em; }
.post-content :deep(li) { margin: 0.35em 0; }
.post-content :deep(img) { max-width: 100%; height: auto; border-radius: 10px; margin: 1.2em 0; }
.post-content :deep(blockquote) {
  margin: 1.2em 0;
  padding: 2px 0 2px 16px;
  border-left: 3px solid var(--accent);
  color: var(--text-dim);
}
/* ---------- 代码块：行号 + 高亮 ----------
   外面一层 flex：左行号（不随横向滚动移动）、右代码（可横向滚动）。
   两边字号 / 行高必须完全一致，行号才对得齐。 */
.post-content :deep(.code-wrap) {
  display: flex;
  margin: 1.3em 0;
  background: var(--code-surface);
  border: 1px solid var(--code-border);
  border-radius: 10px;
  overflow: hidden;
}
.post-content :deep(.code-ln) {
  flex: 0 0 auto;
  /* 留够三位数的宽度，行号不会贴到分隔线上 */
  min-width: 52px;
  padding: 14px 14px 14px 16px;
  text-align: right;
  font-family: var(--font-mono);
  font-size: 13.5px;
  line-height: 1.7;
  color: var(--text-dim);
  opacity: 0.7;
  white-space: pre;
  user-select: none;
  border-right: 1px solid var(--code-border);
}
.post-content :deep(pre) {
  flex: 1;
  min-width: 0;
  margin: 0;
  padding: 14px 16px;
  background: none;
  border: none;
  border-radius: 0;
  overflow-x: auto;
  font-size: 13.5px;
  line-height: 1.7;
}
/* 没有语言、没被处理的 pre 仍然保持原来的独立块样式 */
.post-content :deep(pre:not([data-hl])) {
  background: var(--code-surface);
  border: 1px solid var(--code-border);
  border-radius: 10px;
  margin: 1.2em 0;
}
.post-content :deep(code) {
  font-family: var(--font-mono);
  font-size: 0.9em;
  background: var(--accent-soft);
  color: var(--accent-ink);
  padding: 0.15em 0.45em;
  border-radius: 6px;
}
.post-content :deep(pre code) { background: none; color: inherit; padding: 0; }

/* 语法着色：highlight.js 只负责加 class，颜色统一走 --code-* 变量，
   深色主题整套换掉，不用再引一份 hljs 主题 CSS 和它打架。 */
.post-content :deep(.hljs-comment),
.post-content :deep(.hljs-quote) { color: var(--code-com); font-style: italic; }
.post-content :deep(.hljs-string),
.post-content :deep(.hljs-regexp),
.post-content :deep(.hljs-addition),
.post-content :deep(.hljs-template-tag),
.post-content :deep(.hljs-template-variable) { color: var(--code-str); }
.post-content :deep(.hljs-number),
.post-content :deep(.hljs-symbol),
.post-content :deep(.hljs-bullet),
.post-content :deep(.hljs-link) { color: var(--code-num); }
.post-content :deep(.hljs-keyword),
.post-content :deep(.hljs-selector-tag),
.post-content :deep(.hljs-literal),
.post-content :deep(.hljs-doctag) { color: var(--code-kw); }
.post-content :deep(.hljs-title),
.post-content :deep(.hljs-title.function_),
.post-content :deep(.hljs-section) { color: var(--code-fn); }
.post-content :deep(.hljs-type),
.post-content :deep(.hljs-built_in),
.post-content :deep(.hljs-title.class_),
.post-content :deep(.hljs-class .hljs-title) { color: var(--code-ty); }
.post-content :deep(.hljs-meta),
.post-content :deep(.hljs-attribute),
.post-content :deep(.hljs-attr) { color: var(--code-meta); }
.post-content :deep(.hljs-deletion) { color: #e5484d; }
.post-content :deep(.hljs-emphasis) { font-style: italic; }
.post-content :deep(.hljs-strong) { font-weight: 700; }
.post-content :deep(table) { width: 100%; border-collapse: collapse; margin: 1.2em 0; font-size: 14px; }
.post-content :deep(th),
.post-content :deep(td) { border: 1px solid var(--border); padding: 8px 12px; text-align: left; }
.post-content :deep(th) { background: var(--bg-alt); }

/* 进入动画：封面文字先落位，正文随后跟上。
   用纯 CSS keyframes（不依赖 IntersectionObserver）—— 正文不能因为观察器没触发就看不见。
   三处基本动效：进入（这里）、悬停（按钮 / 目录项已有）、状态切换（目录高亮 + 平滑跳转）。 */
.post-cover-inner { animation: cover-in 0.5s cubic-bezier(0.22, 1, 0.36, 1) both; }
@keyframes cover-in {
  from { opacity: 0; transform: translateY(12px); }
  to { opacity: 1; transform: none; }
}

.post-main { animation: body-in 0.5s 0.08s cubic-bezier(0.22, 1, 0.36, 1) both; }
@keyframes body-in {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: none; }
}

/* 系统开启「减少动效」时全部关掉，内容直接呈现 */
@media (prefers-reduced-motion: reduce) {
  .post-cover-inner,
  .post-main { animation: none; }
}

/* ---------- 长文缩略 ----------
   正文超过 COLLAPSE_AT 就收起来：只留上面一段，底部用 mask 渐隐，
   下面接一个「展开全文」。渐隐用 mask 而不是盖一层白 —— 深色主题下自动对。 */
.post-content.is-collapsed {
  max-height: 1400px;
  overflow: hidden;
  -webkit-mask-image: linear-gradient(180deg, #000 84%, transparent 100%);
  mask-image: linear-gradient(180deg, #000 84%, transparent 100%);
}

.post-more {
  display: flex;
  justify-content: center;
  margin-top: 22px;
}
/* 无边框：正文白底上只留文字，悬停时给一层淡主题色底 */
.post-more-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: none;
  background: transparent;
  color: var(--accent-ink);
  padding: 10px 18px;
  border-radius: 10px;
  transition: background 0.18s ease, transform 0.18s ease;
}
.post-more-btn:hover {
  background: var(--accent-soft);
  color: var(--accent-ink);
  transform: translateY(-1px);
}
.post-more-btn:hover svg { transform: translateY(2px); }
.post-more-btn svg { transition: transform 0.18s ease; }

/* ---------- ③ 右侧文章结构 ----------
   吸顶在右列，跟随正文滚动。 */
.post-toc {
  grid-column: 3;
  align-self: start;
  position: sticky;
  top: 84px;
  /* 右列是弹性宽度，别让目录把整列撑满：给自己一个上限，
     高亮底色才不会被拉成一条长条 */
  max-width: 280px;
  max-height: calc(100svh - 120px);
  overflow-y: auto;
  padding-left: 4px;
  border-left: 1px solid var(--border);
}

.toc-title {
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  color: var(--text-dim);
  margin-bottom: 8px;
  padding-left: 14px;
}

.toc-list { display: flex; flex-direction: column; gap: 1px; }

.toc-item {
  display: block;
  width: 100%;
  text-align: left;
  border: none;
  background: none;
  cursor: pointer;
  color: var(--text-dim);
  font-size: 13px;
  line-height: 1.5;
  /* 行高做到 32px：够鼠标点，也不至于把 9 项撑成一屏 */
  padding: 6px 10px 6px 14px;
  border-radius: 0 6px 6px 0;
  /* 长标题截断，完整内容放 title 里 */
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color 0.15s ease, background 0.15s ease;
}
.toc-item.lv1 { padding-left: 14px; font-weight: 600; }
.toc-item.lv2 { padding-left: 22px; }
/* 第二级缩进 16px：12px 字宽只有一个字，两级几乎看不出差别 */
.toc-item.lv3 { padding-left: 32px; font-size: 12.5px; }
.toc-item:hover { color: var(--accent-ink); background: color-mix(in srgb, var(--accent) 8%, transparent); }
/* 当前节：3px 主题条 + 主题色文字。
   底色用 color-mix 半透明主题色，而不是 --accent-soft（10% 在深色底上几乎是透明的），
   深浅两套主题同一套状态语言。 */
.toc-item.active {
  color: var(--accent-ink);
  background: color-mix(in srgb, var(--accent) 14%, transparent);
  box-shadow: inset 3px 0 0 var(--accent);
  font-weight: 600;
}

/* ---------- 小屏目录（折叠面板，宽屏隐藏） ---------- */
.post-toc-mobile { display: none; }

.toc-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 44px;
  padding: 0 14px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg);
  color: var(--text);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}
.toc-toggle-text { flex: 1; text-align: left; }
.toc-toggle-n {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-dim);
}
.toc-chev {
  color: var(--text-dim);
  transition: transform 0.2s ease;
}
.toc-chev.open { transform: rotate(180deg); }

.toc-mobile-list {
  display: flex;
  flex-direction: column;
  margin-top: 8px;
  border: 1px solid var(--border);
  border-radius: 12px;
  overflow: hidden;
}
/* 触控目标 44px */
.toc-mobile-list .toc-item {
  border-radius: 0;
  padding-top: 11px;
  padding-bottom: 11px;
  border-bottom: 1px solid var(--border);
}
.toc-mobile-list .toc-item:last-child { border-bottom: none; }
.toc-mobile-list .toc-item.active { box-shadow: inset 3px 0 0 var(--accent); }

/* ---------- 响应式 ----------
   ≤1180px 两侧留白不够放目录：右侧固定栏收起，改成正文上方的折叠面板，
   正文列回到页面正中单列。 */
@media (max-width: 1180px) {
  .post-layout {
    grid-template-columns: minmax(0, 680px);
    justify-content: center;
  }
  .post-main { grid-column: 1; }
  .post-toc { display: none; }
  .post-toc-mobile { display: block; margin-bottom: 28px; }
}

@media (max-width: 760px) {
  /* 手机上同样把导航栏吃掉的高度补回来。
     加了作者一行之后标题区多了一行，可视背景带从 33vh 放到 35vh，
     免得标签行贴到分界线上。 */
  .post-cover { min-height: calc(clamp(240px, 35vh, 380px) + var(--nav-h)); }
  .post-cover-inner { padding: 0 20px 24px; }
  .post-layout { padding: 36px 20px 48px; gap: 0; }
  .post-content { font-size: 16px; }
  .post-content.is-collapsed { max-height: 1100px; }

  /* 窄屏没有右侧留白：点赞 / 分享挪到封面右上角，和「返回」同一行，
     免得挤到底部的标签行旁边 */
  .post-back { top: calc(var(--nav-h) + 10px); left: 20px; padding: 0 12px 0 9px; }
  .cover-actions {
    right: 20px;
    top: calc(var(--nav-h) + 10px);
    bottom: auto;
    flex-direction: row;
    align-items: center;
    gap: 8px;
  }
  .cover-btn { min-height: 34px; padding: 0 11px; font-size: 12.5px; }
}
</style>
