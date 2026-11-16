// ============================================================
// useClickFx.js —— 全站点击特效（ba-click-fx）的唯一入口
//
// 为什么单独一个 composable：
//   · ba-click-fx 的 dist 有 600 KB 左右，必须**动态 import**（Vite 会单独切一个
//     chunk），关掉特效的站点 / 访客一个字节都不下载；
//   · 特效实例是全局单例，跟着站点设置走：后台改一个参数，所有已打开的页面
//     下一次轮询/刷新就能拿到新值，中途不需要刷新页面。
//
// 生命周期：App.vue 调一次 useClickFx()，内部自己挂 onMounted / onBeforeUnmount。
// 效果：
//   clickFx.enabled 为 true  → 懒加载库 → new BAClickFX(...)
//   clickFx.enabled 为 false → destroy() 并移除画布
//   其它参数变化            → updateConfig(...)，不重建实例
// ============================================================

import { onBeforeUnmount, watch } from 'vue'
import { useSite } from './useSite'

// 库的默认游戏蓝，用兜底（站点主题色是 #rgb 以外的格式时）
const FALLBACK_COLOR = '#4ca7ff'

// 单例状态（模块级，多个组件调用也只会有一份特效）
let fx = null // BAClickFX 实例
let loading = null // 正在加载库的 Promise（防止快速开关时重复加载）
let started = false // 是否已经挂过生命周期

/**
 * 把站点设置翻译成 ba-click-fx 的构造参数。
 *
 * 几个刻意的选择：
 *   · outputCompositing: 'browser-overlay' —— 普通网页读不到特效下方的真实像素
 *     （多层 CSS 背景 + 滚动内容 + 跨域图片），所以输出带 Alpha 的透明覆盖层，
 *     由 DOM 做最后一次合成。target 不传 → 库自建 pointer-events:none 的全屏层，
 *     不会挡住任何点击（实测 elementFromPoint 仍然命中导航链接）。
 *   · hostCompositing 用默认的 'source-over'（正常 alpha 叠加），**刻意不用官网推荐的
 *     'screen'**：本站浅色主题下背景接近纯白，screen 是「加色」，白底上加什么都是白，
 *     特效会淡到几乎看不见（实测只勉强看到一圈轮廓）。source-over 在浅色/深色/背景图
 *     上都能正常显色。想要游戏里那种发光感可以改成 'screen'（深色站点才划算）。
 *   · themeColor 只在「6 位十六进制」时才传：库只接受 6 位（#rgb 会抛错）。
 */
function buildOptions(cfg, accent) {
  const color = cfg.color || accent || FALLBACK_COLOR
  return {
    outputCompositing: 'browser-overlay',
    hostCompositing: 'source-over',
    hostCompositingSurface: 'dom-backdrop',
    clickEnabled: cfg.clickEnabled,
    trailEnabled: cfg.trailEnabled,
    trailAlways: cfg.trailAlways,
    scale: cfg.scale,
    opacity: cfg.opacity,
    maxDpr: cfg.maxDpr,
    effectBackend: cfg.backend,
    // 只认 #rrggbb；主题色是 #rgb 时退回库默认蓝，避免构造直接抛错
    ...(/^#[0-9a-fA-F]{6}$/.test(color) ? { themeColor: color } : {})
  }
}

// 运行时可热改的字段（target / inputFilter 只能在构造时给，这里不含）
function buildUpdate(cfg) {
  return {
    clickEnabled: cfg.clickEnabled,
    trailEnabled: cfg.trailEnabled,
    trailAlways: cfg.trailAlways,
    scale: cfg.scale,
    opacity: cfg.opacity,
    maxDpr: cfg.maxDpr,
    effectBackend: cfg.backend
  }
}

async function start(cfg, accent) {
  if (fx || loading) return
  loading = import('ba-click-fx')
    .then((mod) => {
      const BAClickFX = mod.BAClickFX || mod.default
      if (!BAClickFX) throw new Error('ba-click-fx 没有导出 BAClickFX')
      fx = new BAClickFX(buildOptions(cfg, accent))
    })
    .catch((e) => {
      // 加载 / 初始化失败只记日志：特效是锦上添花，绝不能影响页面本身
      console.warn('[click-fx] 特效初始化失败：', e)
    })
    .finally(() => {
      loading = null
    })
  return loading
}

function stop() {
  if (fx) {
    try {
      fx.destroy()
    } catch (e) {
      console.warn('[click-fx] 销毁失败：', e)
    }
    fx = null
  }
}

/**
 * 挂载全站点击特效。在根组件调用一次即可。
 */
export function useClickFx() {
  const { clickFx, theme, ready } = useSite()

  if (started) return { clickFx }
  started = true

  // 按当前设置决定「加载并创建」还是「销毁」。默认值是开启的，所以必须等
  // 真实设置回来（ready）才能动 —— 否则后台关了也会先抢跑下载 600 KB 的 chunk。
  function applyEnabled() {
    if (clickFx.value.enabled) start(clickFx.value, theme.value.accent)
    else stop()
  }

  onBeforeUnmount(stop)

  // 设置到位（成功或失败）后按真实开关启动一次
  watch(ready, (ok) => {
    if (ok) applyEnabled()
  }, { immediate: true })

  // 开关的运行时变化：开 → 加载并创建；关 → 销毁
  watch(() => clickFx.value.enabled, applyEnabled)

  // 其它参数：实例在就直接热改，不在（还没加载完 / 已关闭）就等下次 start 用新值
  watch(
    () => [
      clickFx.value.clickEnabled,
      clickFx.value.trailEnabled,
      clickFx.value.trailAlways,
      clickFx.value.scale,
      clickFx.value.opacity,
      clickFx.value.maxDpr,
      clickFx.value.backend
    ],
    () => {
      if (!fx) return
      try {
        fx.updateConfig(buildUpdate(clickFx.value))
      } catch (e) {
        console.warn('[click-fx] 参数更新失败：', e)
      }
    }
  )

  // 颜色 / 主题色变了：单独走 setThemeColor（updateConfig 不接受 themeColor）
  watch(
    () => [clickFx.value.color, theme.value.accent],
    () => {
      if (!fx) return
      const color = clickFx.value.color || theme.value.accent
      if (!/^#[0-9a-fA-F]{6}$/.test(color)) return
      try {
        fx.setThemeColor(color)
      } catch (e) {
        console.warn('[click-fx] 颜色更新失败：', e)
      }
    }
  )

  return { clickFx }
}
