// 车牌号规则（与后端 internal/api/handlers.go 的正则保持一致）
export const PROVINCE_CHARS = '京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤青藏川宁琼'

// 完整车牌：省份简称 + 字母 + 5~6 位字母数字
const PLATE_RE = new RegExp(`^[${PROVINCE_CHARS}][A-Z][A-Z0-9]{5,6}$`)

/** 去掉空白与间隔符并转大写 */
export function normalizePlate(value) {
  return String(value ?? '')
    .replace(/[\s·•]/g, '')
    .toUpperCase()
}

/** 是否为合法车牌号 */
export function isPlateValid(value) {
  return PLATE_RE.test(normalizePlate(value))
}

/** Element Plus 表单校验器 */
export function plateValidator(rule, value, callback) {
  const plate = normalizePlate(value)
  if (!plate) {
    callback(new Error('请输入车牌号'))
    return
  }
  if (!isPlateValid(plate)) {
    callback(new Error('车牌号格式不正确，示例：粤A12345'))
    return
  }
  callback()
}
