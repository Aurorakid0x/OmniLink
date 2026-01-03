import { createStore } from 'vuex'
import { normalizeUrl, getUserSessionList, getGroupSessionList, getNewContactList } from '../api/im'
import { normalizeSession, normalizeIncomingMessage } from '../utils/imNormalize'
import { ElNotification } from 'element-plus'

// 辅助函数：尝试解析 JSON
function safeJSONParse(str) {
  try {
    return JSON.parse(str)
  } catch (e) {
    return null
  }
}

const DEFAULT_BACKEND_URL = import.meta.env.VITE_BACKEND_URL || 'http://localhost:8000'

// 推断 WS URL (A3: 修复双斜杠问题 & 推断逻辑)
const inferWsUrl = (backendUrl) => {
    if (import.meta.env.VITE_WS_URL) {
        // 去除末尾斜杠
        return import.meta.env.VITE_WS_URL.replace(/\/+$/, '')
    }
    // 从 backendUrl 推断
    const url = backendUrl.replace(/\/+$/, '')
    return url.replace(/^http/, 'ws')
}

export default createStore({
  state: {
    userInfo: safeJSONParse(localStorage.getItem('userInfo')),
    token: localStorage.getItem('token') || '',
    backendUrl: DEFAULT_BACKEND_URL,
    wsUrl: inferWsUrl(DEFAULT_BACKEND_URL),
    
    // WebSocket 相关
    socket: null,
    isWsConnected: false,
    wsReconnectAttempts: 0,
    
    // 聊天数据
    sessionList: [], // 会话列表
    currentSessionId: null, // 当前选中的会话 ID (Session UUID)
    currentChatId: null, // 当前聊天对象 ID (User UUID or Group UUID) aka peerId
    
    // 缓存数据
    messageMap: {}, // peerId -> [message]
    unreadMap: {}, // peerId -> count
    sessionIdMap: {}, // peerId -> sessionId (用于快速查找)
    contactMap: {}, // peerId -> info (用户或群组的基本信息缓存)
    pendingApplyList: [], // 待处理好友申请
  },
  getters: {
    isAuthenticated: state => !!state.token,
    currentMessages: state => {
        if (!state.currentChatId) return []
        return state.messageMap[state.currentChatId] || []
    },
    totalUnread: state => {
        return Object.values(state.unreadMap).reduce((a, b) => a + b, 0)
    },
    pendingApplyCount: state => state.pendingApplyList.length
  },
  mutations: {
    setUserInfo(state, userInfo) {
      // 处理头像 URL
      if (userInfo && userInfo.avatar) {
        userInfo.avatar = normalizeUrl(userInfo.avatar)
      }
      state.userInfo = userInfo
      localStorage.setItem('userInfo', JSON.stringify(userInfo))
    },
    setToken(state, token) {
      state.token = token
      localStorage.setItem('token', token)
    },
    setBackendUrl(state, url) {
        state.backendUrl = url
        state.wsUrl = inferWsUrl(url)
    },
    clearAuth(state) {
      state.userInfo = null
      state.token = ''
      localStorage.removeItem('userInfo')
      localStorage.removeItem('token')
      // 断开 WS
      if (state.socket) {
          state.socket.close()
          state.socket = null
      }
    },
    
    // WS Mutations
    setSocket(state, socket) {
        state.socket = socket
    },
    setWsConnected(state, status) {
        state.isWsConnected = status
    },
    incrementWsReconnectAttempts(state) {
        state.wsReconnectAttempts++
    },
    resetWsReconnectAttempts(state) {
        state.wsReconnectAttempts = 0
    },

    // Chat Mutations
    setSessionList(state, list) {
        // A2: 使用 normalizeSession 归一化
        const normalizedList = list.map(item => normalizeSession(item))
        state.sessionList = normalizedList
        
        // 更新 sessionIdMap
        normalizedList.forEach(session => {
            if (session.peer_id) {
                state.sessionIdMap[session.peer_id] = session.session_id
            }
        })
    },
    setCurrentSession(state, { sessionId, peerId }) {
        state.currentSessionId = sessionId
        state.currentChatId = peerId
        // 清除未读 (A6: 基于 peerId)
        if (peerId) {
            state.unreadMap[peerId] = 0
        }
    },
    addMessage(state, rawMsg) {
        // A5: 健壮性处理与归一化
        try {
            const message = normalizeIncomingMessage(rawMsg, state.userInfo?.uuid)
            const peerId = message.peer_id

            if (!peerId) {
                console.warn('Message missing peer_id info, ignored:', rawMsg)
                return
            }

            if (!state.messageMap[peerId]) {
                state.messageMap[peerId] = []
            }
            
            // 简单追加
            state.messageMap[peerId].push(message)

            // 处理未读 (A6: 基于 peer_id)
            if (state.currentChatId !== peerId) {
                const count = state.unreadMap[peerId] || 0
                state.unreadMap[peerId] = count + 1
            }
            
            // 更新会话列表最后一条消息
            const session = state.sessionList.find(s => s.peer_id === peerId)
            if (session) {
                session.last_msg = message.type === 0 ? message.content : '[多媒体消息]'
                session.updated_at = message.created_at
                // 重新排序 sessionList (简单的把最新消息的会话置顶)
                const idx = state.sessionList.indexOf(session)
                if (idx > 0) {
                    state.sessionList.splice(idx, 1)
                    state.sessionList.unshift(session)
                }
            }
        } catch (e) {
            console.error('Error adding message:', e)
        }
    },
    setHistoryMessages(state, { peerId, messages }) {
        if (!messages) return
        const normalized = messages.map(m => normalizeIncomingMessage(m, state.userInfo?.uuid))
        state.messageMap[peerId] = normalized
    },
    prependHistoryMessages(state, { peerId, messages }) {
        if (!messages) return
        const normalized = messages.map(m => normalizeIncomingMessage(m, state.userInfo?.uuid))
        const cur = state.messageMap[peerId] || []
        state.messageMap[peerId] = normalized.concat(cur)
    },
    updateContactInfo(state, { id, info }) {
        state.contactMap[id] = info
    },
    setPendingApplyList(state, list) {
        state.pendingApplyList = list
    }
  },
  actions: {
    // A1: 实现 loadSessions action
    async loadSessions({ commit, state }) {
        if (!state.userInfo || !state.userInfo.uuid) return
        try {
            const ownerId = state.userInfo.uuid
            // 并行请求私聊和群聊会话
            const [userSessionsRes, groupSessionsRes] = await Promise.all([
                getUserSessionList(ownerId),
                getGroupSessionList(ownerId)
            ])

            let list = []
            if (userSessionsRes && userSessionsRes.data && userSessionsRes.data.data) {
                list = list.concat(userSessionsRes.data.data)
            }
            if (groupSessionsRes && groupSessionsRes.data && groupSessionsRes.data.data) {
                list = list.concat(groupSessionsRes.data.data)
            }
            
            // 按时间倒序排序
            list.sort((a, b) => {
                const t1 = new Date(a.updated_at || a.created_at).getTime()
                const t2 = new Date(b.updated_at || b.created_at).getTime()
                return t2 - t1
            })

            commit('setSessionList', list)
        } catch (e) {
            console.error('Failed to load sessions', e)
        }
    },

    async loadPendingApplies({ commit, state }) {
        if (!state.userInfo || !state.userInfo.uuid) return
        try {
            const res = await getNewContactList(state.userInfo.uuid)
            if (res.data && res.data.code === 200) {
                commit('setPendingApplyList', res.data.data || [])
            }
        } catch (e) {
            console.error('Failed to load pending applies', e)
        }
    },

    // 初始化并连接 WebSocket
    connectWebSocket({ state, commit, dispatch }) {
        if (import.meta.env.VITE_DISABLE_WS === 'true') return
        if (state.socket && state.socket.readyState === WebSocket.OPEN) return
        if (!state.userInfo || !state.userInfo.uuid) return

        // A3: WS URL 拼接修复
        const wsUrl = `${state.wsUrl}/wss?client_id=${state.userInfo.uuid}&token=${state.token}`
        console.log('Connecting to WS:', wsUrl)
        
        try {
            const socket = new WebSocket(wsUrl)
            commit('setSocket', socket)

            socket.onopen = () => {
                console.log('WS Connected')
                commit('setWsConnected', true)
                commit('resetWsReconnectAttempts')
            }

            socket.onmessage = (event) => {
                try {
                    const msg = JSON.parse(event.data)
                    console.log('WS Received:', msg)
                    
                    // A5: 健壮性处理
                    if (msg && typeof msg === 'object') {
                         // 处理好友申请事件
                         if (msg.type === 'contact.apply' || msg.type === 'friend_apply') {
                             ElNotification({
                                 title: '好友申请',
                                 message: '收到新的好友申请',
                                 type: 'info'
                             })
                             dispatch('loadPendingApplies')
                             return
                         }

                         commit('addMessage', msg)
                         
                         // A7: 如果是新会话（列表里没有），自动刷新会话列表
                         const myId = state.userInfo?.uuid
                         if (myId) {
                             const normalized = normalizeIncomingMessage(msg, myId)
                             const peerId = normalized.peer_id
                             if (peerId && !state.sessionList.find(s => s.peer_id === peerId)) {
                                 console.log('New session detected, reloading session list...')
                                 dispatch('loadSessions')
                             }
                         }
                    }
                } catch (e) {
                    console.error('WS Message Parse Error:', e)
                }
            }

            socket.onclose = () => {
                console.log('WS Closed')
                commit('setWsConnected', false)
                // 简单重连逻辑
                if (state.token) { // 只有在登录状态下才重连
                    const timeout = Math.min(1000 * (2 ** state.wsReconnectAttempts), 30000)
                    setTimeout(() => {
                        commit('incrementWsReconnectAttempts')
                        dispatch('connectWebSocket')
                    }, timeout)
                }
            }

            socket.onerror = (err) => {
                console.error('WS Error:', err)
            }
        } catch (e) {
            console.error('WS Connection Failed:', e)
        }
    },
    disconnectWebSocket({ state, commit }) {
        if (state.socket) {
            state.socket.close()
            commit('setSocket', null)
            commit('setWsConnected', false)
        }
    },
    
    sendMessage({ state }, messagePayload) {
        // messagePayload: { session_id, type, content, ... }
        if (state.socket && state.socket.readyState === WebSocket.OPEN) {
            state.socket.send(JSON.stringify(messagePayload))
        } else {
            console.error('WS not connected, message not sent')
            // 可以做消息队列缓存，待重连后发送
        }
    }
  },
  modules: {
  }
})
