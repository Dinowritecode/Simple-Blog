<script setup>
// 评论组件：文章页和说说页共用（样式与层级完全一致）。
//   · 文章：传 slug（公开短码）
//   · 说说：传 momentId
// 两者只能传一个，内部据此决定调哪组接口。
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../../api'
import { useAuth } from '../../composables/useAuth'
import { useToast } from '../../composables/useToast'
import { useMessenger } from '../../composables/useMessenger'
import { useSite } from '../../composables/useSite'
import LevelHeart from '../messenger/LevelHeart.vue'
import { formatDateTime } from '../../utils/format'

const props = defineProps({
  slug: { type: String, default: '' },
  momentId: { type: Number, default: 0 }
})
// 说说模式：接口按 id 走，返回结构（items/total）与文章评论完全一致
const isMoment = computed(() => props.momentId > 0)
const route = useRoute()
const router = useRouter()
const { isLoggedIn, user, displayName, avatarUrl } = useAuth()
const { toast } = useToast()
// Momo Talk：打开小窗并直接进入与某人的私信
const { open: openMessenger } = useMessenger()
// 功能开关：后台关掉 Momo Talk 后，评论区的「私信」按钮也一起隐藏
const { momoTalkEnabled } = useSite()

const items = ref([])
const total = ref(0)
const loading = ref(true)
const error = ref('')

const content = ref('')
const sending = ref(false)
// 正在回复的对象：null=发表顶级评论；否则 { id, name }
const replying = ref(null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const d = isMoment.value
      ? await api.listMomentComments(props.momentId)
      : await api.listComments(props.slug)
    items.value = d.items || []
    total.value = d.total || 0
    // 通知跳转：滚动到对应评论
    const cid = route.query.c
    if (cid) {
      setTimeout(() => {
        const el = document.getElementById('comment-' + cid)
        if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }, 300)
    }
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)

function startReply(c) {
  if (!isLoggedIn.value) return goLogin()
  replying.value = { id: c.id, name: c.author?.nickname || '用户' }
}
function cancelReply() { replying.value = null }

function goLogin() {
  router.push({ path: '/login', query: { next: route.fullPath } })
}

// 私信这条评论的作者：打开桃信小窗 → 显示对方资料卡 → 自动进入聊天。
// 未登录先引导登录；点到自己则不处理。
function privateMessage(author) {
  if (!author || !author.userId) return
  if (!isLoggedIn.value) return goLogin()
  if (user.value && author.id === user.value.id) return toast.info('这是你自己的评论呀')
  openMessenger({ userId: author.userId, autoChat: true })
}

async function submit() {
  if (!isLoggedIn.value) return goLogin()
  const text = content.value.trim()
  if (!text) return toast.error('评论内容不能为空')
  sending.value = true
  try {
    const payload = {
      content: text,
      parentId: replying.value ? replying.value.id : 0
    }
    if (isMoment.value) await api.createMomentComment(props.momentId, payload)
    else await api.createComment(props.slug, payload)
    content.value = ''
    replying.value = null
    await load()
    if (!replying.value) toast.success('评论成功')
  } catch (e) {
    toast.error(e.message)
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <section class="comments" :id="'comments'">
    <h2 class="comments-title">
      评论
      <span v-if="total > 0" class="comments-count">{{ total }}</span>
    </h2>

    <!-- 未登录提示 -->
    <div v-if="!isLoggedIn" class="comment-login-tip">
      登录后即可评论，与大家交流～
      <button class="btn-sm" @click="goLogin">去登录 / 注册</button>
    </div>

    <!-- 发表评论 / 回复框 -->
    <div v-else class="comment-box">
      <span class="avatar" style="--avatar-size: 36px">
        <img v-if="avatarUrl" :src="avatarUrl" alt="" />
        <span v-else class="avatar-letter">{{ (displayName || '?')[0] }}</span>
      </span>
      <div class="comment-box-main">
        <div v-if="replying" class="replying-to">
          回复 <b>@{{ replying.name }}</b>
          <button class="cancel-reply" @click="cancelReply">取消</button>
        </div>
        <textarea v-model="content" rows="3" maxlength="2000"
          :placeholder="replying ? `回复 @${replying.name}…` : '写下你的评论…（支持中文，最多 2000 字）'" />
        <div class="comment-box-foot">
          <span class="comment-as">评论身份：{{ displayName }}</span>
          <button class="btn-sm" :disabled="sending" @click="submit">
            {{ sending ? '发送中…' : replying ? '发送回复' : '发表评论' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="loading" class="comments-state">评论加载中…</div>
    <div v-else-if="error" class="comments-state comments-error">{{ error }}</div>
    <div v-else-if="!items.length" class="comments-state">还没有评论，来抢沙发吧～</div>

    <!-- 评论列表（顶级 + 回复） -->
    <div v-else class="comment-list">
      <div v-for="root in items" :key="root.id" class="comment-item" :id="'comment-' + root.id">
        <router-link v-if="root.author.id" class="avatar" style="--avatar-size: 38px" :to="`/u/${root.author.id}`" :title="`看 ${root.author.nickname || 'TA'} 的主页`">
          <img v-if="root.author.avatar" :src="root.author.avatar" alt="" />
          <span v-else class="avatar-letter">{{ (root.author.nickname || '?')[0] }}</span>
        </router-link>
        <span v-else class="avatar">
          <img v-if="root.author.avatar" :src="root.author.avatar" alt="" />
          <span v-else class="avatar-letter">{{ (root.author.nickname || '?')[0] }}</span>
        </span>
        <div class="comment-body">
          <div class="comment-meta">
            <router-link v-if="root.author.id" class="comment-author" :to="`/u/${root.author.id}`">{{ root.author.nickname || '用户' }}
              <i v-if="root.author.role === 'admin'" class="badge-admin">站长</i>
            </router-link>
            <span v-else class="comment-author">{{ root.author.nickname || '用户' }}
              <i v-if="root.author.role === 'admin'" class="badge-admin">站长</i>
            </span>
            <LevelHeart :level="root.author.level || 1" :size="18" />
            <span class="comment-time">{{ formatDateTime(root.createdAt) }}</span>
          </div>
          <p class="comment-content">{{ root.content }}</p>
          <div class="comment-ops">
            <button class="comment-reply-btn" @click="startReply(root)">回复</button>
            <button
              v-if="momoTalkEnabled && (!user || root.author.id !== user.id)"
              class="comment-reply-btn pm-btn"
              @click="privateMessage(root.author)"
            >私信</button>
          </div>

          <!-- 回复列表 -->
          <div v-if="root.replies && root.replies.length" class="reply-list">
            <div v-for="r in root.replies" :key="r.id" class="comment-item reply-item" :id="'comment-' + r.id">
              <router-link v-if="r.author.id" class="avatar" style="--avatar-size: 30px" :to="`/u/${r.author.id}`" :title="`看 ${r.author.nickname || 'TA'} 的主页`">
                <img v-if="r.author.avatar" :src="r.author.avatar" alt="" />
                <span v-else class="avatar-letter">{{ (r.author.nickname || '?')[0] }}</span>
              </router-link>
              <span v-else class="avatar" style="--avatar-size: 30px">
                <img v-if="r.author.avatar" :src="r.author.avatar" alt="" />
                <span v-else class="avatar-letter">{{ (r.author.nickname || '?')[0] }}</span>
              </span>
              <div class="comment-body">
                <div class="comment-meta">
                  <router-link v-if="r.author.id" class="comment-author" :to="`/u/${r.author.id}`">{{ r.author.nickname || '用户' }}
                    <i v-if="r.author.role === 'admin'" class="badge-admin">站长</i>
                  </router-link>
                  <span v-else class="comment-author">{{ r.author.nickname || '用户' }}
                    <i v-if="r.author.role === 'admin'" class="badge-admin">站长</i>
                  </span>
                  <LevelHeart :level="r.author.level || 1" :size="16" />
                  <span class="comment-time">{{ formatDateTime(r.createdAt) }}</span>
                </div>
                <p class="comment-content">
                  <span v-if="r.replyTo" class="reply-at">@{{ r.replyTo.nickname || '用户' }}</span>
                  {{ r.content }}
                </p>
                <div class="comment-ops">
                  <button class="comment-reply-btn" @click="startReply(r)">回复</button>
                  <button
                    v-if="momoTalkEnabled && (!user || r.author.id !== user.id)"
                    class="comment-reply-btn pm-btn"
                    @click="privateMessage(r.author)"
                  >私信</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
/* 分节线用实线：虚线是旧卡片版留下的，在白底正文里看着像草稿，
   而且和文末操作区那条 1px 实线不统一。 */
.comments {
  margin-top: 48px;
  padding-top: 28px;
  border-top: 1px solid var(--border);
}
.comments-title {
  font-size: 1.25rem;
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 22px;
}
.comments-count {
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--text-dim);
  background: var(--bg-alt);
  padding: 1px 8px;
  border-radius: 999px;
}

.comment-login-tip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  /* 原来是一圈蓝色虚线框，套在文章卡片里就成了「卡片里再画一个框」。
     改成一条干净的浅色横条：同样能让人看出这是提示区，但不额外画边界。 */
  background: color-mix(in srgb, var(--text) 4%, transparent);
  border: none;
  color: var(--text-dim);
  border-radius: 10px;
  padding: 14px 18px;
  margin-bottom: 20px;
  font-size: 14px;
  flex-wrap: wrap;
}

.comment-box { display: flex; gap: 12px; margin-bottom: 26px; }
.comment-box-main { flex: 1; min-width: 0; }
.comment-box-main textarea {
  width: 100%;
  padding: 11px 13px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg);
  color: var(--text);
  font-size: 14.5px;
  line-height: 1.7;
  outline: none;
  resize: vertical;
  font-family: inherit;
  box-sizing: border-box;
  transition: border-color 0.2s ease;
}
.comment-box-main textarea:focus { border-color: var(--accent-fill); }
.comment-box-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 8px;
  gap: 10px;
  flex-wrap: wrap;
}
.comment-as { font-size: 12.5px; color: var(--text-dim); }
.replying-to {
  font-size: 13px;
  color: var(--text-dim);
  background: var(--accent-soft);
  border-radius: 8px 8px 0 0;
  padding: 7px 12px;
}
.replying-to b { color: var(--accent-ink); }
.cancel-reply {
  margin-left: 10px;
  border: none;
  background: none;
  color: var(--text-dim);
  font-size: 12px;
  cursor: pointer;
}
.cancel-reply:hover { color: #e5484d; }

.comments-state { text-align: center; color: var(--text-dim); padding: 30px 0; font-size: 14px; }
.comments-error { color: #e5484d; }

.comment-list { display: flex; flex-direction: column; gap: 22px; }
.comment-item { display: flex; gap: 12px; scroll-margin-top: 80px; }
.comment-body { flex: 1; min-width: 0; }
.comment-meta { display: flex; align-items: baseline; gap: 10px; flex-wrap: wrap; }
/* 头像与昵称都是通往个人主页的链接：只加 hover 反馈，不改变原有排版 */
a.avatar { flex-shrink: 0; transition: box-shadow 0.18s ease, transform 0.18s ease; }
a.avatar:hover { transform: translateY(-1px); box-shadow: 0 0 0 2px var(--accent-soft); }
a.comment-author { color: inherit; transition: color 0.18s ease; }
a.comment-author:hover { color: var(--accent-ink); text-decoration: none; }
.comment-author { font-weight: 600; font-size: 14px; }
.comment-time { font-family: var(--font-mono); font-size: 12px; color: var(--text-dim); }
.comment-content {
  margin: 5px 0 4px;
  font-size: 14.5px;
  line-height: 1.8;
  word-break: break-word;
  white-space: pre-wrap;
}
.reply-at { color: var(--accent-ink); font-weight: 600; margin-right: 2px; }
.comment-ops { display: flex; align-items: center; gap: 14px; }
.comment-reply-btn {
  border: none;
  background: none;
  color: var(--text-dim);
  font-size: 12.5px;
  cursor: pointer;
  padding: 0;
}
.comment-reply-btn:hover { color: var(--accent-ink); }
/* 「私信」按钮用桃信的粉色，和回复区分开 */
.comment-reply-btn.pm-btn:hover { color: #ff5a8a; }

.reply-list {
  margin-top: 14px;
  padding-left: 14px;
  border-left: 2px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 14px;
}

@media (max-width: 560px) {
  .comment-box { flex-direction: column; }
}
</style>
