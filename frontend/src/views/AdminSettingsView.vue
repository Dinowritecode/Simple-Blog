<script setup>
// 后台：页面管理 —— 首页/文章页背景图 + 首页公告 + 列表分页 + 字体 + 点击特效 + 外观定制
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '../api'
import { useToast } from '../composables/useToast'
import {
  applyTheme, applyFont, pickTheme, DEFAULT_THEME, DEFAULT_CLICK_FX, pickClickFx,
  useSite
} from '../composables/useSite'
import AdminBgCard from '../components/admin/AdminBgCard.vue'

const { toast } = useToast()
// 直接改 useSite 里的值：保存背景图后切到前台立刻生效，不用刷新
const { homeBg, postBg } = useSite()

// ---------- 背景图（首页 / 文章页各一张，交给 AdminBgCard 渲染） ----------
const currentBg = ref('') // 首页已保存的地址
const currentPostBg = ref('') // 文章页已保存的地址

function onHomeBgSaved(v) {
  currentBg.value = v
  homeBg.value = v
}
function onPostBgSaved(v) {
  currentPostBg.value = v
  postBg.value = v
}
const loading = ref(true)

// ---------- 首页公告 ----------
const NOTICE_MAX = 300 // 与后端 noticeMaxRunes 保持一致
const currentNotice = ref('')
const noticeInput = ref('')
const savingNotice = ref(false)
const noticeLen = computed(() => noticeInput.value.length)

// ---------- 列表分页 ----------
const PAGE_SIZE_MIN = 1
const PAGE_SIZE_MAX = 50 // 与后端 maxPageSize 保持一致
const currentPageSize = ref(20)
const pageSizeInput = ref(20)
const savingPageSize = ref(false)

// ---------- 外观定制（主题色 + 各模块独立的浓度/模糊） ----------
const themeInput = ref({ ...DEFAULT_THEME })
const currentTheme = ref({ ...DEFAULT_THEME })
const savingTheme = ref(false)

// ---------- 功能开关：Momo Talk ----------
const momoOn = ref(true)      // 输入框当前值
const momoCurrent = ref(true) // 已保存的值
const savingMomo = ref(false)

async function saveMomoTalk() {
  savingMomo.value = true
  try {
    const d = await api.saveMomoTalk(momoOn.value)
    momoCurrent.value = !!d.momoTalkEnabled
    momoOn.value = momoCurrent.value
    toast.success(momoCurrent.value ? 'Momo Talk 已启用' : 'Momo Talk 已关闭（前端不再显示入口）')
  } catch (e) {
    toast.error(e.message)
  } finally {
    savingMomo.value = false
  }
}

// ---------- 全站字体 ----------
// value 与后端 settings.go 里的常量、以及 useSite.js 的字体栈一一对应
const FONT_OPTIONS = [
  { value: 'system', label: '系统默认', desc: '苹方 / 微软雅黑 / Segoe UI，跟随访问设备，加载最快' },
  { value: 'rouhei', label: '思源柔黑（Gen Jyuu Gothic）', desc: '站点自带字体，圆润柔和；文件较大，首次访问需下载' },
  { value: 'round', label: '圆体', desc: '使用系统里的圆体字体栈（macOS / iOS 效果最好）' }
]
const fontInput = ref('system')
const fontCurrent = ref('system')
const savingFont = ref(false)

async function saveFont() {
  savingFont.value = true
  try {
    const d = await api.saveFont(fontInput.value)
    fontCurrent.value = d.siteFont || fontInput.value
    fontInput.value = fontCurrent.value
    applyFont(fontCurrent.value) // 立刻在本页生效，不用刷新
    toast.success('全站字体已切换')
  } catch (e) {
    toast.error(e.message)
  } finally {
    savingFont.value = false
  }
}

// ---------- 点击特效（ba-click-fx） ----------
// 一份参数对象整体保存；字段名与后端 ClickFXSettings / 库的选项名一一对应
const fxInput = ref({ ...DEFAULT_CLICK_FX })
const fxCurrent = ref({ ...DEFAULT_CLICK_FX })
const savingFx = ref(false)

// 渲染后端选项：auto 让库自己挑（WebGPU → WebGL2 → Canvas2D 逐级回退）
const FX_BACKENDS = [
  { value: 'auto', label: '自动', desc: '优先 WebGPU / WebGL2，不支持时回退 Canvas2D' },
  { value: 'webgl2', label: 'WebGL2', desc: '完整特效（含辉光）；老设备可能不支持' },
  { value: 'canvas2d', label: 'Canvas2D', desc: '兼容性最好、最省电，但没有辉光等 GPU 效果' }
]

async function saveClickFx() {
  savingFx.value = true
  try {
    const d = await api.saveClickFx(fxInput.value)
    const saved = pickClickFx(d.clickFx)
    fxCurrent.value = saved
    fxInput.value = { ...saved }
    toast.success(saved.enabled ? '点击特效已保存（前台刷新后生效）' : '点击特效已关闭')
  } catch (e) {
    toast.error(e.message)
  } finally {
    savingFx.value = false
  }
}

// 四个可独立调节的模块：透明度 + 模糊；毛玻璃区域额外可改底色
const themeModules = [
  { key: 'card', label: '卡片（技能 / 项目 / 数据块 / 名片 / 公告 / 统计）', alpha: 'cardAlpha', blur: 'cardBlur' },
  { key: 'post', label: '文章卡片', alpha: 'postAlpha', blur: 'postBlur' },
  { key: 'panel', label: '下方毛玻璃区域（含页脚 / 文章详情页面板）', alpha: 'panelAlpha', blur: 'panelBlur', color: true },
  { key: 'nav', label: '导航栏（滚到顶变色后）', alpha: 'navAlpha', blur: 'navBlur' }
]

const HEX = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/

// 拖动滑块/取色时先本地预览（只改 CSS 变量，不落库）
function previewTheme() {
  applyTheme({
    ...themeInput.value,
    accent: (themeInput.value.accent || '').trim() || DEFAULT_THEME.accent,
    panelColor: (themeInput.value.panelColor || '').trim() || DEFAULT_THEME.panelColor
  })
}

// 没保存就离开这个页面时，把预览还原成已保存的值
onBeforeUnmount(() => applyTheme(currentTheme.value))

function resetTheme() {
  themeInput.value = { ...DEFAULT_THEME }
  previewTheme()
}

async function saveTheme() {
  const accent = (themeInput.value.accent || '').trim()
  const panelColor = (themeInput.value.panelColor || '').trim()
  if (!HEX.test(accent)) return toast.error('主题色需为 #RRGGBB 或 #RGB 格式')
  if (!HEX.test(panelColor)) return toast.error('毛玻璃区域底色需为 #RRGGBB 或 #RGB 格式')
  savingTheme.value = true
  try {
    const d = await api.saveTheme({ ...themeInput.value, accent, panelColor })
    currentTheme.value = pickTheme(d)
    themeInput.value = { ...currentTheme.value }
    applyTheme(currentTheme.value)
    toast.success('外观已更新')
  } catch (err) {
    toast.error(err.message)
  } finally {
    savingTheme.value = false
  }
}

async function load() {
  loading.value = true
  try {
    const s = await api.getSettings()
    currentBg.value = s.homeBg || ''
    currentPostBg.value = s.postBg || ''
    currentNotice.value = s.notice || ''
    noticeInput.value = currentNotice.value
    currentPageSize.value = Number(s.pageSize) || 20
    pageSizeInput.value = currentPageSize.value
    currentTheme.value = pickTheme(s)
    themeInput.value = { ...currentTheme.value }
    // 功能开关（后端没返回该字段时按「启用」处理）
    momoCurrent.value = s.momoTalkEnabled !== false
    momoOn.value = momoCurrent.value
    // 全站字体
    fontCurrent.value = s.siteFont || 'system'
    fontInput.value = fontCurrent.value
    // 点击特效
    fxCurrent.value = pickClickFx(s.clickFx)
    fxInput.value = { ...fxCurrent.value }
  } catch (e) {
    toast.error(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

// ---------- 保存公告 ----------
async function saveNotice() {
  if (noticeLen.value > NOTICE_MAX) {
    return toast.error(`公告最多 ${NOTICE_MAX} 字，当前 ${noticeLen.value} 字`)
  }
  savingNotice.value = true
  try {
    const d = await api.saveNotice(noticeInput.value)
    currentNotice.value = d.notice
    noticeInput.value = d.notice
    toast.success(d.notice ? '公告已更新，去首页看看吧' : '已清空公告')
  } catch (err) {
    toast.error(err.message)
  } finally {
    savingNotice.value = false
  }
}

// ---------- 保存每页条数 ----------
async function savePageSize() {
  const n = Number(pageSizeInput.value)
  if (!Number.isInteger(n) || n < PAGE_SIZE_MIN || n > PAGE_SIZE_MAX) {
    return toast.error(`每页条数需在 ${PAGE_SIZE_MIN}~${PAGE_SIZE_MAX} 之间`)
  }
  savingPageSize.value = true
  try {
    const d = await api.savePageSize(n)
    currentPageSize.value = d.pageSize
    pageSizeInput.value = d.pageSize
    toast.success(`已保存：每页 ${d.pageSize} 篇`)
  } catch (err) {
    toast.error(err.message)
  } finally {
    savingPageSize.value = false
  }
}
</script>

<template>
  <div class="dash">
    <div class="dash-head">
      <h1>页面管理</h1>
      <router-link class="btn btn-ghost" to="/" target="_blank">🌐 查看首页效果</router-link>
    </div>

    <div v-if="loading" class="dash-state">加载中…</div>

    <div v-else class="settings-stack">
      <!-- 首页背景图 -->
      <AdminBgCard
        title="🏠 首页背景图"
        desc="显示在<strong>首页</strong>最顶部首屏，会自动加暗色遮罩保证文字可读。建议用较宽的大图（如 1920×1080）。二选一：<strong>粘贴图片 URL</strong>，或<strong>直接上传本地图片</strong>（≤10MB，jpg/png/webp）。"
        :current="currentBg"
        :save-fn="api.saveHomeBg"
        empty-text="未设置背景（首页用默认纯色网格背景）"
        @saved="onHomeBgSaved"
      />

      <!-- 文章页背景图：与首页完全分开的一张图 -->
      <AdminBgCard
        title="📄 文章页背景图"
        desc="显示在<strong>文章详情页</strong>（点开任意一篇博客进去的那一页）。与上面的首页背景图<strong>互不影响</strong>，可以只改其中一个；留空则该页用默认纯色网格背景。"
        :current="currentPostBg"
        :save-fn="api.savePostBg"
        empty-text="未设置背景（文章页用默认纯色网格背景）"
        @saved="onPostBgSaved"
      />

      <!-- 首页公告：显示在首页毛玻璃区右侧的公告卡片里 -->
      <div class="settings-card">
        <h2>📢 首页公告</h2>
        <p class="settings-desc">
          显示在首页右侧的公告卡片中，<strong>只支持纯文本</strong>（不会解析 HTML），最多
          <strong>{{ NOTICE_MAX }}</strong> 字。留空保存即清除公告。
        </p>

        <label class="m-field">
          <span>公告内容（{{ noticeLen }} / {{ NOTICE_MAX }}）</span>
          <textarea
            v-model="noticeInput"
            rows="6"
            :maxlength="NOTICE_MAX"
            placeholder="例如：本站已迁移到新域名，欢迎收藏～"
          ></textarea>
        </label>

        <div class="bg-buttons">
          <button class="btn-sm ghost" :disabled="!noticeInput" @click="noticeInput = ''">清空输入</button>
          <button
            class="btn-sm"
            :disabled="savingNotice || noticeInput === currentNotice"
            @click="saveNotice"
          >
            {{ savingNotice ? '保存中…' : '💾 保存公告' }}
          </button>
        </div>
        <p class="settings-hint">
          {{ currentNotice ? '当前公告已发布（可在首页右侧看到）' : '当前没有公告' }}
        </p>
      </div>

      <!-- 列表分页：每页显示多少篇文章 -->
      <div class="settings-card">
        <h2>📄 列表分页</h2>
        <p class="settings-desc">
          首页博客区与「博客」列表页每页显示多少篇文章。文章数超过这个值就会出现页码，
          默认 <strong>20</strong>，可填 {{ PAGE_SIZE_MIN }}~{{ PAGE_SIZE_MAX }}。
        </p>

        <label class="m-field">
          <span>每页文章数</span>
          <input
            v-model.number="pageSizeInput"
            type="number"
            :min="PAGE_SIZE_MIN"
            :max="PAGE_SIZE_MAX"
            step="1"
          />
        </label>

        <div class="bg-buttons">
          <button
            class="btn-sm"
            :disabled="savingPageSize || Number(pageSizeInput) === currentPageSize"
            @click="savePageSize"
          >
            {{ savingPageSize ? '保存中…' : '💾 保存' }}
          </button>
        </div>
        <p class="settings-hint">当前：每页 {{ currentPageSize }} 篇</p>
      </div>

      <!-- 功能开关：Momo Talk（私信 / 好友） -->
      <div class="settings-card">
        <h2>💬 Momo Talk 功能开关</h2>
        <p class="settings-desc">
          关闭后，前端的<strong>所有 Momo Talk 入口都会消失</strong>：
          导航栏的私信按钮、评论区每条评论的「私信」按钮、以及聊天小窗。
          后端接口保留不动，重新启用即可恢复，历史聊天记录不会丢失。
          <br />
          适合「暂时不想开放私信」或「整站还没准备好这个功能」的场景。
        </p>

        <label class="switch-row">
          <input type="checkbox" v-model="momoOn" />
          <span class="switch-text">
            {{ momoOn ? '已启用（前端显示 Momo Talk）' : '已关闭（前端不显示任何入口）' }}
          </span>
        </label>

        <div class="settings-actions">
          <span class="settings-hint">
            当前状态：{{ momoCurrent ? '✅ 启用' : '⛔ 已关闭' }}
          </span>
          <button
            class="btn-sm"
            :disabled="savingMomo || momoOn === momoCurrent"
            @click="saveMomoTalk"
          >
            {{ savingMomo ? '保存中…' : '💾 保存' }}
          </button>
        </div>
      </div>

      <!-- 全站字体 -->
      <div class="settings-card">
        <h2>🔤 全站字体</h2>
        <p class="settings-desc">
          选中后作用于全站文字（正文、导航、评论区、Momo Talk 界面）。
          保存后立即生效，访客刷新页面即可看到。
          <br />
          「思源柔黑」是站点自带字体（<code>public/font/</code>），
          体积较大、首次访问需要下载，字体到位前会先用系统字体显示（不会白屏）。
        </p>

        <div class="font-list">
          <label v-for="f in FONT_OPTIONS" :key="f.value" class="font-item" :class="{ on: fontInput === f.value }">
            <input type="radio" :value="f.value" v-model="fontInput" />
            <span class="font-item-body">
              <span class="font-item-name">{{ f.label }}</span>
              <span class="font-item-desc">{{ f.desc }}</span>
            </span>
          </label>
        </div>

        <div class="settings-actions">
          <span class="settings-hint">
            当前：{{ (FONT_OPTIONS.find((f) => f.value === fontCurrent) || {}).label || fontCurrent }}
          </span>
          <button
            class="btn-sm"
            :disabled="savingFont || fontInput === fontCurrent"
            @click="saveFont"
          >
            {{ savingFont ? '保存中…' : '💾 保存' }}
          </button>
        </div>
      </div>

      <!-- 点击特效（ba-click-fx） -->
      <div class="settings-card">
        <h2>✨ 点击特效</h2>
        <p class="settings-desc">
          全站的鼠标点击波纹 / 碎片与移动拖尾（蔚蓝档案风格，基于 <code>ba-click-fx</code>）。
          保存后访客刷新页面即生效；<strong>关闭时浏览器根本不会下载特效代码</strong>
          （约 600 KB，只在开启时才加载）。触屏设备的点按同样会触发。
        </p>

        <label class="switch-row">
          <input type="checkbox" v-model="fxInput.enabled" />
          <span class="switch-text">
            {{ fxInput.enabled ? '已启用（全站开启点击特效）' : '已关闭（不加载任何特效代码）' }}
          </span>
        </label>

        <!-- 只有开启时才展开细节，避免一屏参数吓人 -->
        <div v-if="fxInput.enabled" class="fx-body">
          <div class="fx-row">
            <label class="switch-row sub">
              <input type="checkbox" v-model="fxInput.clickEnabled" />
              <span class="switch-text">点击波纹与碎片</span>
            </label>
            <label class="switch-row sub">
              <input type="checkbox" v-model="fxInput.trailEnabled" />
              <span class="switch-text">鼠标拖尾</span>
            </label>
          </div>

          <!-- 拖尾触发方式：需求里的「按住拖尾 / 不按住也拖尾」 -->
          <div class="fx-field">
            <span class="fx-label">拖尾触发方式</span>
            <div class="fx-radio-row">
              <label class="fx-radio" :class="{ on: fxInput.trailAlways }">
                <input type="radio" :value="true" v-model="fxInput.trailAlways" />
                <span class="fx-radio-body">
                  <span class="fx-radio-name">移动即拖尾</span>
                  <span class="fx-radio-desc">不用按鼠标，光标划过就留下拖尾（更显眼）</span>
                </span>
              </label>
              <label class="fx-radio" :class="{ on: !fxInput.trailAlways }">
                <input type="radio" :value="false" v-model="fxInput.trailAlways" />
                <span class="fx-radio-body">
                  <span class="fx-radio-name">按住才拖尾</span>
                  <span class="fx-radio-desc">按住鼠标左键拖动时才出现拖尾（更克制）</span>
                </span>
              </label>
            </div>
          </div>

          <div class="fx-grid two">
            <div class="fx-field">
              <span class="fx-label">特效颜色</span>
              <div class="fx-color-row">
                <input
                  class="fx-color"
                  type="color"
                  :value="fxInput.color || currentTheme.accent"
                  @input="fxInput.color = $event.target.value"
                />
                <input class="fx-text" type="text" maxlength="7" placeholder="#4ca7ff" v-model="fxInput.color" />
                <button class="btn-sm ghost" @click="fxInput.color = ''">跟随主题色</button>
              </div>
              <span class="fx-hint">
                留空即跟随站点主题色（当前 {{ currentTheme.accent }}）；只接受 <code>#rrggbb</code>。
              </span>
            </div>

            <div class="fx-field">
              <span class="fx-label">渲染后端</span>
              <div class="fx-radio-row col">
                <label v-for="b in FX_BACKENDS" :key="b.value" class="fx-radio" :class="{ on: fxInput.backend === b.value }">
                  <input type="radio" :value="b.value" v-model="fxInput.backend" />
                  <span class="fx-radio-body">
                    <span class="fx-radio-name">{{ b.label }}</span>
                    <span class="fx-radio-desc">{{ b.desc }}</span>
                  </span>
                </label>
              </div>
            </div>
          </div>

          <div class="fx-grid three">
            <div class="fx-field">
              <span class="fx-label">尺寸倍率 <em>{{ Number(fxInput.scale).toFixed(1) }}×</em></span>
              <input class="fx-range" type="range" min="0.2" max="2" step="0.1" v-model.number="fxInput.scale" />
              <span class="fx-hint">波纹与碎片的整体大小，0.2 ~ 2</span>
            </div>
            <div class="fx-field">
              <span class="fx-label">不透明度 <em>{{ Math.round(fxInput.opacity * 100) }}%</em></span>
              <input class="fx-range" type="range" min="0.05" max="1" step="0.05" v-model.number="fxInput.opacity" />
              <span class="fx-hint">调低可以让特效更含蓄，不抢正文注意力</span>
            </div>
            <div class="fx-field">
              <span class="fx-label">清晰度上限 DPR <em>{{ Number(fxInput.maxDpr).toFixed(1) }}×</em></span>
              <input class="fx-range" type="range" min="1" max="2" step="0.5" v-model.number="fxInput.maxDpr" />
              <span class="fx-hint">高分屏上画布的像素比上限；2 更锐利但更吃 GPU</span>
            </div>
          </div>
        </div>

        <div class="settings-actions fx-actions">
          <span class="settings-hint">
            当前状态：{{ fxCurrent.enabled ? '✅ 启用' : '⛔ 已关闭' }}
          </span>
          <button class="btn-sm" :disabled="savingFx" @click="saveClickFx">
            {{ savingFx ? '保存中…' : '💾 保存' }}
          </button>
        </div>
      </div>

      <!-- 外观定制：主题色 + 每个模块独立的透明度与模糊 -->
      <div class="settings-card">
        <h2>🎨 外观定制</h2>
        <p class="settings-desc">
          主题色作用于全站。下面每个模块的<strong>透明度</strong>（底色浓度，越小越透明）
          与<strong>模糊</strong>（px，0 = 不模糊）都可单独调。
          拖动即时预览，满意后再点保存（不保存离开本页会自动还原）。
        </p>

        <label class="m-field">
          <span>主题色</span>
          <div class="color-row">
            <input type="color" v-model="themeInput.accent" @input="previewTheme" />
            <input type="text" v-model="themeInput.accent" placeholder="#00AAEE" @input="previewTheme" />
          </div>
        </label>

        <div v-for="m in themeModules" :key="m.key" class="theme-group">
          <h3 class="theme-group-title">{{ m.label }}</h3>

          <label v-if="m.color" class="m-field">
            <span>底色</span>
            <div class="color-row">
              <input type="color" v-model="themeInput.panelColor" @input="previewTheme" />
              <input type="text" v-model="themeInput.panelColor" placeholder="#FFFFFF" @input="previewTheme" />
            </div>
          </label>

          <label class="m-field">
            <span>透明度（底色浓度 {{ themeInput[m.alpha] }}%）</span>
            <input type="range" min="0" max="100" step="1" v-model.number="themeInput[m.alpha]" @input="previewTheme" />
          </label>

          <label class="m-field">
            <span>模糊 {{ themeInput[m.blur] }} px</span>
            <input type="range" min="0" max="50" step="1" v-model.number="themeInput[m.blur]" @input="previewTheme" />
          </label>
        </div>

        <div class="bg-buttons">
          <button class="btn-sm ghost" @click="resetTheme">恢复默认</button>
          <button class="btn-sm" :disabled="savingTheme" @click="saveTheme">
            {{ savingTheme ? '保存中…' : '💾 保存外观' }}
          </button>
        </div>
        <p class="settings-hint">
          当前：主题色 {{ currentTheme.accent }} · 卡片 {{ currentTheme.cardAlpha }}%/{{ currentTheme.cardBlur }}px ·
          文章卡 {{ currentTheme.postAlpha }}%/{{ currentTheme.postBlur }}px ·
          毛玻璃区 {{ currentTheme.panelColor }} {{ currentTheme.panelAlpha }}%/{{ currentTheme.panelBlur }}px ·
          导航栏 {{ currentTheme.navAlpha }}%/{{ currentTheme.navBlur }}px
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dash-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}
.dash-head h1 { font-size: 1.4rem; }
.dash-state { text-align: center; color: var(--text-dim); padding: 80px 0; }

/* 多张设置卡片纵向排列 */
.settings-stack { display: flex; flex-direction: column; gap: 22px; }

/* 外观定制：取色器 + 十六进制输入 */
.color-row { display: flex; align-items: center; gap: 10px; }

/* 每个模块一组（透明度 + 模糊） */
.theme-group {
  margin-top: 18px;
  padding: 14px 16px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--text) 5%, transparent);
}
.theme-group-title {
  font-size: 13.5px;
  font-weight: 700;
  margin-bottom: 10px;
  color: var(--text);
}
.theme-group .m-field:last-child { margin-bottom: 0; }
.color-row input[type="color"] {
  width: 48px;
  height: 38px;
  padding: 2px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg);
  cursor: pointer;
  flex-shrink: 0;
}
.color-row input[type="text"] { flex: 1; }

/* 滑块：清掉输入框默认样式，用主题色 */
.m-field input[type="range"] {
  width: 100%;
  padding: 0;
  border: none;
  background: transparent;
  accent-color: var(--accent);
  cursor: pointer;
}

/* ---------- 点击特效 ---------- */
.fx-body {
  margin-top: 16px;
  padding: 16px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--text) 5%, transparent);
}
/* 参数区 / 开关和下面的保存行之间留一点距离 */
.fx-actions { margin-top: 18px; }
.fx-row { display: flex; flex-wrap: wrap; gap: 20px; }
.fx-row .switch-row { margin-top: 0; }
.switch-row.sub { font-size: 13.5px; }

.fx-field { display: flex; flex-direction: column; gap: 7px; margin-bottom: 18px; }
.fx-field:last-child { margin-bottom: 0; }
/* 开关行后面的第一个参数区要拉开距离，否则标题挤在复选框下面 */
.fx-row + .fx-field { margin-top: 18px; }
.fx-label { font-size: 13px; font-weight: 600; color: var(--text); }
.fx-label em {
  font-style: normal;
  font-weight: 400;
  font-family: var(--font-mono);
  color: var(--accent);
  margin-left: 6px;
}
.fx-hint { font-size: 12px; color: var(--text-dim); line-height: 1.6; }
.fx-hint code { font-family: var(--font-mono); font-size: 11.5px; }

/* 两列 / 三列的参数区（窄屏自动堆成一列） */
.fx-grid { display: grid; gap: 0 22px; margin-top: 4px; }
.fx-grid.two { grid-template-columns: 1fr 1fr; }
.fx-grid.three { grid-template-columns: repeat(3, 1fr); }
@media (max-width: 720px) {
  .fx-grid.two, .fx-grid.three { grid-template-columns: 1fr; }
}

/* 单选卡（拖尾方式 / 渲染后端） */
.fx-radio-row { display: flex; gap: 10px; }
.fx-radio-row.col { flex-direction: column; gap: 8px; }
.fx-radio {
  flex: 1;
  display: flex;
  align-items: flex-start;
  gap: 9px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 9px;
  cursor: pointer;
  transition: all 0.2s ease;
}
.fx-radio:hover { border-color: var(--accent); }
.fx-radio.on {
  border-color: var(--accent);
  background: var(--accent-soft);
}
.fx-radio input { margin-top: 2px; accent-color: var(--accent); flex-shrink: 0; }
.fx-radio-body { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.fx-radio-name { font-size: 13px; font-weight: 600; }
.fx-radio-desc { font-size: 11.5px; color: var(--text-dim); line-height: 1.5; }

/* 颜色选择 */
.fx-color-row { display: flex; align-items: center; gap: 8px; }
.fx-color-row .fx-color {
  width: 46px;
  height: 38px;
  padding: 2px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg);
  cursor: pointer;
  flex-shrink: 0;
}
.fx-color-row .fx-text { flex: 1; min-width: 0; }

.fx-range {
  width: 100%;
  padding: 0;
  border: none;
  background: transparent;
  accent-color: var(--accent);
  cursor: pointer;
}

.settings-card {
  max-width: 680px;
  background: var(--glass-solid-bg);
  backdrop-filter: var(--glass-solid-blur);
  -webkit-backdrop-filter: var(--glass-solid-blur);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 28px;
}
.settings-card h2 { font-size: 1.15rem; margin-bottom: 8px; }
.settings-desc { color: var(--text-dim); font-size: 14px; line-height: 1.8; margin-bottom: 20px; }

/* 背景图卡片已抽成 AdminBgCard 组件（首页 / 文章页两张图共用），
   预览区的样式跟着搬到了那个组件里，这里只留其它卡片还在用的按钮行。 */
.bg-buttons { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 2px; }
.settings-hint {
  margin-top: 14px;
  font-size: 12.5px;
  color: var(--text-dim);
  font-family: var(--font-mono);
  word-break: break-all;
}

/* ---------- 功能开关（Momo Talk） ---------- */
.switch-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 16px;
  cursor: pointer;
  user-select: none;
}
.switch-row input {
  width: 18px;
  height: 18px;
  accent-color: var(--accent);
  cursor: pointer;
  flex-shrink: 0;
}
.switch-text { font-size: 13.5px; color: var(--text); }

/* 保存行：左边状态、右边按钮，与其它设置卡片一致 */
.settings-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.settings-actions .settings-hint { margin-top: 0; }

/* ---------- 全站字体 ---------- */
.font-list { display: flex; flex-direction: column; gap: 8px; margin-top: 16px; }
.font-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}
.font-item:hover { background: var(--bg-alt); }
.font-item.on { border-color: var(--accent); background: var(--accent-soft); }
.font-item input {
  margin-top: 3px;
  accent-color: var(--accent);
  cursor: pointer;
  flex-shrink: 0;
}
.font-item-body { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.font-item-name { font-size: 13.5px; font-weight: 600; color: var(--text); }
.font-item-desc { font-size: 12px; color: var(--text-dim); line-height: 1.6; }
</style>
