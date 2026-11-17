<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useTypewriter } from '../composables/useTypewriter'
// 站点头像统一用 src/others/photo.jpg（首页首屏、左侧名片、浏览器标签页图标同一张）
import photoUrl from '../others/photo.jpg'

// bg    ：首页背景图 URL（只用于决定「是否压在背景图上」）
// bgOk  ：这张图**是否真的加载成功**。
//   背景图是远端的，CDN 挂了 / 网络慢 / 被拦截时它不会出现，这时页面是浅色底。
//   如果仍按「有背景图」把文字切成白色，就会变成白字白底——所以只有图片真的
//   画出来了才切白色，否则退回浅底深字。
// nickname / desc / roles / avatar：后台「首页文案」里可改；留空则用下面的内置默认值
const props = defineProps({
  bg: { type: String, default: '' },
  bgOk: { type: Boolean, default: false },
  nickname: { type: String, default: '' },
  desc: { type: String, default: '' },
  roles: { type: String, default: '' },
  avatar: { type: String, default: '' }
})

const DEFAULT_ROLES = [
  'Rust Developer',
  'Go Developer',
  'C++ Developer',
  'Systems Programmer',
  '性能强迫症患者'
]
const DEFAULT_DESC = '热爱底层、性能与优雅的代码。用 Rust 写**安全**、用 Go 写**并发**、用 C++ 榨干每一分**性能**。'
const DEFAULT_NICKNAME = '凛冬|RinEyce'

// 昵称：后台用 | 分隔「主题色前半段」和「正文后半段」
const nameParts = computed(() => {
  const raw = (props.nickname || '').trim() || DEFAULT_NICKNAME
  const i = raw.indexOf('|')
  if (i < 0) return { pre: '', main: raw }
  return { pre: raw.slice(0, i), main: raw.slice(i + 1) }
})

// 打字动画的候选词：一行一条；后台没配就用默认
const roleList = computed(() => {
  const arr = (props.roles || '')
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean)
  return arr.length ? arr : DEFAULT_ROLES
})

// 介绍：支持 **文字** 表示高亮（拆成若干段，避免用 v-html）
const descSegments = computed(() => {
  const raw = (props.desc || '').trim() || DEFAULT_DESC
  const out = []
  const re = /\*\*(.+?)\*\*/g
  let last = 0
  let m
  while ((m = re.exec(raw))) {
    if (m.index > last) out.push({ text: raw.slice(last, m.index), hl: false })
    out.push({ text: m[1], hl: true })
    last = m.index + m[0].length
  }
  if (last < raw.length) out.push({ text: raw.slice(last), hl: false })
  return out
})

const avatarUrl = computed(() => props.avatar || photoUrl)

const { text: typed } = useTypewriter(() => roleList.value)

// ---------- 右侧的时钟 + 星期 ----------
// 只画刻度和指针，不画表盘轮廓：这样它和旁边那行等宽小字是同一种「细线」气质。
// 秒针用主题色，是这一小块里唯一的彩色。
const now = ref(new Date())
let tickTimer = null
onMounted(() => {
  tickTimer = setInterval(() => {
    now.value = new Date()
  }, 1000)
})
onBeforeUnmount(() => clearInterval(tickTimer))

// 指针角度（以 12 点为 0 度，顺时针）
const hourDeg = computed(() => (now.value.getHours() % 12) * 30 + now.value.getMinutes() * 0.5)
const minuteDeg = computed(() => now.value.getMinutes() * 6 + now.value.getSeconds() * 0.1)
const secondDeg = computed(() => now.value.getSeconds() * 6)
const WEEKDAYS = ['SUNDAY', 'MONDAY', 'TUESDAY', 'WEDNESDAY', 'THURSDAY', 'FRIDAY', 'SATURDAY']
const weekday = computed(() => WEEKDAYS[now.value.getDay()])
// 6:00–18:00 算白天（太阳），其余时间月亮
const isDay = computed(() => {
  const h = now.value.getHours()
  return h >= 6 && h < 18
})
</script>

<template>
  <section class="hero" :class="{ 'has-bg': bg && bgOk }">
    <div class="hero-inner" v-reveal>
      <!-- 头像和昵称同一行：头像当「字首」，标题跟它垂直居中 -->
      <div class="hero-title-row">
        <img class="avatar" :src="avatarUrl" :alt="`${nameParts.main} 的头像`" />
        <h1 class="hero-name">
          <span v-if="nameParts.pre" class="accent">{{ nameParts.pre }}</span>{{ nameParts.main }}
        </h1>
      </div>

      <p class="hero-role">
        <span class="type-caret">&gt;</span>
        <span>{{ typed }}</span><span class="cursor">_</span>
      </p>

      <p class="hero-desc">
        <template v-for="(seg, i) in descSegments" :key="i">
          <span v-if="seg.hl" class="nb">{{ seg.text }}</span>
          <template v-else>{{ seg.text }}</template>
        </template>
      </p>

      <div class="hero-actions">
        <!-- 主按钮用白色药丸（参考图的 READ JOURNAL 那种）：压在图上比实底蓝更有纸感 -->
        <a class="hero-cta" href="#blog">
          读文章
          <span class="cta-arrow" aria-hidden="true">↓</span>
        </a>
        <span class="hero-cta-note">RUST / GO / SYSTEMS</span>
        <a class="hero-ghost" href="#projects">看看项目</a>
      </div>
    </div>

    <!-- 右侧：时钟 + 太阳/月亮 + 星期 + 一句小字，细线风格 -->
    <div class="hero-aside" aria-hidden="true">
      <svg class="hero-clock" viewBox="0 0 100 100">
        <!-- 12 条刻度，3/6/9/12 点略长 -->
        <line
          v-for="i in 12"
          :key="i"
          x1="50"
          y1="7"
          x2="50"
          :y2="i % 3 === 0 ? 20 : 14"
          :transform="`rotate(${i * 30} 50 50)`"
        />
        <line class="hand-hour" x1="50" y1="50" x2="50" y2="30" :transform="`rotate(${hourDeg} 50 50)`" />
        <line class="hand-minute" x1="50" y1="50" x2="50" y2="19" :transform="`rotate(${minuteDeg} 50 50)`" />
        <line class="hand-second" x1="50" y1="58" x2="50" y2="12" :transform="`rotate(${secondDeg} 50 50)`" />
        <circle class="clock-pin" cx="50" cy="50" r="1.8" />
      </svg>

      <div class="hero-meta">
        <span class="hero-meta-rule"></span>
        <div class="hero-meta-body">
          <!-- 白天太阳、夜里月亮（6:00–18:00 算白天） -->
          <svg v-if="isDay" class="hero-daynight" viewBox="0 0 24 24">
            <circle cx="12" cy="12" r="4.2" />
            <g>
              <line x1="12" y1="2.6" x2="12" y2="4.8" />
              <line x1="12" y1="19.2" x2="12" y2="21.4" />
              <line x1="2.6" y1="12" x2="4.8" y2="12" />
              <line x1="19.2" y1="12" x2="21.4" y2="12" />
              <line x1="5.4" y1="5.4" x2="7" y2="7" />
              <line x1="17" y1="17" x2="18.6" y2="18.6" />
              <line x1="5.4" y1="18.6" x2="7" y2="17" />
              <line x1="17" y1="7" x2="18.6" y2="5.4" />
            </g>
          </svg>
          <svg v-else class="hero-daynight" viewBox="0 0 24 24">
            <path d="M20 14.6A8.6 8.6 0 0 1 9.4 4a8.6 8.6 0 1 0 10.6 10.6z" />
            <path d="M17.4 3.2l.65 1.75 1.75.65-1.75.65-.65 1.75-.65-1.75-1.75-.65 1.75-.65z" />
          </svg>

          <b class="hero-weekday">{{ weekday }}</b>
          <span class="hero-tagline">I really want to stay at your house</span>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.hero {
  /* 这一屏的衬线字体栈：标题 / 副标题 / 简介共用同一套，中英文都落到衬线上 */
  --font-serif: 'Playfair Display', Georgia, 'Times New Roman',
    'Source Han Serif SC', 'Noto Serif CJK SC', 'Songti SC', 'SimSun', serif;
  /* 首屏高度 = 视口 − 8px：白色区域和它上面的三层波浪都压到屏幕最底部，
     只留一线白和那道波浪边在折叠线上。
     （这个数字就是「白色区域上提多少」，越小、波浪和文章区越靠下） */
  min-height: calc(100svh - 8px);
  display: flex;
  flex-direction: column;
  align-items: center;
  /* 内容靠左下：贴住首屏底部，左边与导航栏内容区对齐 */
  justify-content: flex-end;
  text-align: left;
  /* 负外边距把自己拉到导航栏底下：背景图从屏幕最顶端开始铺，导航栏浮在图上，
     这样顶部就不会再露出「导航栏高度那一条」页面底色（所谓白边）。 */
  margin-top: calc(-1 * var(--nav-h));
  /* 底部留够：波浪带会往上顶到约 68px，文字要停在波浪之上 */
  padding: calc(var(--nav-h) + 40px) 0 152px;
  position: relative;
  overflow: hidden;
}

/* 极淡的网格背景，增加层次而不喧宾夺主 */
.hero::before {
  content: "";
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(var(--border) 1px, transparent 1px),
    linear-gradient(90deg, var(--border) 1px, transparent 1px);
  background-size: 44px 44px;
  opacity: 0.35;
  mask-image: radial-gradient(ellipse 70% 60% at 50% 40%, black 30%, transparent 75%);
  -webkit-mask-image: radial-gradient(ellipse 70% 60% at 50% 40%, black 30%, transparent 75%);
  pointer-events: none;
}

/* 内容容器：与导航栏同一条左边缘（1328 容器 + 24px 内边距），左对齐 */
.hero-inner {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 1328px;
  margin: 0 auto;
  padding: 0 24px;
}

/* 背景图上一律不压任何遮罩，原图原样呈现（站长要求）。
   可读性交给多层文字阴影：贴着字形的一圈勾边 + 一层中距扩散 + 一层大范围压暗，
   光晕跟着字走，不会在图上留下任何一块矩形暗面。 */
.hero.has-bg::before { display: none; }
.has-bg .avatar { border-color: #fff; outline-color: rgba(255, 255, 255, 0.35); }
.has-bg .hero-desc {
  color: #fff;
  /* 衬线字本身笔画细，三层重阴影会糊成一团；改成一层很轻的光晕 */
  text-shadow: 0 1px 12px rgba(0, 0, 0, 0.55);
}
/* 昵称不压阴影：大字靠字号和对比度本身就够，加了阴影反而显得脏 */
.has-bg .hero-name {
  color: #fff;
}
/* 压在图上的「凛冬」用亮品牌色（深色底上亮度足够），不跟着浅底那档变深 */
.has-bg .hero-name .accent {
  color: var(--accent);
}
.has-bg .hero-role {
  color: #fff;
  text-shadow: 0 1px 12px rgba(0, 0, 0, 0.55);
}
.has-bg .hero-scroll { border-color: rgba(255, 255, 255, 0.6); z-index: 1; }

/* 压在图上时，右侧时钟和小字统一切白色系 */
.has-bg .hero-cta-note,
.has-bg .hero-aside {
  color: rgba(255, 255, 255, 0.72);
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.55);
}
.has-bg .hero-ghost {
  color: rgba(255, 255, 255, 0.85);
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.55);
}
.has-bg .hero-ghost:hover {
  color: #fff;
  border-color: rgba(255, 255, 255, 0.8);
}
.has-bg .btn-ghost { border-color: rgba(255, 255, 255, 0.7); color: #fff; background: rgba(8, 12, 20, 0.4); }
.has-bg .btn-ghost:hover { border-color: var(--accent); color: #fff; background: rgba(8, 12, 20, 0.6); }

/* 头像 + 昵称同一行 */
.hero-title-row {
  display: flex;
  align-items: center;
  gap: 18px;
  margin-bottom: 10px;
}

.avatar {
  width: 72px;
  height: 72px;
  border-radius: 50%;
  object-fit: cover;
  flex-shrink: 0;
  border: 3px solid var(--bg);
  outline: 1px solid var(--border);
  box-shadow: var(--shadow);
  transition: transform 0.4s ease, box-shadow 0.4s ease;
}
.avatar:hover {
  transform: translateY(-4px) scale(1.02);
  box-shadow: 0 0 0 4px var(--accent-soft), var(--shadow);
}

/* 标题用衬线体：参考图那种「淡雅大标题」的感觉，中文回退到宋体系 */
.hero-name {
  font-family: var(--font-serif);
  font-size: clamp(2.4rem, 6.4vw, 4.4rem);
  font-weight: 500;
  letter-spacing: 1px;
  line-height: 1.08;
  margin-bottom: 0;
}

.hero-role {
  /* 副标题用衬线 + 字距，和参考图那句「一个用于记录技术、项目与生活的个人博客」同一套语气 */
  font-family: var(--font-serif);
  font-size: clamp(1.05rem, 2.6vw, 1.32rem);
  letter-spacing: 0.12em;
  color: var(--text-dim);
  margin-bottom: 12px;
  min-height: 1.7em;
}
.hero-role .type-caret { color: var(--accent); margin-right: 6px; }

.cursor {
  display: inline-block;
  color: var(--accent);
  animation: blink 1s step-end infinite;
}
@keyframes blink { 50% { opacity: 0; } }

.hero-desc {
  max-width: 560px;
  margin: 0 0 18px;
  color: var(--text-dim);
  /* 简介和副标题一样用衬线，字距稍微放开一点，读起来更像「一句题记」 */
  font-family: var(--font-serif);
  font-size: 16px;
  letter-spacing: 0.08em;
  line-height: 1.85;
  /* 中文断行：窄屏时让每行长度尽量均匀，避免末行只剩「能。」这种孤字 */
  text-wrap: balance;
}
/* 「安全 / 并发 / 性能」这种两字词被拆到两行会读着别扭，整词不换行 */
.hero-desc .nb { white-space: nowrap; }

/* 按钮组：白色药丸 + 一行等宽小字注脚 + 次级文字按钮，跟文字一样靠左 */
.hero-actions {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 0;
}

.hero-cta {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 13px 24px;
  border-radius: 999px;
  background: #fff;
  color: #12151b;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.08em;
  box-shadow: 0 10px 28px rgba(8, 12, 20, 0.28);
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}
.hero-cta:hover {
  text-decoration: none;
  transform: translateY(-2px);
  box-shadow: 0 14px 34px rgba(8, 12, 20, 0.36);
}
.cta-arrow {
  font-size: 14px;
  line-height: 1;
  transition: transform 0.2s ease;
}
.hero-cta:hover .cta-arrow { transform: translateY(3px); }

.hero-cta-note {
  /* 参考图里那行 CODE / LIFE / QUIET THOUGHTS：粗体、全大写、大字距 */
  font-family: var(--font-sans);
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.22em;
  text-transform: uppercase;
  color: var(--text-dim);
}

.hero-ghost {
  font-size: 13px;
  color: var(--text-dim);
  border-bottom: 1px solid transparent;
  padding-bottom: 2px;
  transition: color 0.18s ease, border-color 0.18s ease;
}
.hero-ghost:hover { color: var(--accent-ink); border-color: var(--accent-ink); text-decoration: none; }

/* 右侧：时钟 + 竖线小字块，整体靠右。
   这块是底部锚定的：gap 变大只会把时钟往上推。
   要让「小字往下挪、时钟不动」，就把 bottom 和 gap 一起调整 ——
   现在这组值（126 / 34）正好让时钟停在原位、小字块下移 12px。 */
.hero-aside {
  position: absolute;
  right: max(24px, calc((100% - 1328px) / 2 + 24px));
  bottom: 126px;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 34px;
  color: var(--text-dim);
}

/* 表盘只画刻度 + 三根指针，不画外圈 */
.hero-clock {
  width: 112px;
  height: 112px;
  overflow: visible;
}
.hero-clock line {
  stroke: currentColor;
  stroke-width: 2.4;
  stroke-linecap: round;
  opacity: 0.75;
}
.hero-clock .hand-hour {
  stroke-width: 3.4;
  opacity: 0.95;
}
.hero-clock .hand-minute {
  stroke-width: 2.6;
  opacity: 0.95;
}
/* 秒针：主题色，最细 —— 这一小块里唯一的彩色 */
.hero-clock .hand-second {
  stroke: var(--accent);
  stroke-width: 1.3;
  opacity: 1;
}
.hero-clock .clock-pin {
  fill: currentColor;
  opacity: 0.9;
}

/* 竖线 + 图标 / 星期 / 一句小字 */
.hero-meta { display: flex; gap: 14px; }
.hero-meta-rule {
  width: 1px;
  flex-shrink: 0;
  background: currentColor;
  opacity: 0.32;
}
.hero-meta-body {
  display: flex;
  flex-direction: column;
  gap: 9px;
}
.hero-daynight {
  width: 21px;
  height: 21px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.hero-weekday {
  font-family: var(--font-mono);
  font-size: 12.5px;
  font-weight: 600;
  letter-spacing: 0.22em;
  /* 等宽字体最后一个字母后面也会留一份字距，补回来保持左边线对齐 */
  padding-right: 0.22em;
}
.hero-tagline {
  font-size: 11.5px;
  letter-spacing: 0.005em;
  opacity: 0.72;
  white-space: nowrap;
}

/* 底部的 SCROLL 提示没留：在这张图上它正好压在角色的围巾上，
   白字加阴影也读不清；参考图里那处背景是纯暗部，我们的构图不成立。
   不用它，看下面的波浪和露出来的白色区域，也知道要往下滚。 */

/* 社交链接已移到左侧名片卡里（见 ProfileCard.vue 的 .pc-social） */

/* 底部的滚动提示已删：白色区域上提之后它正好落在波浪交界上，反而碍事 */

@media (max-width: 760px) {
  /* 手机上首屏更矮一点，内容依旧靠左下；底部同样要给波浪留位置 */
  .hero {
    min-height: calc(100svh - 20px);
    padding: calc(var(--nav-h) + 24px) 0 112px;
  }
  .hero-inner { padding: 0 20px; }
  .hero-title-row { gap: 12px; margin-bottom: 10px; }
  .avatar { width: 56px; height: 56px; }
  .hero-desc { font-size: 15px; max-width: 100%; }
  /* 窄屏放不下右侧卷号块，隐藏 */
  .hero-aside { display: none; }
  .hero-cta { padding: 11px 20px; }
  .hero-cta-note { display: none; }
}
</style>
