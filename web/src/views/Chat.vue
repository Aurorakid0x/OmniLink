<template>
  <div class="chat-container">
    <!-- Background Ink Layer (Consistent with Login) -->
    <div class="ink-bg-layer"></div>

    <div class="main-layout glass-card">
      <SideBar 
        v-model:activeTab="activeTab" 
      />
      
      <SessionList 
        v-if="activeTab === 'chat'"
        :sessions="sessions" 
        :currentSessionId="currentSessionId"
        @select-session="handleSelectSession" 
      />
      
      <!-- Placeholder for Contacts/Settings -->
      <div v-else class="placeholder-panel glass-panel">
        <el-empty :description="activeTab === 'contacts' ? '联系人功能开发中' : '设置功能开发中'" />
      </div>

      <ChatWindow 
        :session="currentSession" 
        :messages="currentMessages"
        @send-message="handleSendMessage"
        @toggle-right-sidebar="showRightSidebar = !showRightSidebar"
      />

      <transition name="slide-fade">
        <GroupInfo 
          v-if="showRightSidebar && currentSession" 
          :session="currentSession"
        />
      </transition>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import SideBar from '../components/chat/SideBar.vue'
import SessionList from '../components/chat/SessionList.vue'
import ChatWindow from '../components/chat/ChatWindow.vue'
import GroupInfo from '../components/chat/GroupInfo.vue'

// State
const activeTab = ref('chat')
const currentSessionId = ref(1)
const showRightSidebar = ref(false)

// Mock Data
const sessions = ref([
  { 
    id: 1, 
    name: 'OmniLink 官方交流群', 
    avatar: 'https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png', 
    lastMsg: '欢迎加入 OmniLink!', 
    time: '10:30', 
    unread: 2,
    online: true 
  },
  { 
    id: 2, 
    name: '产品经理', 
    avatar: 'https://cube.elemecdn.com/3/7c/3ea6beec64369c2642b92c6726f1epng.png', 
    lastMsg: '需求文档已经更新了，看一下', 
    time: '09:15', 
    unread: 0,
    online: false 
  },
  { 
    id: 3, 
    name: '前端组', 
    avatar: 'https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png', 
    lastMsg: 'Code Review 时间定了吗？', 
    time: '昨天', 
    unread: 5,
    online: true 
  }
])

const messagesMap = ref({
  1: [
    { id: 1, type: 'text', content: '大家好，欢迎来到 OmniLink!', isMine: false, timestamp: Date.now() - 3600000 },
    { id: 2, type: 'text', content: '这个界面风格我很喜欢，很有质感。', isMine: true, timestamp: Date.now() - 3500000 },
    { id: 3, type: 'image', content: 'https://fuss10.elemecdn.com/e/5d/4a731a90594a4af544c0c25941171jpeg.jpeg', isMine: false, timestamp: Date.now() - 3400000 },
  ],
  2: [
    { id: 1, type: 'text', content: '需求文档更新了', isMine: false, timestamp: Date.now() - 86400000 },
  ],
  3: []
})

// Computed
const currentSession = computed(() => {
  return sessions.value.find(s => s.id === currentSessionId.value)
})

const currentMessages = computed(() => {
  return messagesMap.value[currentSessionId.value] || []
})

// Logic
const handleSelectSession = (session) => {
  currentSessionId.value = session.id
  // Clear unread
  const s = sessions.value.find(s => s.id === session.id)
  if (s) s.unread = 0
}

const handleSendMessage = (text) => {
  if (!currentSessionId.value) return
  
  const newMsg = {
    id: Date.now(),
    type: 'text',
    content: text,
    isMine: true,
    timestamp: Date.now()
  }
  
  if (!messagesMap.value[currentSessionId.value]) {
    messagesMap.value[currentSessionId.value] = []
  }
  messagesMap.value[currentSessionId.value].push(newMsg)

  // Mock Auto Reply
  setTimeout(() => {
    const replyMsg = {
      id: Date.now() + 1,
      type: 'text',
      content: `自动回复: 我收到了 "${text}"`,
      isMine: false,
      timestamp: Date.now()
    }
    messagesMap.value[currentSessionId.value].push(replyMsg)
  }, 1000)
}

// WebSocket Placeholder
onMounted(() => {
  console.log('Initialize WebSocket connection here...')
  // const ws = new WebSocket('ws://...')
  // ws.onmessage = (event) => { ... }
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

/* Ink Background Layer */
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
