import { ref } from 'vue'

const saved = localStorage.getItem('theme')
const theme = ref(
  saved || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
)

function apply() {
  document.documentElement.setAttribute('data-theme', theme.value)
}
apply()

export function useTheme() {
  function toggle() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    localStorage.setItem('theme', theme.value)
    apply()
  }
  return { theme, toggle }
}
