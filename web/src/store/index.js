import { createStore } from 'vuex'

export default createStore({
  state: {
    userInfo: null,
    backendUrl: 'http://localhost:8080'
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
