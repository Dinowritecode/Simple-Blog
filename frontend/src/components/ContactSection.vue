<template>
  <section class="section section-alt" id="contact">
    <div class="section-inner contact-inner">
      <h2 class="section-title" v-reveal><span class="accent">#</span> 联系</h2>

      <p class="contact-desc" v-reveal>
        欢迎交流技术、闲聊或一起做点有趣的东西。我通常会在 24 小时内回复。
      </p>

      <div class="contact-actions" v-reveal>
        <a class="btn btn-primary" href="mailto:zoo.to@qq.com">发送邮件</a>
        <a class="btn btn-ghost" href="https://github.com/Dinowritecode" target="_blank" rel="noopener">GitHub</a>
      </div>

      <p class="contact-note" v-reveal>或者直接扫描下方二维码 —— 微信 / 微信公众号</p>
    </div>
  </section>
</template>

<style scoped>
.contact-inner { text-align: center; }

.contact-desc {
  color: var(--text-dim);
  max-width: 520px;
  margin: 0 auto 36px;
}

.contact-actions { display: flex; gap: 14px; justify-content: center; margin-bottom: 24px; }

.contact-note {
  color: var(--text-dim);
  font-size: 13.5px;
}
</style>
