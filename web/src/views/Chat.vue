<template>
  <div class="chat-container">
    <div class="ink-bg-layer"></div>

    <div class="main-layout glass-card">
      <SideBar 
        v-model:activeTab="activeTab" 
      />
      
      <!-- Session List Panel -->
      <SessionList 
        v-if="activeTab === 'chat'"
        @select-session="handleSelectSession" 
        @show-create-group="showCreateGroup = true"
      />
      
      <!-- Contact List Panel -->
      <ContactList 
        v-else-if="activeTab === 'contacts'"
        @start-chat="handleStartChatFromContact"
      />

      <!-- Placeholder for Me/Settings -->
      <div v-else class="placeholder-panel glass-panel">
        <div class="me-panel" v-if="activeTab === 'me'">
             <el-avatar :src="normalizeUrl(userInfo.avatar)" :size="100" />
             <h2>{{ userInfo.nickname }}</h2>
             <p>ID: {{ userInfo.uuid }}</p>
        </div>
        <el-empty v-else description="功能开发中" />
      </div>

      <!-- Chat Window -->
      <ChatWindow 
        :session="currentSession" 
        :messages="currentMessages"
        @send-message="handleSendMessage"
        @toggle-right-sidebar="showRightSidebar = !showRightSidebar"
        @load-more="handleLoadMore"
      />

      <!-- Right Info Panel -->
      <transition name="slide-fade">
        <GroupInfo 
          v-if="showRightSidebar && currentSession" 
          :session="currentSession"
        />
      </transition>
      
      <CreateGroupDialog 
        v-model:visible="showCreateGroup"
        @success="handleCreateGroupSuccess"
      />
      
      <!-- Agent管理弹窗 -->
      <AgentManageDialog v-model:visible="showAgentManage" />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useStore } from 'vuex'
import SideBar from '../components/chat/SideBar.vue'
import SessionList from '../components/chat/SessionList.vue'
import ContactList from '../components/chat/ContactList.vue'
import ChatWindow from '../components/chat/ChatWindow.vue'
import GroupInfo from '../components/chat/GroupInfo.vue'
import CreateGroupDialog from '../components/chat/CreateGroupDialog.vue'
import AgentManageDialog from '../components/chat/AgentManageDialog.vue'
import { getMessageList, getGroupMessageList, normalizeUrl } from '../api/im'
import { getSessionMessages } from '../api/ai'
import { normalizeSession } from '../utils/imNormalize'

const HISTORY_PAGE_SIZE = 20

const store = useStore()

// State
const activeTab = ref('chat')
const showRightSidebar = ref(false)
const showCreateGroup = ref(false)

const userInfo = computed(() => store.state.userInfo)
const currentSessionId = computed(() => store.state.currentSessionId)
const currentSession = computed(() => {
    return store.state.sessionList.find(s => s.session_id === currentSessionId.value)
})
const currentMessages = computed(() => store.getters.currentMessages)
const showAgentManage = computed(() => store.state.showAgentManage)

// History paging (per peer)
const historyPageMap = ref({})
const historyNoMoreMap = ref({})

// Logic
const handleSelectSession = (session) => {
  if (session.type === 'ai') {
    // AI会话
    store.commit('setCurrentSession', { 
      sessionId: session.session_id, 
      peerId: null, // AI会话无peerId
      isAISession: true
    })
    loadAIMessages(session.session_id)
  } else {
    // IM会话（现有逻辑保持不变）
    const peerId = session.peer_id
    store.commit('setCurrentSession', { 
        sessionId: session.session_id, 
        peerId: peerId 
    })

    historyPageMap.value[peerId] = 1
    historyNoMoreMap.value[peerId] = false

    loadHistoryMessages(peerId, 1, false)
  }
}

const handleStartChatFromContact = (data) => {
    // data: { session_id, receive_id, ... }
    // 切换到 chat tab
    activeTab.value = 'chat'
    
    // 检查 sessionList 是否已有该会话
    const exist = store.state.sessionList.find(s => s.session_id === data.session_id)
    if (exist) {
        handleSelectSession(exist)
    } else {
        // A1: 修复 loadSessions 调用
        store.dispatch('loadSessions').then(() => {
             // 重新获取
             const reExist = store.state.sessionList.find(s => s.session_id === data.session_id)
             if (reExist) {
                 handleSelectSession(reExist)
             } else {
                 // 手动 commit 一个归一化的 session (优雅降级)
                 const rawSession = {
                     session_id: data.session_id,
                     user_id: data.receive_id.startsWith('U') ? data.receive_id : null,
                     group_id: data.receive_id.startsWith('G') ? data.receive_id : null,
                     peer_id: data.receive_id,
                     username: data.username,
                     group_name: data.group_name,
                     avatar: data.avatar,
                     updated_at: new Date().toISOString()
                 }
                 const newSession = normalizeSession(rawSession)
                 const newList = [newSession, ...store.state.sessionList]
                 store.commit('setSessionList', newList)
                 handleSelectSession(newSession)
             }
        })
    }
}

const handleCreateGroupSuccess = (session) => {
    handleSelectSession(session)
}

const loadHistoryMessages = async (peerId, page, prepend) => {
    try {
        let res;
        if (peerId.startsWith('G')) {
             res = await getGroupMessageList({
                group_id: peerId,
                page,
                page_size: HISTORY_PAGE_SIZE,
            })
        } else {
            res = await getMessageList({
                user_one_id: userInfo.value.uuid,
                user_two_id: peerId,
                page,
                page_size: HISTORY_PAGE_SIZE,
            })
        }

        const list = (res.data && res.data.data) ? res.data.data : []
        if (prepend) {
            store.commit('prependHistoryMessages', { peerId, messages: list })
        } else {
            store.commit('setHistoryMessages', { peerId, messages: list })
        }

        if (!list || list.length < HISTORY_PAGE_SIZE) {
            historyNoMoreMap.value[peerId] = true
        }
    } catch (e) {
        console.error('Fetch history failed', e)
    }
}

const handleLoadMore = async () => {
    if (!currentSession.value) return
    const peerId = currentSession.value.peer_id
    if (!peerId) return
    if (historyNoMoreMap.value[peerId]) return

    const curPage = historyPageMap.value[peerId] || 1
    const nextPage = curPage + 1
    historyPageMap.value[peerId] = nextPage

    await loadHistoryMessages(peerId, nextPage, true)
}

// 加载AI会话消息
const loadAIMessages = async (sessionId) => {
  try {
    const res = await getSessionMessages(sessionId, { limit: 100, offset: 0 })
    if (res.data && res.data.code === 200) {
      store.commit('setAIMessages', { sessionId, messages: res.data.data.messages || [] })
    }
  } catch (error) {
    console.error('Failed to load AI messages:', error)
  }
}

const handleSendMessage = (payload) => {
    // payload: { type, content, url, ... }
    if (!currentSession.value) return
    
    const peerId = currentSession.value.peer_id
    
    const msgData = {
        session_id: currentSessionId.value,
        type: payload.type,
        content: payload.content || '',
        url: payload.url || '',
        send_id: userInfo.value.uuid,
        send_name: userInfo.value.nickname,
        send_avatar: userInfo.value.avatar, 
        receive_id: peerId,
        file_name: payload.file_name || '',
        file_size: payload.file_size || '',
        file_type: payload.file_type || ''
    }
    
    store.dispatch('sendMessage', msgData)
}

// WebSocket Lifecycle
onMounted(() => {
  store.dispatch('connectWebSocket')
})
</script>

<style scoped>
.chat-container {
  height: 100vh;
  width: 100vw;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #f5f7fa;
  background-image: 
    radial-gradient(at 10% 10%, rgba(0,0,0,0.08) 0px, transparent 50%),
    radial-gradient(at 90% 90%, rgba(0,0,0,0.05) 0px, transparent 50%),
    linear-gradient(135deg, #ffffff 0%, #e6e9f0 100%);
  overflow: hidden;
  position: relative;
}

.ink-bg-layer {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 0;
  opacity: 0.6;
  background-image: 
    radial-gradient(circle at 30% 40%, rgba(0,0,0,0.1) 0%, transparent 40%),
    radial-gradient(circle at 70% 20%, rgba(0,0,0,0.08) 0%, transparent 35%);
  filter: blur(40px);
  pointer-events: none;
}

.main-layout {
  position: relative;
  z-index: 10;
  width: 90vw;
  height: 90vh;
  max-width: 1400px;
  background: rgba(255, 255, 255, 0.4);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.6);
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.1);
  border-radius: 20px;
  display: flex;
  overflow: hidden;
}

.placeholder-panel {
  width: 280px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-right: 1px solid rgba(255, 255, 255, 0.3);
  background: rgba(255, 255, 255, 0.3);
}

.me-panel {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
}

/* Transition for Right Sidebar */
.slide-fade-enter-active,
.slide-fade-leave-active {
  transition: all 0.3s ease;
}

.slide-fade-enter-from,
.slide-fade-leave-to {
  transform: translateX(20px);
  opacity: 0;
  width: 0;
  padding: 0;
}
</style>
