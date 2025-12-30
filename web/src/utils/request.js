import axios from 'axios'
import store from '../store'
import { ElMessage } from 'element-plus'
import router from '../router'

// 创建 axios 实例
const service = axios.create({
  // 优先使用环境变量，否则回退到 store 中的配置，最后回退到 localhost
  // 注意：store.state.backendUrl 初始化可能较晚，但在组件调用时通常已就绪
  baseURL: import.meta.env.VITE_BACKEND_URL || 'http://localhost:8000',
  timeout: 10000 // 请求超时时间
})

// request 拦截器
service.interceptors.request.use(
  config => {
    // 动态获取 baseURL，防止 store 变化后 axios 实例没更新
    if (!config.baseURL || config.baseURL === 'http://localhost:8000') {
        config.baseURL = store.state.backendUrl || import.meta.env.VITE_BACKEND_URL || 'http://localhost:8000'
    }

    const token = store.state.token
    if (token) {
      config.headers['Authorization'] = 'Bearer ' + token
    }
    return config
  },
  error => {
    console.log(error) // for debug
    return Promise.reject(error)
  }
)

// response 拦截器
service.interceptors.response.use(
  response => {
    return response
  },
  error => {
    console.log('err' + error) // for debug
    if (error.response) {
      const { status } = error.response
      if (status === 401) {
        ElMessage.error('登录状态已过期，请重新登录')
        store.commit('clearAuth')
        router.push('/login')
      } else {
        // 允许业务层捕获错误进行优雅降级，这里只做通用提示
        // ElMessage.error(error.message || '请求失败')
      }
    } else {
       ElMessage.error('网络连接失败')
    }
    return Promise.reject(error)
  }
)

export default service
