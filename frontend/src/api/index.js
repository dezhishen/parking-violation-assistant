import axios from 'axios'
import { ElMessage } from 'element-plus'

const http = axios.create({
  timeout: 30000
})

// 导出等二进制接口失败时，错误体是 Blob，需要还原其中的 JSON 提示
async function readBlobError(data) {
  try {
    const text = await data.text()
    return JSON.parse(text)?.message || ''
  } catch {
    return ''
  }
}

// 响应拦截器：统一提示错误，并标记 err.handled，业务层不要重复弹窗
http.interceptors.response.use(
  res => {
    // 二进制响应（导出）交给调用方处理
    if (res.config?.responseType === 'blob') {
      return res
    }
    if (res.data && res.data.code !== 0) {
      const err = new Error(res.data.message || '操作失败')
      err.handled = true
      ElMessage.error(err.message)
      return Promise.reject(err)
    }
    return res.data.data ?? res.data
  },
  async err => {
    let msg = err.message || '网络错误'
    const data = err.response?.data
    if (data instanceof Blob) {
      msg = (await readBlobError(data)) || msg
    } else if (data?.message) {
      msg = data.message
    }
    err.handled = true
    ElMessage.error(msg)
    return Promise.reject(err)
  }
)

// --- 车牌统计（主界面） ---
export function getPlateStats(params) {
  return http.get('/api/plates', { params })
}

export function getPlateDetail(plate, params) {
  return http.get(`/api/plates/${encodeURIComponent(plate)}`, { params })
}

// --- 违停记录 ---
export function getRecords(params) {
  return http.get('/api/records', { params })
}

export function createRecord(data) {
  return http.post('/api/records', data)
}

// 传 File/Blob 时走 multipart 上传；传已上传的图片路径（如 /uploads/xxx.jpg）时走 JSON，
// 避免同一张图片被重复上传一次。
export function runOCR(image) {
  if (typeof image === 'string' && image) {
    return http.post('/api/ocr', { image_path: image })
  }
  const formData = new FormData()
  formData.append('file', image)
  return http.post('/api/ocr', formData)
}

// 丢弃已上传但最终没有创建记录的图片，避免留下孤儿文件
export function discardUpload(url) {
  return http.post('/api/uploads/discard', { url })
}

export function updateRecord(id, data) {
  return http.put(`/api/records/${id}`, data)
}

export function deleteRecord(id) {
  return http.delete(`/api/records/${id}`)
}

// --- 首页统计 ---
export function getDashboard() {
  return http.get('/api/dashboard')
}

// --- 导出 ---
function pad(n) {
  return String(n).padStart(2, '0')
}

function timestamp() {
  const d = new Date()
  return `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}_${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`
}

function parseFilename(disposition, fallback) {
  if (!disposition) return fallback
  const utf8 = disposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8?.[1]) {
    try {
      return decodeURIComponent(utf8[1])
    } catch {
      // 忽略解码失败，继续尝试普通文件名
    }
  }
  const plain = disposition.match(/filename="?([^";]+)"?/i)
  return plain?.[1] || fallback
}

async function downloadFile(path, params, fallbackName) {
  const res = await http.get(path, { params, responseType: 'blob' })
  const filename = parseFilename(res.headers['content-disposition'], fallbackName)
  const url = URL.createObjectURL(res.data)
  try {
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
  } finally {
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }
}

export function exportDetail(params) {
  return downloadFile('/api/export/detail', params, `违停详细记录_${timestamp()}.xlsx`)
}

export function exportSummary(params) {
  return downloadFile('/api/export/summary', params, `违停统计记录_${timestamp()}.xlsx`)
}
