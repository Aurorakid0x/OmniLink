import { createStore } from 'vuex'

export default createStore({
  state: {
    userInfo: null,
    backendUrl: 'https://localhost:8000'
  },
  getters: {
  },
  mutations: {
    setUserInfo(state, userInfo) {
      state.userInfo = userInfo
    }
  },
  actions: {
  },
  modules: {
  }
})
