import axios from 'axios'
import { ElMessage } from 'element-plus'

const http = axios.create({
  timeout: 30000
})

// 响应拦截器
http.interceptors.response.use(
  res => {
    if (res.data && res.data.code !== 0) {
      ElMessage.error(res.data.message || '操作失败')
      return Promise.reject(new Error(res.data.message))
    }
    return res.data.data ?? res.data
  },
  err => {
    ElMessage.error(err.response?.data?.message || err.message || '网络错误')
    return Promise.reject(err)
  }
)

// --- 车牌统计（主界面） ---
export function getPlateStats(params) {
  return http.get('/api/plates', { params })
}

export function getPlateDetail(plate) {
  return http.get(`/api/plates/${encodeURIComponent(plate)}`)
}

// --- 违停记录 ---
export function getRecords(params) {
  return http.get('/api/records', { params })
}

export function getRecord(id) {
  return http.get(`/api/records/${id}`)
}

export function createRecord(data) {
  return http.post('/api/records', data)
}

export function runOCR(image) {
  const formData = new FormData()
  formData.append('file', image)
  return http.post('/api/ocr', formData)
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
export function exportDetail(params) {
  const qs = new URLSearchParams(params).toString()
  window.open('/api/export/detail?' + qs)
}

export function exportSummary(params) {
  const qs = new URLSearchParams(params).toString()
  window.open('/api/export/summary?' + qs)
}
