<script setup>
// 首页：Hero（支持站长配置的背景图）+ 白色区域（博客 / 关于 / 技能 / 项目 / 联系）
// 个人名片贴在页面最左端、文章区之外：
//   · 滚到白色区域之前，它随页面一起滚动；
//   · 白色区域占满页面（hero 完全滚出视口）后改为 fixed，钉住不动，其余内容继续滑。
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import HeroSection from '../components/HeroSection.vue'
import PageBackdrop from '../components/PageBackdrop.vue'
import ProfileCard from '../components/ProfileCard.vue'
import NoticeCard from '../components/NoticeCard.vue'
import StatsCard from '../components/StatsCard.vue'
import CalendarCard from '../components/CalendarCard.vue'
import BlogSection from '../components/BlogSection.vue'
import AboutSection from '../components/AboutSection.vue'
import SkillsSection from '../components/SkillsSection.vue'
import ProjectsSection from '../components/ProjectsSection.vue'
import ContactSection from '../components/ContactSection.vue'
import { useSite } from '../composables/useSite'

// 首页背景图（来自 /api/settings，站长在后台「页面管理」配置）。
// 抽到共享 composable：导航栏也要用它判断文字该用白色还是主题色。
const { homeBg, heroNickname, heroDesc, heroRoles, heroAvatar, ownerNickname, ownerAvatar, owner } = useSite()

// 首屏的头像与昵称以「站长信息」为准：后台文案里显式填过就用文案，否则跟随站长账号
// 站长账号优先：在后台「设置」里改昵称 / 换头像，首页首屏立刻同步；
// 没填站长昵称时才退回「页面管理 → 首页文案」里的那份。
const heroName = computed(() => ownerNickname.value || heroNickname.value)
const heroPic = computed(() => ownerAvatar.value || heroAvatar.value)

// ---------- 背景图是否真的加载出来了 ----------
// 背景图在远端（图床 / CDN / 站长自己填的地址），可能挂了、被墙了、或者只是慢。
// 首屏文字只有在这张图**确实画出来**时才切白色；否则页面是浅色底，
// 白字会直接看不见（截图里整块白掉）。所以这里先预加载一次拿真实结果。
const bgOk = ref(false)

watch(
  homeBg,
  (url) => {
    bgOk.value = false
    if (!url) return
    const img = new Image()
    img.onload = () => {
      bgOk.value = img.naturalWidth > 0
    }
    img.onerror = () => {
      bgOk.value = false
    }
    img.src = url
  },
  { immediate: true }
)

// ---------- 名片吸顶 ----------
// docked = 白色区域是否已经占满页面（即 hero 整体滚出视口上沿）。
// 名片在未 docked 时是 absolute，正好贴在固定位置的同一坐标上，
// 所以切换瞬间不会有跳动。
const docked = ref(false)

function measureDock() {
  const hero = document.querySelector('.hero')
  docked.value = !!hero && hero.getBoundingClientRect().bottom <= 0
}

onMounted(() => {
  window.addEventListener('scroll', measureDock, { passive: true })
  window.addEventListener('resize', measureDock)
  nextTick(measureDock)
})
onBeforeUnmount(() => {
  window.removeEventListener('scroll', measureDock)
  window.removeEventListener('resize', measureDock)
})
</script>

<template>
  <main id="top">
    <!-- ① 固定背景图层：页面滚动时它纹丝不动（用「首页背景图」这张） -->
    <PageBackdrop :src="homeBg" />

    <!-- ② 首屏文字：背景图由上层固定图层透出来；文案/头像可在后台改 -->
    <HeroSection
      :bg="homeBg"
      :bg-ok="bgOk"
      :nickname="heroName"
      :desc="heroDesc"
      :roles="heroRoles"
      :avatar="heroPic"
    />

    <!-- ③ 毛玻璃区域：从下方滑上来盖住背景图。左栏名片、右栏公告+统计，正文居中 -->
    <div class="lower-wrap">
      <!-- 波浪交界：贴白色区域顶边上沿，颜色与面板一致；面板本身仍是矩形，
           所以玻璃的模糊只作用在矩形内，波浪只是「多出来的一条边」 -->
      <svg class="wave" viewBox="0 0 1440 80" preserveAspectRatio="none" aria-hidden="true">
        <!-- 三层波浪，同一个色相（面板白）只差浓度，所以叠在一起只会「更白」，
             不会出现两种颜色混出的脏色。三层的相位/周期各不相同，才有层叠感：
               第 1 层：最淡、最高，波峰在 x=180
               第 2 层：中等，和它反相（波峰在 540）
               第 3 层：不透明，周期更长（波峰在 360），也就是面板本色的那一层 -->
        <!-- 振幅 = |基准线 − 控制点| / 2：
             第 1 层 20 → 6 变成 20 → 3（振幅 14 → 17）
             第 2 层 40 → 54 变成 40 → 57（振幅 14 → 17）
             第 3 层 56 → 34 变成 56 → 34 保持不变的下沿：只把波峰抬到 34、波谷压到 78，
                   再大就会超出 SVG 底边被裁掉 -->
        <path class="wave-l1" d="M0,20 Q180,-14 360,20 T720,20 T1080,20 T1440,20 L1440,80 L0,80 Z" />
        <path class="wave-l2" d="M0,40 Q180,74 360,40 T720,40 T1080,40 T1440,40 L1440,80 L0,80 Z" />
        <path class="wave-l3" d="M0,56 Q360,12 720,56 T1440,56 L1440,80 L0,80 Z" />

        <!-- 主题色边压在最后一层的波线上 -->
        <path class="wave-line" d="M0,56 Q360,12 720,56 T1440,56" />
      </svg>

      <aside class="pc-rail" :class="{ docked }">
        <ProfileCard
        :name="heroName"
        :avatar="heroPic"
        :birthday="owner.birthday"
        :zodiac="owner.zodiac"
        :gender="owner.gender"
        :bio="owner.bio"
        :github="owner.github"
        :contact="owner.contact"
        :avatar-letter="heroName ? heroName.charAt(0) : '凛'"
      />
      </aside>

      <div class="lower">
        <BlogSection />
        <AboutSection />
        <SkillsSection />
        <ProjectsSection />
        <ContactSection />
      </div>

      <aside class="notice-rail" :class="{ docked }">
        <NoticeCard />
        <StatsCard />
        <CalendarCard />
      </aside>
    </div>
  </main>
</template>

<style scoped>
/* ============================================================
   首页纵向分四层：
     ① .bg-fixed    ：固定不动的背景图（页面滚动时它不动）
     ② .hero        ：首屏文字，背景图从固定图层透出来
     ③ .lower-wrap  ：定位参照 + 「三栏整体居中」的参数
        · .pc-rail     左栏：个人名片
        · .lower       中间：透明毛玻璃面板，从下方滑上来盖住背景图
        · .notice-rail 右栏：公告卡 + 全站统计卡（与左栏镜像对称）
   ★ 想调整整体布局，只改 .lower-wrap 里这三个变量：
     · --rail-width    左右两栏宽度（统一同一个值才能镜像对称）
     · --rail-gap      栏与正文之间的间距
     · --content-width 正文列宽（必须与 style.css 里 .section-inner 的 max-width 一致）
   当前 240 + 28 + 744 + 28 + 240 = 1280，
   导航栏内容区也按 1280 设置，因此三栏两端与导航栏完全对齐、左右留白相等。
   ============================================================ */

/* ① 背景图固定层已在 PageBackdrop 组件里（首页与文章详情页共用） */

/* ③ 定位参照：两栏绝对定位的基准，也是整体居中参数的出处。
   注意：这里绝不能加 backdrop-filter/filter/transform ——
   那会让它成为 fixed 后代的包含块，两栏的吸顶就失效了。 */
.lower-wrap {
  position: relative;
  --rail-width: 240px;
  --rail-gap: 28px;
  --content-width: 744px;
  /* 正文左右各让出一栏的位置，于是它居中在剩余空间里，
     外侧留白自然相等 → 三栏整体居中 */
  --content-inset-left: calc(var(--rail-width) + var(--rail-gap));
  --content-inset-right: calc(var(--rail-width) + var(--rail-gap));
  /* 左右两栏卡片与正文标题统一的起始高度：
     .lower 用它做 padding-top、两栏用它做 top，两边的顶端因此严格对齐。
     必须 >= 导航栏高度，否则吸顶时会被导航栏盖住。 */
  --panel-top: calc(var(--nav-h) + 12px);
}

/* 左右两栏：同一个 top、同一个宽度，一个靠左一个靠右 → 镜像对称。
   未固定时它们跟随页面滚动；毛玻璃区占满整页后只换定位方式，
   坐标全部沿用，所以切换瞬间不会位移。 */
.pc-rail,
.notice-rail {
  position: absolute;
  top: var(--panel-top);
  width: var(--rail-width);
  z-index: 5;
}
/* 外侧留白 = (总宽 - 正文宽 - 两栏宽 - 两个间距) / 2 */
.pc-rail {
  left: calc((100% - var(--content-width) - 2 * var(--rail-width) - 2 * var(--rail-gap)) / 2);
}
.notice-rail {
  right: calc((100% - var(--content-width) - 2 * var(--rail-width) - 2 * var(--rail-gap)) / 2);
  display: flex;
  flex-direction: column;
  gap: 18px; /* 公告卡与统计卡之间的间距 */
}

/* 毛玻璃区占满页面后：两栏换成 fixed 钉住，中间内容继续滑 */
.pc-rail.docked,
.notice-rail.docked {
  position: fixed;
}

/* 波浪交界：一条贴着白色区域顶边的 SVG。
   面板现在是**不透明**的，所以最后一层浪直接用面板本色填满，接缝完全看不出来。 */
.wave {
  position: absolute;
  left: 0;
  right: 0;
  /* 三层波浪整体压得低一些：带状区域从 74px 收到 60px，
     波峰的位置随之下移，蓝线仍然贴着面板上沿 */
  top: -59px;
  /* 高度比上移量多 1px：下沿压进面板 1px，避免出现一条缝 */
  height: 60px;
  width: 100%;
  display: block;
  pointer-events: none;
  z-index: 1;
}
/* 三层浪：同一个颜色（面板本色），只差浓度。
   注意用 --glass-panel-bg 而不是 --card —— 夜间模式下这两者不是同一个色
   （--card 是 #171c26，面板是 #0e1116），用 --card 会让最底下那层和面板对不上，
   交界处出现一条错位的分界。 */
.wave-l1 { fill: var(--glass-panel-bg); opacity: 0.34; }
.wave-l2 { fill: var(--glass-panel-bg); opacity: 0.68; }
.wave-l3 { fill: var(--glass-panel-bg); } /* 不透明，等于面板本色 */

/* 夜间模式：前两层用极淡的白，否则「深色压深色」等于看不见。
   第 3 层必须保持面板本色（它要和下面的面板严丝合缝），所以不动它。 */
html[data-theme="dark"] .wave-l1 { fill: rgba(255, 255, 255, 0.05); opacity: 1; }
html[data-theme="dark"] .wave-l2 { fill: rgba(255, 255, 255, 0.09); opacity: 1; }
/* 主题色边压在最后一层浪的波线上，把品牌色带到交界上 */
.wave-line {
  fill: none;
  stroke: var(--accent);
  stroke-width: 3;
  vector-effect: non-scaling-stroke;
}

/* ③ 不透明面板：白色区域不再透出背景图，交界处的三层浪才有「实心山丘」的体积感。
   首屏那段上边距由这里统一给，高度与左右两栏卡片一致 → 博客标题和名片顶端对齐。 */
.lower {
  background: var(--glass-panel-bg);
  backdrop-filter: var(--glass-panel-blur);
  -webkit-backdrop-filter: var(--glass-panel-blur);
  padding-top: var(--panel-top);
}

/* 面板内区块原本是不透明的 --bg / --bg-alt 底色，会挡住玻璃效果，这里全部清掉；
   同时把区块自身的上边距清零（上边距已由 .lower 的 padding-top 提供），
   这样各区块之间只剩下边距 → 页面更紧凑。 */
.lower :deep(.section),
.lower :deep(.section-alt) {
  background: transparent;
}
.lower :deep(.section) {
  padding-top: 0;
}

/* 窗口不够宽、三栏放不下时：正文恢复占满宽度，两栏回退成普通块上下排布 */
@media (max-width: 1339px) {
  .lower-wrap {
    --content-inset-left: 0px;
    --content-inset-right: 0px;
  }
  .pc-rail,
  .pc-rail.docked,
  .notice-rail,
  .notice-rail.docked {
    position: static;
    width: auto;
    max-width: 420px;
    margin: 40px auto 0;
  }
}
</style>
