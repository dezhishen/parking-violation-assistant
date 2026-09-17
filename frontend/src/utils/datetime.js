// 本地时间格式化工具。
// 注意：不要使用 Date#toISOString()，它返回 UTC，会导致东八区出现 8 小时偏移。
function pad(n) {
  return String(n).padStart(2, '0')
}

/** 格式化为 YYYY-MM-DD */
export function formatDate(date) {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/** 格式化为 YYYY-MM-DD HH:mm:ss */
export function formatDateTime(date) {
  return `${formatDate(date)} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

/** 当前本地时间字符串 */
export function nowDateTimeString() {
  return formatDateTime(new Date())
}

/** 当月第一天 */
export function firstDayOfMonth(date = new Date()) {
  return formatDate(new Date(date.getFullYear(), date.getMonth(), 1))
}

/** 当月最后一天（取“下月第 0 天”，按本地时区计算） */
export function lastDayOfMonth(date = new Date()) {
  return formatDate(new Date(date.getFullYear(), date.getMonth() + 1, 0))
}
