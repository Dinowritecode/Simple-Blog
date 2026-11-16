<script setup>
// ============================================================
// MessengerWindow.vue —— 桃信小窗（主页上的浮层，不是独立页面）
//
// 动画顺序：
//   ① splash ：窗口「放大出现」——整张卡片从 0.56 倍放大到原尺寸（0 → 450ms）
//   ② intro  ：卡片中央的桃信图标 + 「桃信」文字【和窗口一起放大出现】
//              ★ 图标/文字在 .mz-window 内部，同属一棵变换子树，
//                所以窗口的 scale() 会把它们一起放大，天然同步、不需要单独延迟
//   ③ morph  ：粉色区域快速缩小上移，收成顶部粉色标题栏；
//              中央的图标文字原地缩小淡出（不飞向角落），
//              与此同时主界面与左侧灰色图标栏一起从左侧滑入
//              （★ 主界面不是等粉色上移完才渲染，粉色层只是盖在上面被「抽走」）
//   ④ panel  ：定格成完整界面（列表 / 资料卡 / 聊天）
//
// 关闭时反过来播一遍。
// ============================================================
import { computed, ref } from 'vue'
import { useAuth } from '../../composables/useAuth'
import { useMessenger } from '../../composables/useMessenger'
import logoUrl from '../../others/icon.svg'
import ChatPane from './ChatPane.vue'
import FriendList from './FriendList.vue'
import UserProfilePane from './UserProfilePane.vue'

const { user, displayName } = useAuth()
const { stage, view, listMode, unread, close, showFriends, showChats } = useMessenger()

// morph / panel 阶段才渲染主界面？不 —— 一打开就渲染，只是被粉色层盖住，
// 这样粉色层「抽走」的同时界面已经在那里了（参考录屏就是这个效果）。
const showBar = computed(() => stage.value === 'morph' || stage.value === 'panel')

const closing = ref(false)
function onClose() {
  closing.value = true
  close() // 状态机切到 morph，再在 400ms 后置为 closed
}
// 关闭动画播完（此时组件已被 v-if 卸载，这里只是兜底复位）
function onAnimEnd(e) {
  if (e.animationName === 'mz-window-out') closing.value = false
}

const myId = computed(() => user.value?.userId || '——')
const myLevel = computed(() => user.value?.level || 1)
</script>

<template>
  <!-- ★ 舞台层：只承载「阶段类」（stage-splash / stage-morph / stage-panel），本身不参与
       transform。窗口和开场图标层都是它的子元素，阶段类才能通过后代选择器命中它们。 -->
  <div class="mz-stage" :class="['stage-' + stage, { 'is-closing': closing }]">
    <!-- 开场遮罩：盖住整页，让首页内容退到后面；关闭时淡出。
         点空白处也能关窗（右下角小圆钮删掉之后，关窗就靠右上角 X 或点这里） -->
    <div class="mz-scrim" :class="{ on: stage !== 'closed' }" @click.self="onClose"></div>

    <div class="mz-window" @animationend="onAnimEnd">
      <!-- ③ 粉色区域：一开始铺满整张卡片，morph 时缩小上移成顶部标题栏 -->
      <div class="mz-pink"></div>

      <!-- ② 开场图标 + 文字。
           ★ 按需求放在 .mz-window【内部】：窗口入场动画给 .mz-window 加
             transform: scale()，子元素会被一起放大 —— 所以图标和文字是「跟着窗口
             一起放大出现」的，不再单独延迟。
           位置用 absolute + inset:0 + flex 居中，正好落在卡片正中央。 -->
      <div class="mz-splash">
        <img class="mz-splash-icon" :src="logoUrl" alt="" />
        <div class="mz-splash-title">Momo Talk</div>
      </div>

      <!-- ③ 顶部标题栏内容（粉色落位后淡入）：图标 + 桃信 + 关闭 -->
      <div class="mz-bar" :class="{ show: showBar }">
        <!-- 标题栏图标是「新出现」的一颗：等粉色区域收成标题栏时才弹出来，
             而不是从中央那颗图标飞过来（中央那颗是直接在原地缩没的） -->
        <img v-if="showBar" class="mz-bar-logo" :src="logoUrl" alt="" />
        <span class="mz-bar-title">Momo Talk</span>
        <span class="mz-bar-help" title="Momo Talk 是站内私信">?</span>
        <span class="mz-bar-spacer"></span>
        <button class="mz-bar-close" title="关闭" @click="onClose">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round">
            <path d="M6 6l12 12M18 6L6 18" />
          </svg>
        </button>
      </div>

      <!-- ④ 主界面：一开始就渲染好，只是被粉色层盖着 -->
      <div class="mz-body">
        <!-- 最左图标栏：选中项是「整条方框」高亮（方框式），不是圆角小按钮 -->
        <aside class="mz-side">
          <button
            class="mz-side-btn"
            :class="{ on: view === 'friends' && listMode === 'friends' }"
            :title="`成员（我：${displayName}）`"
            @click="showFriends"
          >
            <!-- 人形剪影（大头 + 梯形肩），与参考图一致 -->
            <svg viewBox="0 0 24 24" width="26" height="26" aria-hidden="true">
              <circle cx="12" cy="8" r="4.4" fill="currentColor" />
              <path d="M12 13.6c-4.3 0-7 2.9-7 6.2 0 .9.6 1.4 1.5 1.4h11c.9 0 1.5-.5 1.5-1.4 0-3.3-2.7-6.2-7-6.2z" fill="currentColor" />
            </svg>
          </button>
          <button
            class="mz-side-btn"
            :class="{ on: view === 'friends' && listMode === 'chats' }"
            title="未读信息"
            @click="showChats"
          >
            <!-- 对话气泡 + 三个点 + 左下小尾巴 -->
            <svg viewBox="0 0 24 24" width="26" height="26" aria-hidden="true">
              <path
                d="M4 4h16a1.6 1.6 0 0 1 1.6 1.6v9.2A1.6 1.6 0 0 1 20 16.4H9.6L6 20.4V16.4H4A1.6 1.6 0 0 1 2.4 14.8V5.6A1.6 1.6 0 0 1 4 4z"
                fill="currentColor"
              />
              <circle class="mz-side-dots" cx="8" cy="10.6" r="1.15" />
              <circle class="mz-side-dots" cx="12" cy="10.6" r="1.15" />
              <circle class="mz-side-dots" cx="16" cy="10.6" r="1.15" />
            </svg>
            <span v-if="unread.chat > 0" class="mz-side-dot"></span>
          </button>
          <span class="mz-side-spacer"></span>
          <span class="mz-side-lv" :title="`我的等级 ${myLevel}`">{{ myLevel }}</span>
        </aside>

        <!-- 列表列：聊天 / 资料卡时都保留在左边（聊天只在右侧空白区展开，
             不再占满整个窗口），选中项和会话列表始终看得见 -->
        <FriendList />

        <!-- 右侧区域：默认是空态提示，点成员 → 资料卡，点会话 → 聊天 -->
        <ChatPane v-if="view === 'chat'" />
        <UserProfilePane v-else-if="view === 'profile'" />
        <div v-else class="mz-hint">
          <img class="mz-hint-icon" :src="logoUrl" alt="" />
          <p>点击左边的成员查看资料<br />再按「私信」开始聊天</p>
          <p class="mz-hint-id">我的 Momo Talk ID：{{ myId }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* ============================================================
   尺寸变量：窗口是一张居中的卡片（不占满全屏、圆角已按要求去掉）
     --mz-w / --mz-h   卡片宽高（屏幕小的时候自动收窄）
     --mz-bar-h        顶部粉色标题栏高度
     --mz-icon         开场图标尺寸（标题栏 Logo 固定 17px，不再由它缩放而来）
     --mz-list-w       左侧列表列宽度（聊天在它右边展开）
   ============================================================ */
.mz-window {
  position: fixed;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  width: var(--mz-w);
  height: var(--mz-h);
  --mz-w: min(1020px, calc(100vw - 180px));
  --mz-h: min(620px, calc(100vh - 160px));
  --mz-bar-h: 44px;
  --mz-icon: 76px;
  --mz-list-w: 280px;
  z-index: 1600;
  overflow: hidden;
  /* 无圆角（按需求取消） */
  border-radius: 0;
  box-shadow: 0 26px 70px rgba(23, 29, 45, 0.28);
  font-family: var(--font-sans);
}
/* ① 窗口放大出现；关闭时整体缩小淡出。注意动画加在 .mz-window 上（不是 .mz-stage），
   所以开场图标层作为兄弟不会被带着一起缩放。
   时长 0.34s，缓动是「前快后慢」的 ease-out，起来干脆、收尾不拖。 */
.mz-stage.stage-splash .mz-window { animation: mz-window-in 0.34s cubic-bezier(0.16, 0.84, 0.34, 1) both; }
.mz-stage.is-closing .mz-window { animation: mz-window-out 0.26s ease-in both; }
@keyframes mz-window-in {
  from { transform: translate(-50%, -50%) scale(0.56); opacity: 0; }
  to   { transform: translate(-50%, -50%) scale(1); opacity: 1; }
}
@keyframes mz-window-out {
  from { transform: translate(-50%, -50%) scale(1); opacity: 1; }
  to   { transform: translate(-50%, -50%) scale(0.72); opacity: 0; }
}

/* 开场遮罩：整页淡暗 + 轻微模糊 */
.mz-scrim {
  position: fixed;
  inset: 0;
  z-index: 1500;
  background: rgba(24, 20, 26, 0.2);
  backdrop-filter: blur(2px);
  -webkit-backdrop-filter: blur(2px);
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.34s ease;
}
.mz-scrim.on { opacity: 1; }

/* ---------------- ③ 粉色区域 ---------------- */
/* 用 transition（不是 animation）：这样关闭时能自动倒放回满屏。
   打开时延迟 0.01s（下面 .mz-stage:not(.is-closing) 那条）：
   基本等于「粉色一铺满就开始收缩」，只剩约 1 帧的间隔。
   这个值不能再小了 —— 完全为 0 时浏览器会把「初始态 → 目标态」合并成
   同一次样式计算，过渡不触发，粉色会直接跳到终态（看不到收缩过程）。
   收缩本身 0.26s，利落收进标题栏。
   关闭时不加延迟，粉色立刻往回落，收尾才干脆。 */
.mz-pink {
  position: absolute;
  left: 0;
  top: 0;
  width: 100%;
  height: 100%;
  background: linear-gradient(170deg, #ff9db4 0%, #ec6484 55%, #d9506d 100%);
  z-index: 3; /* 盖在主界面之上，morph 时一边收一边把它「抽走」 */
  transition: height 0.26s cubic-bezier(0.4, 0, 0.2, 1);
}
.stage-morph .mz-pink,
.stage-panel .mz-pink { height: var(--mz-bar-h); }
/* 关闭时粉色立刻往回长满整张卡片，不然会被延迟拖住 */
.mz-stage:not(.is-closing) .mz-pink { transition-delay: 0.01s; }

/* ---------------- 标题栏（粉色落位后淡入） ---------------- */
.mz-bar {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  height: var(--mz-bar-h);
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 0 14px;
  z-index: 5;
  opacity: 0;
  transition: opacity 0.2s ease;
}
.mz-bar.show { opacity: 1; transition-delay: 0.12s; }
.mz-bar-logo {
  width: 21px;
  height: 21px;
  display: block;
  /* icon.svg 本身就是白色桃子剪影，压在粉色标题栏上正好，不需要再染色。
     上移的动作写在关键帧里（如下），否则会被动画的 transform 覆盖。 */
  animation: mz-bar-logo-in 0.18s cubic-bezier(0.34, 1.5, 0.64, 1) both;
}
/* 标题栏图标「短促地弹出来」：时间很短，和中央图标原地缩没错开一点点。
   translateY(-3px) 写在首尾帧里：剪影在方形 viewBox 里重心偏下
   （桃子主体偏下、叶尖朝上），按 flex 居中会显得比「Momo Talk」低，
   上提 3px 后两者的视觉中心落在一水平线上。
   动画带 both 填充，所以结束态也停在这个上移后的位置。 */
@keyframes mz-bar-logo-in {
  from { transform: translateY(-3px) scale(0.3); opacity: 0; }
  to   { transform: translateY(-3px) scale(1); opacity: 1; }
}
.mz-bar-title {
  color: #fff;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 1px;
}
.mz-bar-help {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.9);
  color: #e2567b;
  font-size: 11px;
  font-weight: 800;
}
.mz-bar-spacer { flex: 1; }
.mz-bar-close {
  border: none;
  background: none;
  color: #fff;
  cursor: pointer;
  display: inline-flex;
  padding: 4px;
  border-radius: 4px;
  transition: background 0.15s ease;
}
.mz-bar-close:hover { background: rgba(255, 255, 255, 0.18); }

/* ---------------- ② 开场图标 + 文字 ---------------- */
/* 按需求恢复成「和窗口一起放大出现」：
   这一层在 .mz-window 内部，所以窗口的 transform: scale() 会把它一起放大，
   不需要（也不该）再给它自己加延迟 —— 两者同属一棵变换子树，天然同步。

   位置：absolute + inset:0 铺满卡片 + flex 居中 → 正好是卡片正中央；
   z-index 3 盖在粉色层之上。
   可见性只跟阶段走：splash 显示，morph 时原地缩小淡出，panel 时整层淡出。 */
.mz-splash {
  position: absolute;
  inset: 0;
  z-index: 3;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  pointer-events: none;
  opacity: 1;
  transition: opacity 0.2s ease;
}
/* morph：粉色区域收缩时，图标和文字就在原地缩小淡出 ——
   不位移、不飞向右上角；右上角那颗是标题栏里另起的一颗新图标。
   时长与粉色收缩（0.28s）对齐，两件事一起结束，画面更干净。 */
.stage-morph .mz-splash {
  transform: scale(0.72);
  opacity: 0;
  transition: transform 0.2s ease-in, opacity 0.16s ease;
}
/* panel：主界面出来了，整层淡出让位 */
.stage-panel .mz-splash { opacity: 0; }
.mz-splash-icon {
  width: 76px; /* 写死尺寸：不依赖 .mz-window 上的 --mz-icon 变量（它继承不到） */
  height: 76px;
  display: block;
}
.mz-splash-title {
  color: #fff;
  font-size: 22px;
  font-weight: 800;
  /* 字距收紧：原来 6px 太散，且末尾多出一个字距会让整行看起来偏左 */
  letter-spacing: 1.5px;
  /* 末尾那份字距把排版中心推左了；再右移一些，让文字中线对准图标（桃子剪影）中线 */
  padding-left: 20px;
}

/* ---------------- ④ 主界面 ---------------- */
.mz-body {
  position: absolute;
  left: 0;
  right: 0;
  top: var(--mz-bar-h);
  bottom: 0;
  display: flex;
  background: #fff;
  overflow: hidden;
}

/* 列表列的宽度：FriendList 自己写死了 320px，这里统一收到一个变量上，
   窄屏再压缩，好给右侧的聊天区留出空间（聊天区是 flex:1，占满剩下的空白）。 */
.mz-window :deep(.list-col) { width: var(--mz-list-w); }
/* 聊天时右侧是白色面板，列表行贴边会显得顶手，稍微内收一点 */
.mz-window:has(.chat-pane) :deep(.list-col) { padding-left: 6px; padding-right: 6px; }

/* 左侧灰色图标栏：粉色层上移的同时从左侧滑入（关闭时再滑出去）。
   注意 padding-top: 0 —— 上面不留缝，第一个按钮（选中方框）正好和顶部粉色
   标题栏的下边缘紧贴，形成一整块连续的色块。 */
.mz-side {
  --mz-rail-bg: #6d7f97;   /* 图标栏底色（选中方框要在它上面提亮） */
  --mz-rail-on: #8a9bb1;   /* 选中方框的提亮底色 */
  width: 58px;
  flex-shrink: 0;
  background: var(--mz-rail-bg);
  display: flex;
  flex-direction: column;
  align-items: stretch;
  padding: 0 0 14px;
  /* 用 transform/opacity 过渡：进入向左滑入，退出自动倒放。
     稍微提前一点（0.06s）起步，和粉色收缩同时开始，不显得一段一段的 */
  transform: translateX(-100%);
  opacity: 0;
  transition: transform 0.26s cubic-bezier(0.22, 1, 0.36, 1) 0.06s,
              opacity 0.2s ease 0.06s;
}
.stage-morph .mz-side,
.stage-panel .mz-side {
  transform: none;
  opacity: 1;
}

/* 选中方框：正方形（58 × 58，与图标栏等宽等高）、直角、无圆角，只有底色变化。
   第一个按钮贴着图标栏顶端，正好和上面粉色标题栏紧贴。 */
.mz-side-btn {
  position: relative;
  width: 100%;
  height: 58px;
  flex-shrink: 0;
  border: none;
  border-radius: 0;
  background: none;
  color: rgba(255, 255, 255, 0.62);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  transition: background 0.16s ease, color 0.16s ease;
}
.mz-side-btn:hover { color: #fff; background: rgba(255, 255, 255, 0.08); }
/* 选中 = 一整块方框底色 + 纯白图标，左侧再加一条白色细杠强调边框感 */
.mz-side-btn.on {
  background: var(--mz-rail-on);
  color: #fff;
}
.mz-side-btn.on::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 3px;
  background: #fff;
}
.mz-side-btn svg { display: block; }
/* 聊天气泡里的三个点：用图标栏底色画出来，形成镂空效果 */
.mz-side-dots { fill: var(--mz-rail-on); }
.mz-side-btn:not(.on) .mz-side-dots { fill: var(--mz-rail-bg); }

.mz-side-dot {
  position: absolute;
  top: 9px;
  right: 12px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #ff5a8a;
  box-shadow: 0 0 0 2px var(--mz-rail-bg);
}
.mz-side-btn.on .mz-side-dot { box-shadow: 0 0 0 2px var(--mz-rail-on); }
.mz-side-spacer { flex: 1; }
.mz-side-lv {
  align-self: center;
  margin-top: 6px;
  font-family: var(--font-mono);
  font-size: 11px;
  color: #fff;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 2px;
  padding: 1px 8px;
}

/* 右侧空态 */
.mz-hint {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: #a9b2c0;
  font-size: 12.5px;
  line-height: 1.9;
  text-align: center;
  background: #fff;
}
.mz-hint-icon { width: 34px; height: 34px; opacity: 0.45; }
.mz-hint-id { font-family: var(--font-mono); color: #ff8ab0; }

@media (max-width: 760px) {
  /* 小屏幕上让卡片几乎占满，否则放不下「列表 + 聊天」两栏 */
  .mz-window {
    --mz-w: calc(100vw - 24px);
    --mz-h: calc(100vh - 96px);
    /* 手机宽度下把列表压窄，聊天区才不至于只剩一条缝 */
    --mz-list-w: 190px;
  }
  .mz-side { width: 50px; }
  .mz-window :deep(.mz-hint) { display: none; }
}
</style>
