// ============================================================
// useSite.js —— 站点公开设置（GET /api/settings）
//   homeBg    ：首页全屏背景图（Hero 渲染用；导航栏也用它判断是否压在图上）
//   startedAt ：后端本次进程启动时间（毫秒时间戳），统计卡片用它算「本站已连续运行 …」
//   notice    ：首页公告纯文本（后台「设置」页编辑，最多 300 字）
//   pageSize  ：文章列表每页条数（后台「设置」页可改，默认 20）
//   外观定制  ：主题色 + 每个模块独立的「底色浓度」与「模糊半径」
// 模块级状态 + 只请求一次，多处共用同一份数据。
// ============================================================

import { ref } from 'vue'
import { api } from '../api'

export const DEFAULT_PAGE_SIZE = 20 // 与后端 defaultPageSize 保持一致

// 点击特效默认值：与后端 defaultClickFX() 保持一致（后端没返回该字段时就按这份走）
export const DEFAULT_CLICK_FX = {
  enabled: true, // 总开关
  clickEnabled: true, // 点击波纹 / 碎片
  trailEnabled: true, // 鼠标拖尾
  trailAlways: true, // true = 不按鼠标也拖尾；false = 按住左键才拖尾
  color: '', // 空串 = 跟随站点主题色
  scale: 1,
  opacity: 1,
  maxDpr: 1,
  backend: 'auto' // auto | webgl2 | canvas2d
}

// 与后端 settings.go 里的默认值保持一致
export const DEFAULT_THEME = {
  accent: '#00AAEE', // 主题色（蔚蓝）
  cardAlpha: 50, // 卡片底色浓度 %
  cardBlur: 18, // 卡片模糊 px
  postAlpha: 50, // 文章卡片 + 左右两栏卡片 底色浓度 %
  postBlur: 0, // 文章卡片 / 两栏卡片 模糊 px
  panelColor: '#FFFFFF', // 下方毛玻璃区域底色
  panelAlpha: 100, // 下方区域底色浓度 %（100 = 不透明）
  panelBlur: 0, // 下方毛玻璃区域模糊 px（保持不模糊）
  navAlpha: 55, // 导航栏（滚到顶）底色浓度 %
  navBlur: 18 // 导航栏（滚到顶）模糊 px
}

const homeBg = ref('') // 首页背景图（后台「首页背景图」）
const postBg = ref('') // 文章详情页背景图（后台「文章页背景图」，与首页各管各的）
// 首页文案（后台「首页文案」里改）。空串 = 用组件内置的默认值，老库不用迁移。
// heroNickname 用 | 分隔：前段走主题色，后段走正文色；没有 | 时整串走正文色。
const heroNickname = ref('')
const heroDesc = ref('')
const heroRoles = ref('') // 一行一条，给打字动画用
const heroAvatar = ref('') // 首屏头像 URL（空 = 内置头像）
const loginBg = ref('') // 登录页左半屏背景（空 = 主题色底）
// 站长信息（后端从 role='admin' 的账号取）：首页首屏头像/昵称以它为准
const ownerNickname = ref('')
const ownerAvatar = ref('')
const startedAt = ref(0) // 0 = 尚未取到（或后端版本较旧没返回该字段）
const notice = ref('')
const pageSize = ref(DEFAULT_PAGE_SIZE)
const theme = ref({ ...DEFAULT_THEME })
// Momo Talk 功能开关（后台「功能开关」里改）：false 时前端不渲染任何桃信入口
const momoTalkEnabled = ref(true)
// 全站字体标识：system | rouhei | round（后台「页面管理」里改）
const siteFont = ref('system')
// 点击特效（ba-click-fx）参数；字段名与后端 ClickFXSettings 一一对应
const clickFx = ref({ ...DEFAULT_CLICK_FX })
// 设置是否已经拿到（无论成功失败都会置 true）。
// 给「默认开启、但可能被后台关掉」的功能用：这类功能必须等真实设置回来再决定要不要
// 加载代码，否则会照着默认值抢跑 —— 表现就是后台明明关了、浏览器还是下载了那一坨。
const ready = ref(false)
let loading = null // 复用同一个 Promise，避免多处并发重复请求

// 字体标识 → 实际字体栈。与 backend/settings.go 里的常量一一对应。
// rouhei 对应 public/font 下的 Gen Jyuu Gothic（思源柔黑），
// @font-face 声明写在 style.css 里。
const FONT_STACKS = {
  system: '"PingFang SC", "Microsoft YaHei", "Segoe UI", system-ui, -apple-system, sans-serif',
  rouhei: '"GenJyuuGothic", "Gen Jyuu Gothic", "PingFang SC", "Microsoft YaHei", system-ui, sans-serif',
  round: '"Yuanti SC", "Hiragino Maru Gothic ProN", "Quicksand", "PingFang SC", "Microsoft YaHei", system-ui, sans-serif'
}

/**
 * 把全站字体写成 <html> 上的 --font-sans 内联变量。
 * 内联样式优先级高于样式表，所以能盖掉默认值；因为只是改变量，
 * 后台一保存就立刻生效、不需要重新构建。
 */
export function applyFont(name) {
  const key = FONT_STACKS[name] ? name : 'system'
  document.documentElement.style.setProperty('--font-sans', FONT_STACKS[key])
}

// 模糊 px → backdrop-filter 表达式（0 表示完全不模糊）
function blurValue(px) {
  const n = Number(px) || 0
  return n <= 0 ? 'none' : `blur(${n}px) saturate(160%)`
}

/**
 * 把外观定制写成 <html> 上的内联 CSS 变量。
 * 内联样式优先级高于样式表，所以能盖掉默认值；
 * 因为只是改变量，后台调完立刻生效、不需要重新构建。
 *
 * 注意：面板只写「自定义底色」和「浓度」两个变量，**不直接写 --glass-panel-bg**。
 * 面板实际用的底色由样式表决定：浅色主题 = 自定义色，夜间模式 = 强制夜间黑。
 * 这样切换深浅主题时由 CSS 自动重算，不依赖 JS 的刷新时机。
 */
export function applyTheme(values) {
  const t = { ...theme.value, ...(values || {}) }
  const root = document.documentElement

  root.style.setProperty('--accent', t.accent)
  // 卡片
  root.style.setProperty('--glass-solid-bg', `color-mix(in srgb, var(--card) ${t.cardAlpha}%, transparent)`)
  root.style.setProperty('--glass-solid-blur', blurValue(t.cardBlur))
  // 文章卡片
  root.style.setProperty('--card-bg', `color-mix(in srgb, var(--card) ${t.postAlpha}%, transparent)`)
  root.style.setProperty('--card-blur', blurValue(t.postBlur))
  // 下方毛玻璃区域：只给自定义底色与浓度，主题判断交给 CSS
  root.style.setProperty('--panel-color-custom', t.panelColor)
  root.style.setProperty('--panel-alpha', `${t.panelAlpha}%`)
  root.style.setProperty('--glass-panel-blur', blurValue(t.panelBlur))
  // 导航栏（滚到顶变色后）
  root.style.setProperty('--glass-nav-bg', `color-mix(in srgb, var(--card) ${t.navAlpha}%, transparent)`)
  root.style.setProperty('--glass-nav-blur', blurValue(t.navBlur))
}

// 把后端返回的设置合并进 theme（数字落回默认值）。后台保存后也用它整理返回值。
export function pickTheme(s) {
  const num = (v, def) => (Number.isFinite(Number(v)) ? Number(v) : def)
  return {
    accent: s.accent || DEFAULT_THEME.accent,
    cardAlpha: num(s.cardAlpha, DEFAULT_THEME.cardAlpha),
    cardBlur: num(s.cardBlur, DEFAULT_THEME.cardBlur),
    postAlpha: num(s.postAlpha, DEFAULT_THEME.postAlpha),
    postBlur: num(s.postBlur, DEFAULT_THEME.postBlur),
    panelColor: s.panelColor || DEFAULT_THEME.panelColor,
    panelAlpha: num(s.panelAlpha, DEFAULT_THEME.panelAlpha),
    panelBlur: num(s.panelBlur, DEFAULT_THEME.panelBlur),
    navAlpha: num(s.navAlpha, DEFAULT_THEME.navAlpha),
    navBlur: num(s.navBlur, DEFAULT_THEME.navBlur)
  }
}

// 把后端返回的点击特效设置合并进默认值（缺字段 / 类型不对都落回默认）。
export function pickClickFx(s) {
  const src = s && typeof s === 'object' ? s : {}
  const bool = (v, def) => (typeof v === 'boolean' ? v : def)
  const num = (v, def) => (Number.isFinite(Number(v)) ? Number(v) : def)
  const str = (v, def) => (typeof v === 'string' && v ? v : def)
  const d = DEFAULT_CLICK_FX
  return {
    enabled: bool(src.enabled, d.enabled),
    clickEnabled: bool(src.clickEnabled, d.clickEnabled),
    trailEnabled: bool(src.trailEnabled, d.trailEnabled),
    trailAlways: bool(src.trailAlways, d.trailAlways),
    color: typeof src.color === 'string' ? src.color : d.color,
    scale: num(src.scale, d.scale),
    opacity: num(src.opacity, d.opacity),
    maxDpr: num(src.maxDpr, d.maxDpr),
    backend: str(src.backend, d.backend)
  }
}

// 把一份设置对象灌进各个 ref（接口返回、以及本地缓存复用同一套逻辑）
function applySettings(s) {
  homeBg.value = s.homeBg || ''
  // 站长信息：首页首屏与导航栏都用它（后台「设置」里改昵称/头像即可同步）
  ownerNickname.value = s.ownerNickname || ''
  ownerAvatar.value = s.ownerAvatar || ''
  postBg.value = s.postBg || ''
  heroNickname.value = s.heroNickname || ''
  heroDesc.value = s.heroDesc || ''
  heroRoles.value = s.heroRoles || ''
  heroAvatar.value = s.heroAvatar || ''
  loginBg.value = s.loginBg || ''
  startedAt.value = Number(s.startedAt) || 0
  notice.value = s.notice || ''
  pageSize.value = Number(s.pageSize) || DEFAULT_PAGE_SIZE
  theme.value = pickTheme(s)
  applyTheme()
  // 功能开关：后端没返回该字段（旧版本）时按「启用」处理
  momoTalkEnabled.value = s.momoTalkEnabled !== false
  siteFont.value = s.siteFont || 'system'
  applyFont(siteFont.value)
  clickFx.value = pickClickFx(s.clickFx)
}

// ---------- 本地缓存：让「首屏」就知道有没有背景图 ----------
// 为什么需要：背景图地址是异步请求来的，NavBar 在拿到它之前会按「普通页面」渲染
// （文字是近黑色的）。等接口回来、图片再画出来，导航栏文字才转白 —— 用户看到的就是
// 「页面加载完了，导航栏还是黑的，过一下才变白」。把上次的设置缓存一份并在模块加载时
// 先灌进去，最常见的「回访」场景就没有这个闪变了。
const CACHE_KEY = 'spblog_site_settings'
try {
  const cached = JSON.parse(localStorage.getItem(CACHE_KEY) || 'null')
  if (cached && typeof cached === 'object') applySettings(cached)
} catch {
  /* 缓存坏了就忽略，等接口回来再灌 */
}

export function useSite() {
  if (!loading) {
    // 不再需要监听主题变化：面板底色由样式表按主题决定（夜间模式强制黑），
    // 这里写入的只是「自定义色 + 浓度」，深浅主题都会自动重算。
    loading = api.getSettings()
      .then((s) => {
        applySettings(s)
        try { localStorage.setItem(CACHE_KEY, JSON.stringify(s)) } catch { /* 隐私模式下写不进去，无所谓 */ }
      })
      .catch(() => { /* 读取失败时保持默认值（用样式表里的默认外观） */ })
      .finally(() => { ready.value = true })
  }
  return {
    homeBg,
    postBg,
    heroNickname,
    heroDesc,
    heroRoles,
    heroAvatar,
    loginBg,
    ownerNickname,
    ownerAvatar,
    startedAt,
    notice,
    pageSize,
    theme,
    momoTalkEnabled,
    siteFont,
    clickFx,
    ready
  }
}
