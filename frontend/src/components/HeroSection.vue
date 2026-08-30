<script setup>
import { useTypewriter } from '../composables/useTypewriter'

const roles = [
  'Rust Developer',
  'Go Developer',
  'C++ Developer',
  'Systems Programmer',
  '性能强迫症患者'
]
const { text: typed } = useTypewriter(roles)
</script>

<template>
  <section class="hero">
    <div class="hero-inner" v-reveal>
      <img class="avatar" src="/pic/avatar.jpg" alt="凛冬RinEyce 的头像" />

      <p class="hero-greet">你好，我是</p>
      <h1 class="hero-name">凛冬<span class="accent">RinEyce</span></h1>

      <p class="hero-role">
        <span class="type-caret">&gt;</span>
        <span>{{ typed }}</span><span class="cursor">_</span>
      </p>

      <p class="hero-desc">
        热爱底层、性能与优雅的代码。用 Rust 写安全、用 Go 写并发、用 C++ 榨干每一分性能。
      </p>

      <div class="hero-actions">
        <a class="btn btn-primary" href="#contact">联系我</a>
        <a class="btn btn-ghost" href="#projects">看看项目</a>
      </div>

      <div class="hero-links">
        <a href="https://github.com/Dinowritecode" target="_blank" rel="noopener" aria-label="GitHub">
          <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.91.58.11.79-.25.79-.56 0-.28-.01-1.02-.02-2-3.2.7-3.87-1.54-3.87-1.54-.52-1.33-1.28-1.68-1.28-1.68-1.04-.71.08-.7.08-.7 1.15.08 1.76 1.18 1.76 1.18 1.03 1.76 2.7 1.25 3.36.96.1-.75.4-1.25.72-1.54-2.55-.29-5.23-1.28-5.23-5.68 0-1.26.45-2.28 1.18-3.09-.12-.29-.51-1.46.11-3.05 0 0 .96-.31 3.15 1.18a10.9 10.9 0 0 1 5.74 0c2.19-1.49 3.15-1.18 3.15-1.18.62 1.59.23 2.76.11 3.05.74.81 1.18 1.83 1.18 3.09 0 4.41-2.69 5.38-5.25 5.66.41.36.78 1.06.78 2.14 0 1.54-.01 2.79-.01 3.17 0 .31.21.68.8.56A10.52 10.52 0 0 0 23.5 12C23.5 5.65 18.35.5 12 .5z"/></svg>
        </a>
        <a href="mailto:zoo.to@qq.com" aria-label="邮箱">
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="m22 7-10 6L2 7"/></svg>
        </a>
      </div>
    </div>

    <div class="hero-scroll" aria-hidden="true">
      <span></span>
    </div>
  </section>
</template>

<style scoped>
.hero {
  min-height: 100svh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 120px 24px 80px;
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

.hero-inner { position: relative; max-width: 720px; }

.avatar {
  width: 128px;
  height: 128px;
  border-radius: 50%;
  object-fit: cover;
  border: 3px solid var(--bg);
  outline: 1px solid var(--border);
  box-shadow: var(--shadow);
  margin-bottom: 28px;
  transition: transform 0.4s ease, box-shadow 0.4s ease;
}
.avatar:hover {
  transform: translateY(-4px) scale(1.02);
  box-shadow: 0 0 0 4px var(--accent-soft), var(--shadow);
}

.hero-greet {
  color: var(--text-dim);
  font-size: 15px;
  letter-spacing: 4px;
  margin-bottom: 8px;
}

.hero-name {
  font-size: clamp(2.6rem, 8vw, 4.2rem);
  font-weight: 800;
  letter-spacing: 2px;
  line-height: 1.15;
  margin-bottom: 18px;
}

.hero-role {
  font-family: var(--font-mono);
  font-size: clamp(1rem, 3vw, 1.25rem);
  color: var(--text-dim);
  margin-bottom: 22px;
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
  margin: 0 auto 34px;
  color: var(--text-dim);
  font-size: 17px;
}

.hero-actions { display: flex; gap: 14px; justify-content: center; margin-bottom: 36px; }

.hero-links { display: flex; gap: 16px; justify-content: center; }
.hero-links a {
  display: inline-flex;
  width: 42px;
  height: 42px;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  border: 1px solid var(--border);
  color: var(--text-dim);
  transition: all 0.2s ease;
}
.hero-links a:hover {
  color: var(--accent);
  border-color: var(--accent);
  background: var(--accent-soft);
  transform: translateY(-2px);
  text-decoration: none;
}

.hero-scroll {
  position: absolute;
  bottom: 32px;
  left: 50%;
  transform: translateX(-50%);
  width: 24px;
  height: 40px;
  border: 2px solid var(--border);
  border-radius: 14px;
  display: flex;
  justify-content: center;
  padding-top: 8px;
}
.hero-scroll span {
  width: 3px;
  height: 8px;
  border-radius: 2px;
  background: var(--accent);
  animation: scroll 1.6s ease-in-out infinite;
}
@keyframes scroll {
  0% { transform: translateY(0); opacity: 1; }
  70% { transform: translateY(12px); opacity: 0; }
  100% { transform: translateY(0); opacity: 0; }
}

@media (max-width: 760px) {
  .hero { padding-top: 100px; }
}
</style>
