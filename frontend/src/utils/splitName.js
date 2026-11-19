// ============================================================
// splitName.js —— 昵称「蓝白分色」的切分规则
//
// 站长可以在昵称里用 | 指定分界，例如：
//   凛冬|RinEyce   →  「凛冬」用主题色，「RinEyce」用正文色
// 没写分隔符时自动退化成「中文前缀 + 拉丁后缀」切一刀，
// 这样「凛冬RinEyce」也能得到同样的效果，不写分隔符也不会整行一个颜色。
// ============================================================

const CJK = /^([\u2e80-\u9fff\u3040-\u30ff\uac00-\ud7af]+)([\s\S]*)$/

export function splitName(raw) {
  const text = String(raw || '').trim()
  if (!text) return { pre: '', main: '' }

  const bar = text.indexOf('|')
  if (bar >= 0) {
    return { pre: text.slice(0, bar), main: text.slice(bar + 1) }
  }

  const m = text.match(CJK)
  if (m && m[2]) return { pre: m[1], main: m[2] }

  return { pre: '', main: text }
}

// 单字头像用的首字（中文取一个字，英文取大写首字母）
export function firstLetter(raw) {
  const t = String(raw || '').trim()
  return (t.charAt(0) || '?').toUpperCase()
}