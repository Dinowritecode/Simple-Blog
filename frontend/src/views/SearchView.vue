<script setup>
// ============================================================
// SearchView.vue —— 站内搜索页（/search?q=关键字&type=all）
//
// 一次搜索覆盖四类内容：文章 / 分类 / 标签 / 用户。
//   · 「全部」标签页：每类先露 6 条，带「查看全部 N 条 →」跳到对应标签页
//   · 「文章」标签页：走 /api/posts?q= 分页（口径和后端 /api/search 完全一致）
//   · 「分类 / 标签」标签页：点条目直接跳到 /blog?category= / ?tag= 看该类文章
//   · 「用户」标签页：仅登录后可用（后端 usersLocked 控制），点了打开 Momo Talk 资料卡
//
// 关键字与标签页都同步到地址栏（history.replaceState 级别），所以结果可以直接分享 / 收藏。
// 输入是「边打边搜」：停止输入 260ms 后才发请求，避免每敲一个字打一次接口。
// ============================================================

import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import PostCard from '../components/PostCard.vue'
import LevelHeart from '../components/messenger/LevelHeart.vue'
import { useAuth } from '../composables/useAuth'
import { useMessenger } from '../composables/useMessenger'
import { useSite } from '../composables/useSite'

const route = useRoute()
const router = useRouter()
const { isLoggedIn } = useAuth()
const { momoTalkEnabled } = useSite()
const { open: openMessenger } = useMessenger()

// 标签页定义。key 会写进 URL 的 type 参数，顺便也是后端结果里的字段名。
const TABS = [
  { key: 'all', label: '全部' },
  { key: 'post', label: '文章' },
  { key: 'category', label: '分类' },
  { key: 'tag', label: '标签' },
  { key: 'user', label: '用户' }
]
const PREVIEW = 6 // 「全部」标签页里每类先露几条
const EMPTY_RESULT = {
  posts: [], categories: [], tags: [], users: [],
  totals: { posts: 0, categories: 0, tags: 0, users: 0 },
  usersLocked: false
}

const inputRef = ref(null)
const keyword = ref('') // 输入框里的即时内容（可能还没提交）
const q = ref('') // 已提交、正在展示结果的关键字
const tab = ref('all')

const res = ref({ ...EMPTY_RESULT })
const loading = ref(false)
const error = ref('')
const searched = ref(false) // 是否已经搜过一次（用来区分「还没搜」和「搜了但没结果」）

// 没输关键字时的引导内容：分类 + 标签 + 最新文章
const guide = ref({ categories: [], tags: [], posts: [] })

// 「文章」标签页自己的分页（数据源是 /api/posts，和综合搜索是两个接口）
const pageSize = 9
const postItems = ref([])
const postTotal = ref(0)
const postPage = ref(1)
const postLoading = ref(false)
const postPages = computed(() => Math.max(1, Math.ceil(postTotal.value / pageSize)))

// 输入防抖定时器
let timer = null

// ---------- 从地址栏读状态 ----------
// syncInput：是否把地址栏里的关键字回填到输入框。
// 输入框聚焦时**不能**回填 —— 防抖请求返回、路由更新会触发这里，
// 如果此时用户还在打字，回填会把光标和刚敲的字一起吃回去。
function readRoute(syncInput = true) {
  const raw = route.query.q
  const next = (Array.isArray(raw) ? raw[0] : raw) || ''
  const type = route.query.type
  const t = Array.isArray(type) ? type[0] : type
  q.value = next
  if (syncInput) keyword.value = next
  tab.value = TABS.some((x) => x.key === t) ? t : 'all'
}

// ---------- 搜索 ----------
async function runSearch() {
  if (!q.value.trim()) {
    res.value = { ...EMPTY_RESULT }
    searched.value = false
    return
  }
  loading.value = true
  error.value = ''
  try {
    // limit 取 30：切换标签页时不用重新请求，「全部」页只截前 6 条展示
    res.value = await api.search(q.value, { limit: 30 })
    searched.value = true
  } catch (e) {
    error.value = e.message
    res.value = { ...EMPTY_RESULT }
  } finally {
    loading.value = false
  }
}

// 「文章」标签页：单独分页拉取
async function loadPosts() {
  if (tab.value !== 'post' || !q.value.trim()) return
  postLoading.value = true
  try {
    const d = await api.listPosts({ q: q.value, page: postPage.value, pageSize })
    postItems.value = d.items || []
    postTotal.value = d.total || 0
  } catch (e) {
    error.value = e.message
    postItems.value = []
    postTotal.value = 0
  } finally {
    postLoading.value = false
  }
}

// 没输关键字时，拉一份「随便看看」的引导数据（只拉一次）
let guideLoaded = false
async function loadGuide() {
  if (guideLoaded) return
  guideLoaded = true
  try {
    const [cats, tags, posts] = await Promise.all([
      api.listCategories(),
      api.listTags(),
      api.listPosts({ page: 1, pageSize: 6 })
    ])
    guide.value = {
      categories: (cats.items || []).slice(0, 12),
      tags: (tags.items || []).slice(0, 20),
      posts: posts.items || []
    }
  } catch {
    /* 引导内容拿不到就留空，不影响搜索本身 */
  }
}

// ---------- 输入 → 地址栏 → 结果 ----------
// 输入框每变一次就重置防抖；停 260ms 才把关键字写进地址栏。
// 用 replace 而不是 push：边打边搜不该在浏览器历史里留下一串中间状态。
function onInput() {
  clearTimeout(timer)
  timer = setTimeout(() => {
    const next = keyword.value.trim()
    if (next === q.value) return // 关键字没变，地址栏不用动
    // 关键字变了：带着当前标签页重新搜（停在「文章」页继续改词时不该被弹回「全部」）
    const query = next ? { q: next } : {}
    if (next && tab.value !== 'all') query.type = tab.value
    router.replace({ path: '/search', query })
  }, 260)
}

// 回车：立刻搜（push，让「后退」能回到上一个关键字）
function submit() {
  clearTimeout(timer)
  const next = keyword.value.trim()
  router.push({ path: '/search', query: next ? { q: next } : {} })
  inputRef.value?.blur()
}

function clearKeyword() {
  clearTimeout(timer)
  keyword.value = ''
  router.replace({ path: '/search' })
  inputRef.value?.focus()
}

function switchTab(key) {
  if (tab.value === key) return
  tab.value = key
  const query = { ...route.query, type: key === 'all' ? undefined : key }
  router.replace({ path: '/search', query })
}

// 点分类 / 标签：跳到博客列表页按该分类 / 标签筛选
function goCategory(name) {
  router.push({ path: '/blog', query: { category: name } })
}
function goTag(name) {
  router.push({ path: '/blog', query: { tag: name } })
}

// 点用户：打开 Momo Talk 资料卡（Momo Talk 关掉时不可点，行会退化成纯展示）
function openUser(u) {
  if (!momoTalkEnabled.value || !isLoggedIn.value) return
  openMessenger({ userId: u.userId })
}
const userClickable = computed(() => momoTalkEnabled.value && isLoggedIn.value)

// ---------- 生命周期 ----------
onMounted(() => {
  readRoute()
  if (q.value) runSearch()
  else loadGuide()
  // 进页面就聚焦输入框（移动端不抢焦点，避免一进来就弹键盘）
  if (window.innerWidth > 760) inputRef.value?.focus()
  window.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  clearTimeout(timer)
  window.removeEventListener('keydown', onKeydown)
})

// Esc：清空并收起键盘
function onKeydown(e) {
  if (e.key === 'Escape' && document.activeElement === inputRef.value) {
    if (keyword.value) clearKeyword()
    else inputRef.value?.blur()
  }
}

// 地址栏变化（含点标签页、点「查看全部」、防抖提交）→ 重新搜
watch(
  () => [route.query.q, route.query.type],
  () => {
    // 输入框还在焦点里就不要回填关键字，否则会打断正在进行的输入
    readRoute(document.activeElement !== inputRef.value)
    postPage.value = 1
    if (q.value) runSearch()
    else {
      res.value = { ...EMPTY_RESULT }
      searched.value = false
      loadGuide()
    }
    loadPosts()
  }
)

// 翻页
function gotoPage(p) {
  const target = Math.min(Math.max(1, p), postPages.value)
  if (target === postPage.value) return
  postPage.value = target
  loadPosts()
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

// ---------- 展示辅助 ----------
// 四类命中数之和
const totalHits = computed(() => {
  const t = res.value.totals || {}
  return (t.posts || 0) + (t.categories || 0) + (t.tags || 0) + (t.users || 0)
})

// 是否真的「什么都没搜到」。
// 注意：未登录时「用户」这一类是**被锁住**（后端根本没查），不是「没有结果」，
// 所以停在「用户」标签页时要让位给登录提示卡，不能显示「没找到」。
const nothingFound = computed(
  () => totalHits.value === 0 && !(res.value.usersLocked && tab.value === 'user')
)

// 「用户」被锁、但文章/分类/标签里有结果时，在「全部」页末尾补一张小提示卡
const showLockedHintInAll = computed(
  () =>
    res.value.usersLocked &&
    !!((res.value.totals || {}).posts || (res.value.totals || {}).categories || (res.value.totals || {}).tags)
)

// 「全部」页里每类截断显示
const previewPosts = computed(() => (res.value.posts || []).slice(0, PREVIEW))
const previewCats = computed(() => (res.value.categories || []).slice(0, PREVIEW))
const previewTags = computed(() => (res.value.tags || []).slice(0, PREVIEW))
const previewUsers = computed(() => (res.value.users || []).slice(0, PREVIEW))

// 标签页上的计数（文章用分页接口的 total 更准，其余用综合搜索的 totals）
function tabCount(key) {
  const t = res.value.totals || {}
  if (key === 'post') return t.posts || 0
  if (key === 'category') return t.categories || 0
  if (key === 'tag') return t.tags || 0
  if (key === 'user') return t.users || 0
  return 0
}

// 用户行 / 头像用的展示名（后端已经做过「昵称为空退回邮箱前缀」的兜底）
function initial(name) {
  return (name || '?').trim()[0] || '?'
}
</script>

<template>
  <section class="section search-page">
    <div class="search-inner">
      <h1 class="search-title">搜索<span class="accent">Search</span></h1>
      <p class="search-sub">在文章、分类、标签和用户里找东西</p>

      <!-- 搜索框 -->
      <div class="sbox">
        <svg class="sbox-icon" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
        <input
          ref="inputRef"
          v-model="keyword"
          class="sbox-input"
          type="search"
          autocomplete="off"
          placeholder="搜索文章 / 分类 / 标签 / 用户…"
          aria-label="站内搜索"
          @input="onInput"
          @keydown.enter.prevent="submit"
        />
        <button v-if="keyword" class="sbox-clear" title="清空" aria-label="清空" @click="clearKeyword">✕</button>
        <button class="sbox-go" @click="submit">搜索</button>
      </div>

      <!-- 标签页：有搜索结果时才出现 -->
      <div v-if="q" class="stabs">
        <button
          v-for="t in TABS"
          :key="t.key"
          class="stab"
          :class="{ on: tab === t.key }"
          @click="switchTab(t.key)"
        >
          {{ t.label }}<span v-if="tabCount(t.key)" class="stab-n">{{ tabCount(t.key) }}</span>
        </button>
      </div>

      <div v-if="loading" class="sstate">搜索中…</div>
      <div v-else-if="error" class="sstate serror">{{ error }}</div>

      <!-- 搜了但什么都没有 -->
      <div v-else-if="q && searched && nothingFound" class="sstate">
        <p>没有找到和「{{ q }}」相关的内容，换个词试试～</p>
        <p v-if="res.usersLocked" class="sstate-sub">
          用户搜索只对登录用户开放，<router-link :to="{ path: '/login', query: { next: route.fullPath } }">登录</router-link> 后也许能找到人
        </p>
      </div>

      <template v-else-if="q && searched">
        <!-- ============ 全部 ============ -->
        <template v-if="tab === 'all'">
          <!-- 文章 -->
          <section v-if="previewPosts.length" class="sgroup">
            <header class="sgroup-head">
              <h2>文章<span class="sgroup-n">{{ res.totals.posts }}</span></h2>
              <button v-if="res.totals.posts > previewPosts.length" class="slink" @click="switchTab('post')">
                查看全部 {{ res.totals.posts }} 篇 →
              </button>
            </header>
            <div class="sgrid">
              <PostCard v-for="p in previewPosts" :key="p.id" :post="p" />
            </div>
          </section>

          <!-- 分类 -->
          <section v-if="previewCats.length" class="sgroup">
            <header class="sgroup-head">
              <h2>分类<span class="sgroup-n">{{ res.totals.categories }}</span></h2>
              <button v-if="res.totals.categories > previewCats.length" class="slink" @click="switchTab('category')">
                查看全部 →
              </button>
            </header>
            <div class="schips">
              <button v-for="c in previewCats" :key="c.name" class="schip" @click="goCategory(c.name)">
                {{ c.name }}<span class="schip-n">{{ c.count }}</span>
              </button>
            </div>
          </section>

          <!-- 标签 -->
          <section v-if="previewTags.length" class="sgroup">
            <header class="sgroup-head">
              <h2>标签<span class="sgroup-n">{{ res.totals.tags }}</span></h2>
              <button v-if="res.totals.tags > previewTags.length" class="slink" @click="switchTab('tag')">
                查看全部 →
              </button>
            </header>
            <div class="schips">
              <button v-for="t in previewTags" :key="t.name" class="schip tag" @click="goTag(t.name)">
                #{{ t.name }}<span class="schip-n">{{ t.count }}</span>
              </button>
            </div>
          </section>

          <!-- 用户 -->
          <section v-if="previewUsers.length" class="sgroup">
            <header class="sgroup-head">
              <h2>用户<span class="sgroup-n">{{ res.totals.users }}</span></h2>
              <button v-if="res.totals.users > previewUsers.length" class="slink" @click="switchTab('user')">
                查看全部 →
              </button>
            </header>
            <div class="susers">
              <button
                v-for="u in previewUsers"
                :key="u.id"
                class="suser"
                :class="{ flat: !userClickable }"
                :disabled="!userClickable"
                @click="openUser(u)"
              >
                <span class="avatar" style="--avatar-size: 38px">
                  <img v-if="u.avatar" :src="u.avatar" alt="" />
                  <span v-else class="avatar-letter">{{ initial(u.nickname) }}</span>
                </span>
                <span class="suser-body">
                  <span class="suser-name">
                    {{ u.nickname || '（未设置昵称）' }}
                    <i v-if="u.role === 'admin'" class="badge-admin">站长</i>
                  </span>
                  <span class="suser-id">Momo Talk ID：{{ u.userId }}</span>
                </span>
                <LevelHeart :level="u.level || 1" :size="28" />
              </button>
            </div>
          </section>

          <!-- 未登录：用户那一类被锁，末尾补一张小提示卡（有其它结果时才出现在这里） -->
          <section v-if="showLockedHintInAll" class="sgroup">
            <header class="sgroup-head"><h2>用户</h2></header>
            <div class="slock compact">
              <p class="slock-desc">用户搜索（昵称 / Momo Talk ID）只对登录用户开放。</p>
              <router-link class="btn btn-primary" :to="{ path: '/login', query: { next: route.fullPath } }">去登录</router-link>
            </div>
          </section>
        </template>

        <!-- ============ 文章（分页） ============ -->
        <template v-else-if="tab === 'post'">
          <div v-if="postLoading" class="sstate">加载中…</div>
          <div v-else-if="!postItems.length" class="sstate">没有匹配的文章</div>
          <template v-else>
            <div class="sgrid">
              <PostCard v-for="p in postItems" :key="p.id" :post="p" />
            </div>
            <div v-if="postPages > 1" class="spager">
              <button class="btn btn-ghost" :disabled="postPage <= 1" @click="gotoPage(postPage - 1)">← 上一页</button>
              <span class="spager-info">{{ postPage }} / {{ postPages }}</span>
              <button class="btn btn-ghost" :disabled="postPage >= postPages" @click="gotoPage(postPage + 1)">下一页 →</button>
            </div>
          </template>
        </template>

        <!-- ============ 分类 ============ -->
        <template v-else-if="tab === 'category'">
          <div v-if="!res.categories.length" class="sstate">没有匹配的分类</div>
          <div v-else class="svlist">
            <button v-for="c in res.categories" :key="c.name" class="svrow" @click="goCategory(c.name)">
              <span class="svrow-icon">
                <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 10.5 12 3l9 7.5" /><path d="M5 9.8V21h14V9.8" /></svg>
              </span>
              <span class="svrow-body">
                <span class="svrow-name">{{ c.name }}</span>
                <span class="svrow-desc">{{ c.count }} 篇文章</span>
              </span>
              <span class="svrow-go">→</span>
            </button>
          </div>
          <p v-if="res.totals.categories > res.categories.length" class="shint">
            只显示前 {{ res.categories.length }} 个分类，输入更具体的关键字可以缩小范围
          </p>
        </template>

        <!-- ============ 标签 ============ -->
        <template v-else-if="tab === 'tag'">
          <div v-if="!res.tags.length" class="sstate">没有匹配的标签</div>
          <div v-else class="schips big">
            <button v-for="t in res.tags" :key="t.name" class="schip tag" @click="goTag(t.name)">
              #{{ t.name }}<span class="schip-n">{{ t.count }}</span>
            </button>
          </div>
          <p v-if="res.totals.tags > res.tags.length" class="shint">
            只显示前 {{ res.tags.length }} 个标签，输入更具体的关键字可以缩小范围
          </p>
        </template>

        <!-- ============ 用户 ============ -->
        <template v-else-if="tab === 'user'">
          <!-- 未登录：后端不返回用户，这里给一个明确的入口而不是「没结果」 -->
          <div v-if="res.usersLocked" class="slock">
            <p class="slock-title">登录后可以搜索用户</p>
            <p class="slock-desc">昵称和 Momo Talk ID 的搜索只对登录用户开放，这样别人没法拿关键字把全站用户枚举一遍。</p>
            <router-link class="btn btn-primary" :to="{ path: '/login', query: { next: route.fullPath } }">去登录</router-link>
          </div>
          <template v-else>
            <div v-if="!res.users.length" class="sstate">没有匹配的用户</div>
            <div v-else class="svlist">
              <button
                v-for="u in res.users"
                :key="u.id"
                class="svrow"
                :disabled="!userClickable"
                @click="openUser(u)"
              >
                <span class="avatar" style="--avatar-size: 40px">
                  <img v-if="u.avatar" :src="u.avatar" alt="" />
                  <span v-else class="avatar-letter">{{ initial(u.nickname) }}</span>
                </span>
                <span class="svrow-body">
                  <span class="svrow-name">
                    {{ u.nickname || '（未设置昵称）' }}
                    <i v-if="u.role === 'admin'" class="badge-admin">站长</i>
                  </span>
                  <span class="svrow-desc">Momo Talk ID：{{ u.userId }}</span>
                </span>
                <LevelHeart :level="u.level || 1" :size="30" />
              </button>
            </div>
            <p v-if="!userClickable && res.users.length" class="shint">
              Momo Talk 当前未启用，用户列表只能浏览
            </p>
          </template>
        </template>
      </template>

      <!-- ============ 还没输关键字：随便看看 ============ -->
      <template v-else>
        <section v-if="guide.categories.length" class="sgroup">
          <header class="sgroup-head"><h2>按分类浏览</h2></header>
          <div class="schips">
            <button v-for="c in guide.categories" :key="c.name" class="schip" @click="goCategory(c.name)">
              {{ c.name }}<span class="schip-n">{{ c.count }}</span>
            </button>
          </div>
        </section>

        <section v-if="guide.tags.length" class="sgroup">
          <header class="sgroup-head"><h2>按标签浏览</h2></header>
          <div class="schips">
            <button v-for="t in guide.tags" :key="t.name" class="schip tag" @click="goTag(t.name)">
              #{{ t.name }}<span class="schip-n">{{ t.count }}</span>
            </button>
          </div>
        </section>

        <section v-if="guide.posts.length" class="sgroup">
          <header class="sgroup-head"><h2>最新文章</h2></header>
          <div class="sgrid">
            <PostCard v-for="p in guide.posts" :key="p.id" :post="p" />
          </div>
        </section>
      </template>
    </div>
  </section>
</template>

<style scoped>
/* 与博客列表 / 文章详情同一条阅读列：切换页面时正文左边缘不跳 */
.search-inner { max-width: var(--content-col); margin: 0 auto; }

.search-title {
  font-size: clamp(1.8rem, 4vw, 2.4rem);
  margin-bottom: 6px;
}
.search-title .accent { margin-left: 8px; }
.search-sub { color: var(--text-dim); margin-bottom: 22px; }

/* ---------- 搜索框 ---------- */
.sbox {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 6px 6px 14px;
  border-radius: 14px;
  background: var(--glass-solid-bg);
  backdrop-filter: var(--glass-solid-blur);
  -webkit-backdrop-filter: var(--glass-solid-blur);
  border: 1px solid var(--glass-border);
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}
.sbox:focus-within {
  /* 聚焦环用深一档的主题色：它要靠「看得见」起作用，亮蓝在白底上偏弱 */
  border-color: var(--accent-ink);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent-ink) 16%, transparent);
}
.sbox-icon { color: var(--text-dim); flex-shrink: 0; }

.sbox-input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  color: var(--text);
  font-size: 15px;
  font-family: inherit;
  padding: 10px 0;
}
/* 去掉浏览器给 type=search 加的原生清除按钮（我们自己画了一个） */
.sbox-input::-webkit-search-cancel-button { display: none; }
.sbox-input::placeholder { color: var(--text-dim); }

.sbox-clear {
  flex-shrink: 0;
  width: 26px;
  height: 26px;
  border: none;
  border-radius: 50%;
  background: color-mix(in srgb, var(--text) 10%, transparent);
  color: var(--text-dim);
  font-size: 12px;
  line-height: 1;
  cursor: pointer;
  transition: all 0.2s ease;
}
.sbox-clear:hover { background: color-mix(in srgb, var(--text) 18%, transparent); color: var(--text); }

.sbox-go {
  flex-shrink: 0;
  border: none;
  border-radius: 10px;
  padding: 9px 18px;
  font-size: 14px;
  font-weight: 600;
  color: var(--on-accent);
  background: var(--accent-fill);
  cursor: pointer;
  transition: filter 0.2s ease;
}
.sbox-go:hover { filter: brightness(1.12); }

/* ---------- 标签页 ---------- */
.stabs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 18px 0 4px;
}
.stab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: none;
  border-radius: 9px;
  padding: 8px 14px;
  font-size: 13.5px;
  color: var(--text-dim);
  background: color-mix(in srgb, var(--text) 8%, transparent);
  cursor: pointer;
  transition: all 0.2s ease;
}
.stab:hover { color: var(--accent-ink); }
.stab.on { color: var(--on-accent); background: var(--accent-fill); font-weight: 600; }
.stab-n {
  font-family: var(--font-mono);
  font-size: 11px;
  /* 不再压透明度：这是「有几条结果」，要读得出来；选中态本身是白字，已经够区分 */
}

/* ---------- 分组 ---------- */
.sgroup { margin-top: 30px; }
.sgroup-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.sgroup-head h2 { font-size: 1.05rem; }
.sgroup-n {
  margin-left: 8px;
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 400;
  color: var(--text-dim);
}
.slink {
  border: none;
  background: transparent;
  color: var(--accent-ink);
  font-size: 13px;
  cursor: pointer;
  padding: 2px 0;
  white-space: nowrap;
}
.slink:hover { text-decoration: underline; }

.sgrid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
}

/* ---------- 分类 / 标签 chip ---------- */
.schips { display: flex; flex-wrap: wrap; gap: 8px; }
.schip {
  display: inline-flex;
  align-items: center;
  border: none;
  border-radius: 9px;
  padding: 8px 13px;
  font-size: 13px;
  color: var(--text-dim);
  background: color-mix(in srgb, var(--text) 8%, transparent);
  cursor: pointer;
  transition: all 0.2s ease;
}
.schip:hover { color: var(--accent-ink); }
/* 标签结果沿用站内标签的皮肤（淡蓝底 + 主题色字），
   原来和「分类」共用灰色胶囊，同一套 #标签 在文章卡上是蓝的、在搜索结果里变灰的。 */
.schip.tag {
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--accent-ink);
  background: var(--accent-soft);
}
.schip.tag:hover { background: color-mix(in srgb, var(--accent) 18%, transparent); }
.schip-n {
  margin-left: 6px;
  font-family: var(--font-mono);
  font-size: 11px;
}
.schips.big .schip { padding: 10px 16px; font-size: 14px; }

/* ---------- 列表行（分类 / 用户） ---------- */
.svlist {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.svrow {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  text-align: left;
  border: 1px solid var(--glass-border);
  border-radius: 12px;
  padding: 10px 14px;
  background: var(--card-bg);
  backdrop-filter: var(--card-blur);
  -webkit-backdrop-filter: var(--card-blur);
  cursor: pointer;
  transition: all 0.2s ease;
  font-family: inherit;
  color: var(--text);
}
.svrow:hover:not(:disabled) { border-color: var(--accent); transform: translateY(-1px); }
.svrow:disabled { cursor: default; }

.svrow-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 10px;
  flex-shrink: 0;
  color: var(--accent-ink);
  background: var(--accent-soft);
}
.svrow-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.svrow-name {
  font-size: 14.5px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 6px;
}
.svrow-desc { font-size: 12.5px; color: var(--text-dim); font-family: var(--font-mono); }
.svrow-go { color: var(--text-dim); flex-shrink: 0; }

/* ---------- 用户卡片（「全部」页里的小块） ---------- */
.susers {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 10px;
}
.suser {
  display: flex;
  align-items: center;
  gap: 10px;
  text-align: left;
  border: 1px solid var(--glass-border);
  border-radius: 12px;
  padding: 10px 12px;
  background: var(--card-bg);
  backdrop-filter: var(--card-blur);
  -webkit-backdrop-filter: var(--card-blur);
  cursor: pointer;
  transition: all 0.2s ease;
  font-family: inherit;
  color: var(--text);
}
.suser:hover:not(:disabled):not(.flat) { border-color: var(--accent); transform: translateY(-1px); }
.suser.flat { cursor: default; }
.suser-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.suser-name {
  font-size: 14px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.suser-id { font-size: 11.5px; color: var(--text-dim); font-family: var(--font-mono); }

/* ---------- 状态 / 提示 ---------- */
.sstate {
  text-align: center;
  color: var(--text-dim);
  padding: 64px 0;
}
.serror { color: #e5484d; }
/* 「没找到」下面的补充说明（比如「用户搜索需要登录」） */
.sstate-sub {
  margin-top: 10px;
  font-size: 13px;
  line-height: 1.8;
}
.sstate-sub a { color: var(--accent-ink); }
.shint {
  margin-top: 14px;
  font-size: 12.5px;
  color: var(--text-dim);
}

/* 未登录时的用户搜索说明卡：
   原来的虚线框和整页的实底卡片不是同一种语法，这里改成一块实底浅色面板。 */
.slock {
  text-align: center;
  background: color-mix(in srgb, var(--text) 4%, transparent);
  border: none;
  border-radius: 12px;
  padding: 34px 24px;
}
.slock-title { font-size: 15px; font-weight: 600; margin-bottom: 8px; }
.slock-desc {
  font-size: 13px;
  color: var(--text-dim);
  max-width: 460px;
  margin: 0 auto 18px;
  line-height: 1.7;
}
/* 「全部」页末尾那张小提示卡：紧凑一点，别抢正文的视觉重量 */
.slock.compact { padding: 18px 20px; }
.slock.compact .slock-desc { margin-bottom: 12px; }

/* ---------- 分页 ---------- */
.spager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 20px;
  background: var(--glass-solid-bg);
  backdrop-filter: var(--glass-solid-blur);
  -webkit-backdrop-filter: var(--glass-solid-blur);
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  padding: 8px 18px;
  width: fit-content;
  margin: 36px auto 0;
}
.spager .btn:disabled { opacity: 0.4; cursor: not-allowed; transform: none; }
.spager-info { font-family: var(--font-mono); color: var(--text-dim); font-size: 14px; }

@media (max-width: 620px) {
  .sgroup-head { flex-direction: column; align-items: flex-start; gap: 4px; }
  .sbox { padding-left: 12px; }
  .sbox-go { padding: 9px 13px; }
}
</style>
