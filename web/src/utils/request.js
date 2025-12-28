import axios from 'axios'
import store from '../store'
import { ElMessage } from 'element-plus'
import router from '../router'

// 创建 axios 实例
const service = axios.create({
  // 从 store 获取 baseURL，如果 store 还没初始化好，可以回退到默认值
  // 注意：这里 store.state.backendUrl 可能在初始化时还没准备好，
  // 但通常 request 是在组件中使用，此时 store 已准备好。
  // 为了安全，也可以在这里硬编码或者读取环境变量
  baseURL: 'https://localhost:8000', 
  timeout: 5000 // 请求超时时间
})

// request 拦截器
service.interceptors.request.use(
  config => {
    // 在发送请求之前做些什么
    const token = store.state.token
    if (token) {
      // 让每个请求携带 token
      // ['Authorization'] 是自定义头部 key
      // 请根据实际情况修改，例如 Bearer + token
      config.headers['Authorization'] = 'Bearer ' + token
    }
    return config
  },
  error => {
    // 对请求错误做些什么
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
        // 401 说明 token 过期或无效
        ElMessage.error('登录状态已过期，请重新登录')
        store.commit('clearAuth')
        router.push('/login')
      } else {
        ElMessage.error(error.message || '请求失败')
      }
    } else {
       ElMessage.error('网络连接失败')
    }
    return Promise.reject(error)
  }
)

export default service
