// ============================================================
// highlight.js 封装 —— 文章正文的代码高亮
//
// 只注册这个博客真正会用到的语言，避免把整包 190+ 种语言打进产物。
// 语言来源：<code class="language-xxx">；编辑器没记语言时用 highlightAuto
// 在下面这份候选里自动判断。
//
// 颜色不在 JavaScript 里写死：highlight.js 只负责给 token 加 class，
// 具体配色在 style.css 的 --code-* 变量里，深浅两套主题各自一套。
// ============================================================

import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import cpp from 'highlight.js/lib/languages/cpp'
import go from 'highlight.js/lib/languages/go'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import plaintext from 'highlight.js/lib/languages/plaintext'
import python from 'highlight.js/lib/languages/python'
import rust from 'highlight.js/lib/languages/rust'
import sql from 'highlight.js/lib/languages/sql'
import typescript from 'highlight.js/lib/languages/typescript'

hljs.registerLanguage('rust', rust)
hljs.registerLanguage('go', go)
hljs.registerLanguage('cpp', cpp)
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('bash', bash)
hljs.registerLanguage('python', python)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('json', json)
hljs.registerLanguage('plaintext', plaintext)

// 自动识别时的候选：不放 plaintext，否则它经常「赢」掉真语言
const SUBSET = ['rust', 'go', 'cpp', 'javascript', 'typescript', 'bash', 'python', 'sql', 'json']

// 编辑器 / 旧文章里可能出现的别名
const ALIAS = {
  rs: 'rust',
  golang: 'go',
  'c++': 'cpp',
  c: 'cpp',
  h: 'cpp',
  hpp: 'cpp',
  cc: 'cpp',
  js: 'javascript',
  jsx: 'javascript',
  ts: 'typescript',
  tsx: 'typescript',
  sh: 'bash',
  shell: 'bash',
  zsh: 'bash',
  py: 'python',
  python3: 'python',
  jsonc: 'json',
  text: 'plaintext',
  plain: 'plaintext'
}

export function normalizeLang(name) {
  if (!name) return ''
  const key = String(name).toLowerCase().trim()
  const lang = ALIAS[key] || key
  return hljs.getLanguage(lang) ? lang : ''
}

const escapeHtml = (s) =>
  String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')

/**
 * 把源码转成带 <span class="hljs-*"> 的 HTML。
 * @param {string} raw 源码
 * @param {string} lang 语言名（可为空 → 自动识别）
 */
export function highlightCode(raw, lang) {
  const code = String(raw == null ? '' : raw)
  const name = normalizeLang(lang)
  if (name && name !== 'plaintext') {
    try {
      return hljs.highlight(code, { language: name, ignoreIllegals: true }).value
    } catch {
      /* 掉到自动识别 */
    }
  }
  try {
    return hljs.highlightAuto(code, SUBSET).value
  } catch {
    return escapeHtml(code)
  }
}
