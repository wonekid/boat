import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'

const request = axios.create({
  baseURL: '/api',
  timeout: 15000,
})

request.interceptors.request.use((config) => {
  const token = localStorage.getItem('boat_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

request.interceptors.response.use(
  (resp) => {
    const data = resp.data
    if (data.code === 0) {
      return data
    }
    if (resp.status === 401 || data.code === 1) {
      // 业务失败或鉴权失败
    }
    if (resp.status === 401) {
      ElMessage.error('登录已过期，请重新登录')
      localStorage.removeItem('boat_token')
      router.push('/login')
      return Promise.reject(data)
    }
    ElMessage.error(data.msg || '请求失败')
    return Promise.reject(data)
  },
  (error) => {
    if (error.response && error.response.status === 401) {
      localStorage.removeItem('boat_token')
      router.push('/login')
    }
    ElMessage.error(error.message || '网络异常')
    return Promise.reject(error)
  }
)

export default request
