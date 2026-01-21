<template>
  <div class="assistant-container">
    <div class="ink-bg-layer"></div>

    <div class="main-layout glass-card">
      <!-- Session List Panel -->
      <div class="session-list glass-panel">
        <div class="header">
          <h3 class="title">AI 助手</h3>
          <el-button circle icon="Plus" class="new-chat-btn" @click="handleNewChat" />
        </div>

        <div class="list-content custom-scrollbar">
          <el-empty v-if="sessions.length === 0" description="暂无会话" :image-size="60" />
          
          <div 
            v-for="item in sessions" 
            :key="item.session_id" 
            class="session-item"
            :class="{ active: currentSessionId === item.session_id }"
            @click="handleSelectSession(item)"
          >
            <div class="session-icon">
              <el-icon><MagicStick /></el-icon>
            </div>
            
            <div class="item-info">
              <div class="item-top">
                <span class="name">{{ item.title || '新对话' }}</span>
                <span class="time">{{ formatTime(item.updated_at) }}</span>
              </div>
              <div class="item-msg text-ellipsis">
                {{ item.summary || item.last_message || '点击开始对话' }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Chat Window -->
      <div class="chat-window glass-panel">
        <!-- Header -->
        <div class="chat-header">
          <div class="header-info">
            <span class="chat-name">AI 个人助手</span>
          </div>
          <div class="header-actions">
            <el-select 
              v-model="selectedAgentId" 
              placeholder="选择 Agent"
              class="agent-selector"
              size="small"
            >
              <el-option
                v-for="agent in agents"
                :key="agent.agent_id"
                :label="agent.name"
                :value="agent.agent_id"
              />
            </el-select>
          </div>
        </div>

        <!-- Message List -->
        <div class="message-area custom-scrollbar" ref="msgListRef">
          <div v-for="(msg, idx) in currentMessages" :key="idx" class="message-row" :class="{ 'is-mine': msg.role === 'user' }">
            <div class="msg-avatar">
              <el-avatar v-if="msg.role === 'user'" :src="userAvatar" :size="36">
                {{ userName ? userName[0] : 'U' }}
              </el-avatar>
              <div v-else class="ai-avatar">
                <el-icon><MagicStick /></el-icon>
              </div>
            </div>
            
            <div class="msg-content-wrapper">
              <div class="msg-bubble" :class="{ 'ai-bubble': msg.role === 'assistant' }">
                <div class="msg-text">{{ msg.content }}</div>
                
                <!-- Citations (only for assistant messages) -->
                <div v-if="msg.role === 'assistant' && msg.citations && msg.citations.length > 0" class="citations-section">
                  <el-collapse accordion>
                    <el-collapse-item>
                      <template #title>
                        <div class="citations-title">
                          <el-icon><Document /></el-icon>
                          <span>引用来源 ({{ msg.citations.length }})</span>
                        </div>
                      </template>
                      <div class="citations-list">
                        <div v-for="(cite, cIdx) in msg.citations" :key="cIdx" class="citation-card">
                          <div class="citation-header">
                            <el-tag size="small" type="info">{{ cite.source_type }}</el-tag>
                            <span class="citation-score">相似度: {{ (cite.score * 100).toFixed(1) }}%</span>
                          </div>
                          <div class="citation-content">{{ cite.content }}</div>
                          <div class="citation-meta">
                            <span>来源: {{ cite.source_key }}</span>
                            <span>Chunk: {{ cite.chunk_id }}</span>
                          </div>
                        </div>
                      </div>
                    </el-collapse-item>
                  </el-collapse>
                </div>
              </div>
            </div>
          </div>

          <!-- Streaming indicator -->
          <div v-if="isStreaming" class="message-row">
            <div class="ai-avatar">
              <el-icon><MagicStick /></el-icon>
            </div>
            <div class="msg-content-wrapper">
              <div class="msg-bubble ai-bubble streaming">
                <el-icon class="is-loading"><Loading /></el-icon>
                <span>AI 正在思考...</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Input Area -->
        <div class="input-area">
          <el-input
            v-model="inputText"
            type="textarea"
            :rows="3"
            resize="none"
            placeholder="输入问题... (Shift+Enter 换行, Enter 发送)"
            @keydown.enter.exact.prevent="handleSend"
            :disabled="isStreaming"
          />
          
          <div class="send-actions">
            <span class="tip">Shift+Enter 换行 · Enter 发送</span>
            <el-button 
              type="primary" 
              round 
              @click="handleSend" 
              :disabled="!inputText.trim() || isStreaming"
              :loading="isStreaming"
            >
              发送
            </el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useStore } from 'vuex'
import { MagicStick, Plus, Document, Loading } from '@element-plus/icons-vue'
import { getSessions, getAgents, chatStream, getSessionMessages } from '../api/ai'
import { ElMessage } from 'element-plus'

const store = useStore()

// State
const sessions = ref([])
const agents = ref([])
const currentSessionId = ref(null)
const selectedAgentId = ref(null)
const currentMessages = ref([])
const inputText = ref('')
const isStreaming = ref(false)
const msgListRef = ref(null)

// User info
const userInfo = computed(() => store.state.userInfo)
const userName = computed(() => userInfo.value?.nickname || '')
const userAvatar = computed(() => userInfo.value?.avatar || '')

// Format time
const formatTime = (timeStr) => {
  if (!timeStr) return ''
  const date = new Date(timeStr)
  const now = new Date()
  if (date.toDateString() === now.toDateString()) {
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  return date.toLocaleDateString()
}

// Load sessions
const loadSessions = async () => {
  try {
    const res = await getSessions()
    if (res.data && res.data.code === 200) {
      sessions.value = res.data.data?.sessions || []
    }
  } catch (e) {
    console.error('Failed to load sessions:', e)
  }
}

// Load agents
const loadAgents = async () => {
  try {
    const res = await getAgents()
    if (res.data && res.data.code === 200) {
      agents.value = res.data.data?.agents || []
      if (agents.value.length > 0 && !selectedAgentId.value) {
        selectedAgentId.value = agents.value[0].agent_id
      }
    }
  } catch (e) {
    console.error('Failed to load agents:', e)
  }
}

// Select session
const handleSelectSession = async (session) => {
  currentSessionId.value = session.session_id
  currentMessages.value = []
  
  // Load session message history
  if (session.session_id) {
    try {
      const res = await getSessionMessages(session.session_id, { limit: 100, offset: 0 })
      if (res.data && res.data.code === 200 && res.data.data) {
        currentMessages.value = res.data.data.messages || []
      }
    } catch (e) {
      console.error('Failed to load session messages:', e)
      ElMessage.warning('加载历史消息失败')
    }
  }
  
  scrollToBottom()
}

// New chat
const handleNewChat = () => {
  currentSessionId.value = null
  currentMessages.value = []
  inputText.value = ''
}

// Send message with SSE streaming
const handleSend = async () => {
  if (!inputText.value.trim() || isStreaming.value) return
  
  const question = inputText.value.trim()
  inputText.value = ''
  
  // Add user message to UI
  currentMessages.value.push({
    role: 'user',
    content: question,
    timestamp: new Date().toISOString()
  })
  
  scrollToBottom()
  isStreaming.value = true
  
  try {
    const response = await chatStream({
      question,
      session_id: currentSessionId.value || undefined,
      agent_id: selectedAgentId.value || undefined
    })
    
    // Parse SSE stream
    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let assistantMessage = {
      role: 'assistant',
      content: '',
      citations: [],
      timestamp: new Date().toISOString()
    }
    
    // Add assistant message placeholder
    const msgIndex = currentMessages.value.length
    currentMessages.value.push(assistantMessage)
    
    let currentEvent = ''
    let shouldStop = false
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''
      
      for (const line of lines) {
        const trimmed = line.trim()
        if (!trimmed) continue
        if (trimmed.startsWith('event:')) {
          currentEvent = trimmed.substring(6).trim()
          continue
        }
        if (!trimmed.startsWith('data:')) continue
        
        const dataStr = trimmed.substring(5).trim()
        if (!dataStr) continue
        
        try {
          const data = JSON.parse(dataStr)
          
          if (currentEvent === 'delta') {
            const token = data.token || ''
            assistantMessage.content += token
            currentMessages.value[msgIndex] = { ...assistantMessage }
            await nextTick()
            scrollToBottom()
          } else if (currentEvent === 'done') {
            if (data.session_id) {
              currentSessionId.value = data.session_id
            }
            if (data.citations) {
              assistantMessage.citations = data.citations
            }
            currentMessages.value[msgIndex] = { ...assistantMessage }
            await loadSessions()
          } else if (currentEvent === 'error') {
            ElMessage.error(data.error || '请求失败')
            currentMessages.value.splice(msgIndex, 1)
            shouldStop = true
            break
          }
        } catch (parseErr) {
          console.error('Failed to parse SSE event:', parseErr, dataStr)
        }
      }
      if (shouldStop) break
    }
  } catch (err) {
    console.error('Stream error:', err)
    ElMessage.error('发送失败，请重试')
    // Remove assistant placeholder if error
    if (currentMessages.value[currentMessages.value.length - 1]?.role === 'assistant' && !currentMessages.value[currentMessages.value.length - 1]?.content) {
      currentMessages.value.pop()
    }
  } finally {
    isStreaming.value = false
  }
}

// Scroll to bottom
const scrollToBottom = async () => {
  await nextTick()
  if (msgListRef.value) {
    msgListRef.value.scrollTop = msgListRef.value.scrollHeight
  }
}

// Lifecycle
onMounted(() => {
  loadSessions()
  loadAgents()
})

// Watch session change to scroll
watch(currentSessionId, () => {
  scrollToBottom()
})
</script>

<style scoped>
.assistant-container {
  height: 100vh;
  width: 100vw;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #f5f7fa;
  background-image: 
    radial-gradient(at 10% 10%, rgba(138, 43, 226, 0.08) 0px, transparent 50%),
    radial-gradient(at 90% 90%, rgba(65, 105, 225, 0.05) 0px, transparent 50%),
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
    radial-gradient(circle at 30% 40%, rgba(138, 43, 226, 0.1) 0%, transparent 40%),
    radial-gradient(circle at 70% 20%, rgba(65, 105, 225, 0.08) 0%, transparent 35%);
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

/* Session List */
.session-list {
  width: 280px;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.3);
  border-right: 1px solid rgba(255, 255, 255, 0.3);
}

.session-list .header {
  padding: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.session-list .title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  background: linear-gradient(135deg, #8a2be2 0%, #4169e1 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.new-chat-btn {
  background: linear-gradient(135deg, #8a2be2 0%, #4169e1 100%);
  border: none;
  color: white;
}

.new-chat-btn:hover {
  opacity: 0.9;
}

.list-content {
  flex: 1;
  overflow-y: auto;
}

.session-item {
  padding: 15px 20px;
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.session-item:hover {
  background: rgba(255, 255, 255, 0.3);
}

.session-item.active {
  background: linear-gradient(90deg, rgba(138, 43, 226, 0.15) 0%, rgba(65, 105, 225, 0.15) 100%);
  border-right: 3px solid #8a2be2;
}

.session-icon {
  width: 44px;
  height: 44px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #8a2be2 0%, #4169e1 100%);
  color: white;
  font-size: 20px;
}

.item-info {
  flex: 1;
  min-width: 0;
}

.item-top {
  display: flex;
  justify-content: space-between;
  margin-bottom: 4px;
}

.name {
  font-weight: 500;
  color: #303133;
  font-size: 14px;
}

.time {
  font-size: 12px;
  color: #909399;
}

.item-msg {
  font-size: 12px;
  color: #909399;
}

.text-ellipsis {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Chat Window */
.chat-window {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.2);
}

.chat-header {
  padding: 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.3);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.chat-name {
  font-size: 16px;
  font-weight: 600;
  background: linear-gradient(135deg, #8a2be2 0%, #4169e1 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.agent-selector {
  width: 200px;
}

.agent-selector :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.6);
  box-shadow: none;
  border-radius: 20px;
}

/* Message Area */
.message-area {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}

.message-row {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
}

.message-row.is-mine {
  flex-direction: row-reverse;
}

.msg-avatar {
  flex-shrink: 0;
}

.ai-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #8a2be2 0%, #4169e1 100%);
  color: white;
  font-size: 18px;
}

.msg-content-wrapper {
  max-width: 70%;
}

.msg-bubble {
  padding: 12px 16px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
  word-wrap: break-word;
}

.is-mine .msg-bubble {
  background: linear-gradient(135deg, #8a2be2 0%, #4169e1 100%);
  color: white;
}

.ai-bubble {
  background: rgba(255, 255, 255, 0.95);
  border: 1px solid rgba(138, 43, 226, 0.2);
}

.msg-text {
  white-space: pre-wrap;
  line-height: 1.6;
}

.streaming {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #8a2be2;
}

/* Citations */
.citations-section {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid rgba(0, 0, 0, 0.05);
}

.citations-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #606266;
}

.citations-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.citation-card {
  padding: 10px;
  background: rgba(138, 43, 226, 0.03);
  border-radius: 8px;
  border: 1px solid rgba(138, 43, 226, 0.1);
}

.citation-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.citation-score {
  font-size: 12px;
  color: #8a2be2;
  font-weight: 500;
}

.citation-content {
  font-size: 13px;
  color: #606266;
  line-height: 1.5;
  margin-bottom: 6px;
}

.citation-meta {
  display: flex;
  gap: 12px;
  font-size: 11px;
  color: #909399;
}

/* Input Area */
.input-area {
  padding: 20px;
  border-top: 1px solid rgba(255, 255, 255, 0.3);
  background: rgba(255, 255, 255, 0.3);
}

.input-area :deep(.el-textarea__inner) {
  background: rgba(255, 255, 255, 0.8);
  border: 1px solid rgba(138, 43, 226, 0.2);
  border-radius: 12px;
  padding: 12px;
}

.input-area :deep(.el-textarea__inner):focus {
  border-color: #8a2be2;
}

.send-actions {
  margin-top: 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.tip {
  font-size: 12px;
  color: #909399;
}

.send-actions .el-button {
  background: linear-gradient(135deg, #8a2be2 0%, #4169e1 100%);
  border: none;
}

/* Scrollbar */
.custom-scrollbar::-webkit-scrollbar {
  width: 6px;
}

.custom-scrollbar::-webkit-scrollbar-track {
  background: rgba(0, 0, 0, 0.05);
  border-radius: 3px;
}

.custom-scrollbar::-webkit-scrollbar-thumb {
  background: rgba(138, 43, 226, 0.3);
  border-radius: 3px;
}

.custom-scrollbar::-webkit-scrollbar-thumb:hover {
  background: rgba(138, 43, 226, 0.5);
}
</style>
