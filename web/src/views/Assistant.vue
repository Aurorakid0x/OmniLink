<template>
  <div class="assistant-container">
    <div class="ink-bg-layer"></div>

    <div class="main-layout glass-card">
      <!-- 1. Agent List Panel -->
      <div class="agent-list glass-panel">
        <div class="header">
          <h3 class="title">Agents</h3>
          <el-button circle icon="Plus" class="new-btn" @click="openCreateAgent" />
        </div>
        <div class="list-content custom-scrollbar">
          <el-empty v-if="agents.length === 0" description="暂无 Agent" :image-size="40" />
          <div 
            v-for="agent in agents" 
            :key="agent.agent_id" 
            class="list-item"
            :class="{ active: selectedAgentId === agent.agent_id }"
            @click="handleSelectAgent(agent)"
          >
            <div class="item-icon agent-icon">
              <el-icon><UserFilled /></el-icon>
            </div>
            <div class="item-info">
              <div class="name">{{ agent.name }}</div>
              <div class="desc text-ellipsis">{{ agent.description || '暂无描述' }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- 2. Session List Panel -->
      <div class="session-list glass-panel">
        <div class="header">
          <h3 class="title">会话列表</h3>
          <el-button circle icon="Plus" class="new-btn" @click="handleNewSession" :disabled="!selectedAgentId" />
        </div>
        <div class="list-content custom-scrollbar">
          <div v-if="!selectedAgentId" class="empty-tip">请先选择左侧 Agent</div>
          <el-empty v-else-if="sessions.length === 0" description="暂无会话" :image-size="40" />
          
          <div 
            v-else
            v-for="item in sessions" 
            :key="item.session_id" 
            class="list-item"
            :class="{ active: currentSessionId === item.session_id }"
            @click="handleSelectSession(item)"
          >
            <div class="item-icon session-icon">
              <el-icon><ChatDotRound /></el-icon>
            </div>
            <div class="item-info">
              <div class="item-top">
                <span class="name">{{ item.title || '新对话' }}</span>
                <span class="time">{{ formatTime(item.updated_at) }}</span>
              </div>
              <div class="desc text-ellipsis">
                {{ item.summary || item.last_message || '点击开始对话' }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 3. Chat Window -->
      <div class="chat-window glass-panel">
        <template v-if="currentSessionId">
          <!-- Header -->
          <div class="chat-header">
            <div class="header-info">
              <span class="chat-name">{{ currentSessionTitle }}</span>
              <el-tag size="small" type="info" class="agent-tag" v-if="currentAgentName">{{ currentAgentName }}</el-tag>
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
        </template>
        <div v-else class="empty-state">
          <el-empty description="请选择会话开始聊天" />
        </div>
      </div>
    </div>

    <!-- Create Agent Dialog -->
    <el-dialog v-model="createAgentDialogVisible" title="创建 AI Agent" width="500px" append-to-body>
      <el-form :model="createAgentForm" label-width="100px">
        <el-form-item label="名称" required>
          <el-input v-model="createAgentForm.name" placeholder="给 Agent 起个名字" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="createAgentForm.description" placeholder="简单描述这个 Agent 的功能" />
        </el-form-item>
        <el-form-item label="人设 Prompt">
          <el-input 
            v-model="createAgentForm.persona_prompt" 
            type="textarea" 
            :rows="4" 
            placeholder="设定 Agent 的性格、语气、角色等" 
          />
        </el-form-item>
        <el-form-item label="知识库类型" required>
          <el-select v-model="createAgentForm.kb_type" style="width: 100%">
            <el-option label="全局知识库 (Global)" value="global" />
            <el-option label="私有知识库 (Agent Private)" value="agent_private" />
          </el-select>
        </el-form-item>
        <el-form-item label="知识库名称" v-if="createAgentForm.kb_type === 'agent_private'" required>
           <el-input v-model="createAgentForm.kb_name" placeholder="为私有知识库命名" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createAgentDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleCreateAgent" :loading="creatingAgent">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useStore } from 'vuex'
import { MagicStick, Plus, Document, Loading, UserFilled, ChatDotRound } from '@element-plus/icons-vue'
import { getSessions, getAgents, getSessionMessages, chatStream, createAgent, createSession } from '../api/ai'
import { ElMessage, ElMessageBox } from 'element-plus'

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

// Create Agent Dialog
const createAgentDialogVisible = ref(false)
const creatingAgent = ref(false)
const createAgentForm = ref({
  name: '',
  description: '',
  persona_prompt: '',
  kb_type: 'global',
  kb_name: ''
})

// User info
const userInfo = computed(() => store.state.userInfo)
const userName = computed(() => userInfo.value?.nickname || '')
const userAvatar = computed(() => userInfo.value?.avatar || '')

const currentAgentName = computed(() => {
  const ag = agents.value.find(a => a.agent_id === selectedAgentId.value)
  return ag ? ag.name : ''
})

const currentSessionTitle = computed(() => {
  const sess = sessions.value.find(s => s.session_id === currentSessionId.value)
  return sess ? (sess.title || '新对话') : ''
})

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

// Load agents
const loadAgents = async () => {
  try {
    const res = await getAgents()
    if (res.data && res.data.code === 200) {
      agents.value = res.data.data?.agents || []
    }
  } catch (e) {
    console.error('Failed to load agents:', e)
  }
}

// Load sessions (filtered by selected agent)
const loadSessions = async () => {
  try {
    const res = await getSessions()
    if (res.data && res.data.code === 200) {
      const allSessions = res.data.data?.sessions || []
      // Filter by selectedAgentId
      if (selectedAgentId.value) {
        sessions.value = allSessions.filter(s => s.agent_id === selectedAgentId.value)
      } else {
        sessions.value = []
      }
    }
  } catch (e) {
    console.error('Failed to load sessions:', e)
  }
}

// Select Agent
const handleSelectAgent = async (agent) => {
  selectedAgentId.value = agent.agent_id
  currentSessionId.value = null
  currentMessages.value = []
  await loadSessions()
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

// Open Create Agent Dialog
const openCreateAgent = () => {
  createAgentForm.value = {
    name: '',
    description: '',
    persona_prompt: '',
    kb_type: 'global',
    kb_name: ''
  }
  createAgentDialogVisible.value = true
}

// Handle Create Agent
const handleCreateAgent = async () => {
  if (!createAgentForm.value.name) {
    ElMessage.warning('请输入 Agent 名称')
    return
  }
  if (createAgentForm.value.kb_type === 'agent_private' && !createAgentForm.value.kb_name) {
    ElMessage.warning('请输入知识库名称')
    return
  }

  creatingAgent.value = true
  try {
    const res = await createAgent(createAgentForm.value)
    if (res.data && res.data.code === 200) {
      ElMessage.success('Agent 创建成功')
      createAgentDialogVisible.value = false
      await loadAgents()
    } else {
      ElMessage.error(res.data?.message || '创建失败')
    }
  } catch (e) {
    console.error(e)
    ElMessage.error('创建失败')
  } finally {
    creatingAgent.value = false
  }
}

// Handle New Session
const handleNewSession = async () => {
  if (!selectedAgentId.value) {
    ElMessage.warning('请先选择一个 Agent')
    return
  }

  // Use MessageBox prompt for title (optional) or just create default
  try {
    const res = await createSession({
      agent_id: selectedAgentId.value,
      title: '新对话'
    })
    if (res.data && res.data.code === 200) {
      ElMessage.success('会话创建成功')
      await loadSessions()
      // Auto select new session
      const newSessionId = res.data.data.session_id
      const newSession = sessions.value.find(s => s.session_id === newSessionId)
      if (newSession) {
        handleSelectSession(newSession)
      }
    } else {
      ElMessage.error(res.data?.message || '创建会话失败')
    }
  } catch (e) {
    console.error(e)
    ElMessage.error('创建会话失败')
  }
}

// Send message with SSE streaming
const handleSend = async () => {
  if (!inputText.value.trim() || isStreaming.value) return
  if (!currentSessionId.value) {
    ElMessage.warning('请先选择或创建会话')
    return
  }
  
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
      session_id: currentSessionId.value,
      agent_id: selectedAgentId.value
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
            if (data.citations) {
              assistantMessage.citations = data.citations
            }
            currentMessages.value[msgIndex] = { ...assistantMessage }
            await loadSessions() // Refresh session list (summary/time)
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
onMounted(async () => {
  await loadAgents()
  // If agents exist, select first one
  if (agents.value.length > 0) {
    handleSelectAgent(agents.value[0])
  }
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
  width: 95vw;
  height: 90vh;
  max-width: 1600px;
  background: rgba(255, 255, 255, 0.4);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.6);
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.1);
  border-radius: 20px;
  display: flex;
  overflow: hidden;
}

/* Common Header */
.header {
  padding: 15px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
}

.title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.new-btn {
  background: linear-gradient(135deg, #8a2be2 0%, #4169e1 100%);
  border: none;
  color: white;
  width: 32px;
  height: 32px;
}

.new-btn:hover {
  opacity: 0.9;
}

.new-btn:disabled {
  background: #ccc;
  cursor: not-allowed;
}

/* Agent List */
.agent-list {
  width: 260px;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.25);
  border-right: 1px solid rgba(255, 255, 255, 0.3);
}

/* Session List */
.session-list {
  width: 280px;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.15);
  border-right: 1px solid rgba(255, 255, 255, 0.3);
}

.empty-tip {
  padding: 40px 20px;
  text-align: center;
  color: #909399;
  font-size: 14px;
}

/* List Content */
.list-content {
  flex: 1;
  overflow-y: auto;
}

.list-item {
  padding: 12px 15px;
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
  transition: all 0.2s;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.list-item:hover {
  background: rgba(255, 255, 255, 0.3);
}

.list-item.active {
  background: rgba(138, 43, 226, 0.1);
  border-right: 3px solid #8a2be2;
}

.item-icon {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 20px;
  flex-shrink: 0;
}

.agent-icon {
  background: linear-gradient(135deg, #FF9A9E 0%, #FECFEF 100%);
}

.session-icon {
  background: linear-gradient(135deg, #a18cd1 0%, #fbc2eb 100%);
}

.item-info {
  flex: 1;
  min-width: 0;
}

.name {
  font-weight: 500;
  color: #303133;
  font-size: 14px;
  margin-bottom: 4px;
}

.desc {
  font-size: 12px;
  color: #909399;
}

.item-top {
  display: flex;
  justify-content: space-between;
  margin-bottom: 4px;
}

.time {
  font-size: 11px;
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
  background: rgba(255, 255, 255, 0.1);
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}

.chat-header {
  padding: 15px 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.3);
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: rgba(255, 255, 255, 0.2);
}

.chat-name {
  font-size: 16px;
  font-weight: 600;
  margin-right: 10px;
}

/* Reuse existing chat styles... */
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
