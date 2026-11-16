/** 格式化日期：2026-08-27 12:00:00 → 2026-08-27 */
export function formatDate(value) {
  if (!value) return ''
  return String(value).slice(0, 10)
}

/** 格式化日期时间（精确到分钟）：2026-08-27 12:00:00 → 2026-08-27 12:00 */
export function formatDateTime(value) {
  if (!value) return ''
  return String(value).slice(0, 16).replace('T', ' ')
}
