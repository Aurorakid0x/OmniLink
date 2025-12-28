import { createStore } from 'vuex'

// 辅助函数：尝试解析 JSON，如果失败返回 null
function safeJSONParse(str) {
  try {
    return JSON.parse(str)
  } catch (e) {
    return null
  }
}

export default createStore({
  state: {
    // 初始化时尝试从 localStorage 获取
    userInfo: safeJSONParse(localStorage.getItem('userInfo')),
    token: localStorage.getItem('token') || '',
    backendUrl: 'http://localhost:8000'
  },
  getters: {
    isAuthenticated: state => !!state.token
  },
  mutations: {
    setUserInfo(state, userInfo) {
      state.userInfo = userInfo
      localStorage.setItem('userInfo', JSON.stringify(userInfo))
    },
    setToken(state, token) {
      state.token = token
      localStorage.setItem('token', token)
    },
    clearAuth(state) {
      state.userInfo = null
      state.token = ''
      localStorage.removeItem('userInfo')
      localStorage.removeItem('token')
    }
  },
  actions: {
  },
  modules: {
  }
})
