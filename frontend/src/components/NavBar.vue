<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTheme } from '../composables/useTheme'
import { useAuth } from '../composables/useAuth'
import { useSite } from '../composables/useSite'
import UserMenu from './user/UserMenu.vue'

const route = useRoute()
const router = useRouter()
const { theme, toggle } = useTheme()
const { isLoggedIn } = useAuth()
const { homeBg } = useSite()
const menuOpen = ref(false)

// 站内页面链接（独立路由，不是首页锚点）
const pageLinks = [
  { to: { path: '/' }, label: '首页' },
  { to: { path: '/moments' }, label: '说说' }
]
// 首页区块锚点链接（博客列表已并入首页，所以「博客」也走页内锚点）
const sectionLinks = [
  { to: { path: '/', hash: '#blog' }, label: '博客' },
  { to: { path: '/', hash: '#about' }, label: '关于' },
  { to: { path: '/', hash: '#skills' }, label: '技能' },
  { to: { path: '/', hash: '#projects' }, label: '项目' },
  { to: { path: '/', hash: '#contact' }, label: '联系' }
]

// ---------- 当前页高亮 ----------
// 这套导航是「首页锚点 + 两个独立页」混在一起的：
//   · 大多数链接是首页的页内锚点，router-link 的 active-class 在 / 上会同时命中全部，
//     所以不能直接用；
//   · /blog（博客列表）和 /search（搜索）是真正独立的路由，读者到了那儿
//     导航里却没有任何提示，看不出自己在哪一页。
// 所以只在「真的换了一页」时高亮：首页及其各区块锚点都不画标记 ——
// 锚点只是滚动位置，不是另一个页面，多一道横线反而显得乱。
const activeLabel = computed(() => {
  const path = route.path
  if (path === '/blog' || path.startsWith('/blog/')) return '博客'
  if (path === '/moments') return '说说'
  if (path === '/search') return '搜索'
  return ''
})

// ---------- 悬停时滑动的蓝色短横线 ----------
// navRef：导航栏本身，横线是它的绝对定位子元素（bottom: 0 贴住导航栏底边），
//         所以纵向位置完全由 CSS 决定，JS 只需要算出横向偏移与宽度。
const navRef = ref(null)
const bar = reactive({ show: false, x: 0, w: 0 })
let hoveredEl = null // 记录当前悬停的链接，避免在同一链接内移动鼠标时反复触发

// 鼠标移到某个链接上：把横线滑到它下方。
// 用事件委托（mouseover 冒泡到容器），不必给每个链接单独绑事件。
function slideBar(e) {
  const a = e.target.closest('a')
  const nav = navRef.value
  if (!a || !nav || !nav.contains(a) || a === hoveredEl) return
  hoveredEl = a
  const navRect = nav.getBoundingClientRect()
  const r = a.getBoundingClientRect()
  bar.x = r.left - navRect.left // 相对导航栏左边缘
  bar.w = r.width
  bar.show = true
}

// 鼠标移出整个导航区：横线淡出
function hideBar() {
  hoveredEl = null
  bar.show = false
}

function closeMenu() {
  menuOpen.value = false
}

// ---------- 站内搜索入口 ----------
function goSearch() {
  closeMenu()
  router.push({ path: '/search' })
}

// 快捷键「/」：不在输入框里时按斜杠直接进搜索页（和 GitHub / 掘金一致的习惯）。
// 这是全局监听，所以必须先把「正在打字」的情况排除掉，否则评论框里打不出斜杠。
function onKeydown(e) {
  if (e.key !== '/' || e.metaKey || e.ctrlKey || e.altKey) return
  const el = document.activeElement
  const tag = (el?.tagName || '').toLowerCase()
  if (tag === 'input' || tag === 'textarea' || tag === 'select' || el?.isContentEditable) return
  e.preventDefault()
  goSearch()
}

// ---------- 导航栏随滚动自动切换 ----------
// measureGlass 同时算出两个状态，两者用同一个临界点，保证「背景」和「文字」永远配套：
//   onImage    ：导航栏还压在首页背景图上 → 文字转白（否则灰字会淹在照片里）
//   glassAtTop ：下方毛玻璃区域已经滑到顶部（hero 完全滚出视口）
//                · 没到顶部 → 保持「偏透明」的毛玻璃
//                · 到顶部之后 → 换成与内容块完全相同的奶白色磨砂
const onImage = ref(false)
const glassAtTop = ref(false)

// 文章详情页：封面从导航栏底下开始，所以导航栏先是一段「压在图上」的透明玻璃，
// 滚过封面之后再换成实底玻璃 —— 与首页同一套材质、同一套切换逻辑。
const isPostRoute = computed(() => /^\/blog\/[^/]+$/.test(route.path))

const solid = computed(() => {
  if (route.path === '/') return !!homeBg.value && glassAtTop.value
  if (isPostRoute.value) return glassAtTop.value
  return false
})

function measureGlass() {
  // 首页：hero（压在背景图上的首屏）是否已经滚出视口
  const hero = document.querySelector('.hero.has-bg') // 只有配置了背景图才会有这个类
  if (hero) {
    const atTop = hero.getBoundingClientRect().bottom <= 0
    glassAtTop.value = atTop
    onImage.value = !atTop
    return
  }

  // 文章页封面 / 个人主页封面：是否还压在导航栏下面（两者都是通栏大图）
  const cover = document.querySelector('.post-cover, .profile-cover')
  if (cover) {
    const bottom = cover.getBoundingClientRect().bottom
    glassAtTop.value = bottom <= 0
    onImage.value = bottom > 60
    return
  }

  // 其它页面（列表 / 搜索 / 后台）：默认实底玻璃，不压图
  glassAtTop.value = true
  onImage.value = false
}

onMounted(() => {
  window.addEventListener('scroll', measureGlass, { passive: true })
  window.addEventListener('resize', measureGlass)
  window.addEventListener('keydown', onKeydown)
  nextTick(measureGlass)
})
onBeforeUnmount(() => {
  window.removeEventListener('scroll', measureGlass)
  window.removeEventListener('resize', measureGlass)
  window.removeEventListener('keydown', onKeydown)
})
// 路由切换、或背景图异步加载完成（.has-bg 出现）后重新判定一次
watch(() => route.fullPath, () => nextTick(measureGlass))
watch(homeBg, () => nextTick(measureGlass))
</script>

<template>
  <header ref="navRef" class="nav" :class="{ 'nav-over-image': onImage, 'nav-solid': solid }">
    <div class="nav-inner">
      <router-link class="nav-brand" to="/" @click="closeMenu"><span class="accent">凛冬</span>RinEyce</router-link>

      <nav
        class="nav-links"
        :class="{ open: menuOpen }"
        @mouseover="slideBar"
        @mouseleave="hideBar"
      >
        <router-link
          v-for="link in pageLinks"
          :key="link.label"
          :to="link.to"
          :class="{ active: activeLabel === link.label }"
          @click="closeMenu"
        >{{ link.label }}</router-link>
        <router-link
          v-for="link in sectionLinks"
          :key="link.label"
          :to="link.to"
          :class="{ active: activeLabel === link.label }"
          @click="closeMenu"
        >{{ link.label }}</router-link>
        <!-- 小屏把搜索收进这个下拉菜单（桌面端用右上角的放大镜按钮） -->
        <router-link
          class="only-mobile"
          :class="{ active: activeLabel === '搜索' }"
          :to="{ path: '/search' }"
          @click="closeMenu"
        >搜索</router-link>
      </nav>

      <div class="nav-actions">
        <button class="icon-btn" aria-label="切换主题" title="切换主题" @click="toggle">
          <svg v-if="theme === 'light'" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
          <svg v-else viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"/></svg>
        </button>

        <!-- 站内搜索：桌面端放在主题切换旁边（小屏收纳进汉堡菜单，见 .only-mobile） -->
        <button class="icon-btn search-btn" aria-label="搜索" title="站内搜索（快捷键 /）" @click="goSearch">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
        </button>

        <!-- 已登录：用户区（铃铛 + 头像菜单）；未登录：登录/注册按钮 -->
        <UserMenu v-if="isLoggedIn" />
        <template v-else>
          <router-link class="btn-nav" to="/login">登录</router-link>
          <router-link class="btn-nav btn-nav-primary" to="/register">注册</router-link>
        </template>

        <button
          class="icon-btn menu-btn"
          :aria-label="menuOpen ? '关闭菜单' : '打开菜单'"
          :aria-expanded="menuOpen"
          :title="menuOpen ? '关闭菜单' : '菜单'"
          @click="menuOpen = !menuOpen"
        >
          <!-- 打开后图标要变成叉，否则「点哪里关」没有提示 -->
          <svg v-if="!menuOpen" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 7h16M4 12h16M4 17h16"/></svg>
          <svg v-else viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M6 6l12 12M18 6 6 18"/></svg>
        </button>
      </div>
    </div>

    <!-- 悬停时滑到目标选项下方的蓝色短横线：
         贴在导航栏底边（CSS 里 bottom: 0），横向位置与宽度由 JS 按目标链接实测后内联 -->
    <span
      class="nav-bar"
      :class="{ show: bar.show }"
      :style="{ transform: `translateX(${bar.x}px)`, width: `${bar.w}px` }"
    ></span>
  </header>
</template>

<style scoped>
.nav {
  position: sticky;
  top: 0;
  z-index: 100;
  /* 接近透明的毛玻璃：几乎不掺主题底色（10%），主要靠背后的高斯模糊 + 适度提饱和，
     所以背后是背景图时能直接透出图片，只有一层很淡的磨砂感。 */
  background: color-mix(in srgb, var(--bg) 10%, transparent);
  /* 模糊与饱和度都压淡了一档：原来的雾面偏重、提饱和偏艳，压在照片上会显冲突 */
  backdrop-filter: blur(12px) saturate(150%);
  -webkit-backdrop-filter: blur(12px) saturate(150%);
  /* 刻意不加 border / box-shadow：任何一条浅色描边压在背景图上都会变成一条白边 */
}

/* 毛玻璃区滑到顶部后：导航栏换成「滚到顶」那一档材质。
   底色和模糊都用导航栏自己的变量 —— 默认值和卡片一致（看起来仍旧统一），
   但后台可以把导航栏单独调开。首屏压在背景图上的「偏透明」状态不受影响。 */
.nav.nav-solid {
  background: var(--glass-nav-bg);
  backdrop-filter: var(--glass-nav-blur);
  -webkit-backdrop-filter: var(--glass-nav-blur);
  border-bottom: 1px solid var(--glass-border);
}

/* 不支持 backdrop-filter 的浏览器：退回不透明背景，避免文字与背景内容糊在一起 */
@supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
  .nav { background: var(--bg); box-shadow: none; }
  .nav.nav-solid { background: var(--card); }
}

.nav-inner {
  /* 1280 = 首页三栏总宽（240 名片 + 28 + 744 正文 + 28 + 240 公告/统计），
     再加上两侧 24px padding = 1328，导航栏两端因此与首页三栏两端对齐 */
  max-width: 1328px;
  margin: 0 auto;
  padding: 0 24px;
  height: var(--nav-h);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.nav-brand {
  font-weight: 700;
  font-size: 17px;
  color: var(--text);
  letter-spacing: 0.5px;
}
.nav-brand:hover { text-decoration: none; }

/* 压在背景图上时品牌字转白；但前面的「凛冬」**始终**保持主题色，
   不跟着变白 —— 它是这个名字的品牌标记，任何底色下都认得出是同一个 LOGO。 */
.nav-over-image .nav-brand { color: #fff; text-shadow: 0 1px 3px rgba(0, 0, 0, 0.5); }

/* align-self: stretch 让整条链接区撑满导航栏高度，
   每条链接再各自撑满这一高度并在其中垂直居中，
   于是鼠标只要进入导航栏上下的这一整条，就已经算悬停在该选项上（不再只限文字） */
.nav-links { display: flex; gap: 32px; position: relative; align-self: stretch; }
.nav-links a {
  display: flex;
  align-items: center;
  color: var(--text-dim);
  font-size: 15px;
  transition: color 0.2s ease;
}
.nav-links a:hover { color: var(--accent-ink); text-decoration: none; } /* 悬停变主题蓝 */

/* 当前页：文字加深 + 一道主题色短下划线。
   文字不直接用品牌蓝（15px 的亮蓝在白底上偏弱），品牌色交给那道下划线表达。 */
.nav-links a { position: relative; }
.nav-links a.active { color: var(--text); font-weight: 600; }
.nav-links a.active::after {
  content: "";
  position: absolute;
  left: 0;
  right: 0;
  bottom: 16px;
  height: 2px;
  border-radius: 2px;
  background: var(--accent);
}
.nav-over-image .nav-links a.active { color: #fff; }
.nav-over-image .nav-links a.active::after { background: #fff; }

/* 导航栏还压在首页背景图上时：选项卡文字转白（灰字会淹在照片里）。
   阴影只是为了让白字在亮部也清楚，不改变文字本身的颜色。 */
.nav-over-image .nav-links a {
  color: #fff;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.45);
}
.nav-over-image .nav-links a:hover {
  color: var(--accent);
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.45);
}

/* 悬停时滑到目标选项下方的蓝色短横线。
   贴在导航栏最底边（bottom: 0），横向位置与宽度由 JS 按被悬停的链接实测后写成内联
   style，这里只负责「滑动」动画：transform 横向平移，width 让不同宽度的选项之间平滑过渡。 */
.nav-bar {
  position: absolute;
  left: 0;
  bottom: 0;
  width: 0;
  height: 4px;
  /* 直角，不要圆角 */
  border-radius: 0;
  background: var(--accent);
  opacity: 0;
  pointer-events: none;
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1),
              width 0.3s cubic-bezier(0.4, 0, 0.2, 1),
              opacity 0.2s ease;
}
.nav-bar.show { opacity: 1; }

.nav-actions { display: flex; align-items: center; gap: 8px; }

/* 登录 / 注册按钮（未登录时显示） */
.btn-nav {
  display: inline-flex;
  align-items: center;
  padding: 7px 16px;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  border: 1px solid transparent; /* 用透明边框占位，保留原来的盒模型尺寸 */
  transition: all 0.2s ease;
  white-space: nowrap;
}
.btn-nav:hover { border-color: var(--accent-ink); color: var(--accent-ink); text-decoration: none; }
.btn-nav-primary { background: var(--accent-fill); border-color: transparent; color: var(--on-accent); }
.btn-nav-primary:hover { filter: brightness(1.12); color: var(--on-accent); border-color: transparent; }

/* 压在背景图上时，未登录按钮也转白（注册按钮保持蓝底白字） */
.nav-over-image .btn-nav:not(.btn-nav-primary) {
  color: #fff;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.45);
}
.nav-over-image .btn-nav:not(.btn-nav-primary):hover {
  color: #fff;
  border-color: transparent;
  background: rgba(255, 255, 255, 0.16);
}

/* .icon-btn 的基础样式与「压在背景图上转白」都已移到全局 style.css，
   这样用户菜单里的通知铃铛也能用上（scoped 样式管不到别的组件）。 */

.menu-btn { display: none; }

/* 「小屏专用」元素：默认不显示，只在下面的媒体查询里放出来。
   选择器必须带上 .nav-links a 的前缀 —— 上面那条 `.nav-links a { display: flex }`
   权重更高，单写 .only-mobile 会被它盖掉（踩过一次：桌面端也冒出了「搜索」）。 */
.nav-links a.only-mobile { display: none; }

@media (max-width: 760px) {
  .menu-btn { display: inline-flex; }

  /* 小屏动作区太挤（主题 + 桃信 + 铃铛 + 头像 + 汉堡），搜索改走下拉菜单 */
  .nav-actions .search-btn { display: none; }
  .nav-links a.only-mobile { display: flex; }

  /* 移动端下拉菜单是整行带分隔线的形式，用不上滑动横线 */
  .nav-bar { display: none; }

  /* 移动端下拉菜单：实底 + 下方圆角 + 投影。
     用 --card（浅色 #fff / 深色 #171c26）而不是 --bg —— 面板要和页面底色分开，
     否则菜单和背后内容同色，边界看不出来。 */
  .nav-links {
    position: fixed;
    inset: var(--nav-h) 0 auto 0;
    flex-direction: column;
    gap: 0;
    background: var(--card);
    border-bottom: 1px solid var(--border);
    border-radius: 0 0 14px 14px;
    box-shadow: 0 16px 40px rgba(15, 23, 42, 0.14);
    padding: 8px 24px 16px;
    transform: translateY(-12px);
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.25s ease, transform 0.25s ease;
  }
  .nav-links.open { opacity: 1; transform: none; pointer-events: auto; }
  .nav-links a { padding: 12px 0; border-bottom: 1px solid var(--border); }
  .nav-links a:last-child { border-bottom: none; }
}
</style>
