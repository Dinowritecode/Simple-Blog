import { ref, onMounted, onBeforeUnmount } from 'vue'

/**
 * 打字机效果
 * @param {string[]} words 依次展示的词组
 */
export function useTypewriter(words, opts = {}) {
  const { typeSpeed = 70, deleteSpeed = 36, hold = 1800, pause = 400 } = opts
  const text = ref('')
  let wordIndex = 0
  let charIndex = 0
  let deleting = false
  let timer = null

  function tick() {
    const word = words[wordIndex]
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
        wordIndex = (wordIndex + 1) % words.length
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
