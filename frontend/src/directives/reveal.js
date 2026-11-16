/**
 * v-reveal 滚动出现指令
 * 用法：<div v-reveal>…</div> 或 <div v-reveal="{ delay: 100 }">…</div>
 */
export default {
  mounted(el, binding) {
    el.classList.add('reveal')

    if (!('IntersectionObserver' in window)) {
      el.classList.add('visible')
      return
    }

    const delay = binding.value?.delay ?? 0
    if (delay > 0) el.style.transitionDelay = `${delay}ms`

    const io = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            el.classList.add('visible')
            io.unobserve(el)
          }
        })
      },
      { threshold: 0.12 }
    )
    io.observe(el)
  }
}
