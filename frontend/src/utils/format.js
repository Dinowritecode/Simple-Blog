/** 格式化日期：2026-08-27 12:00:00 → 2026-08-27 */
export function formatDate(value) {
  if (!value) return ''
  const s = String(value).slice(0, 10)
  return s
}
