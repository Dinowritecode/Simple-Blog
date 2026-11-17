import { ref, onMounted, onBeforeUnmount } from 'vue'

/**
 * 打字机效果
 * @param {string[]|() => string[]} words 依次展示的词组；传函数可以动态取
 *        （首页文案是后台可改的、且可能在组件挂载后才请求回来，所以支持每次现取）
 */
export function useTypewriter(words, opts = {}) {
  const { typeSpeed = 70, deleteSpeed = 36, hold = 1800, pause = 400 } = opts
  const text = ref('')
  let wordIndex = 0
  let charIndex = 0
  let deleting = false
  let timer = null

  // 取当前词组列表：传数组就用数组，传函数就每次调用
  function list() {
    const w = typeof words === 'function' ? words() : words
    return Array.isArray(w) && w.length ? w : ['']
  }

  function tick() {
    const seq = list()
    if (wordIndex >= seq.length) wordIndex = 0
    const word = seq[wordIndex] || ''
    if (!deleting) {
      charIndex++
      text.value = word.slice(0, charIndex)
      if (charIndex === word.length) {
        deleting = true
        timer = setTimeout(tick, hold)
        return
      }
      timer = setTimeout(tick, typeSpeed)
    } else {
      charIndex--
      text.value = word.slice(0, charIndex)
      if (charIndex === 0) {
        deleting = false
        wordIndex = (wordIndex + 1) % seq.length
        timer = setTimeout(tick, pause)
        return
      }
      timer = setTimeout(tick, deleteSpeed)
    }
  }

  onMounted(() => { timer = setTimeout(tick, 300) })
  onBeforeUnmount(() => clearTimeout(timer))

  return { text }
}
