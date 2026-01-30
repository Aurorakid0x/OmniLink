<template>
  <div class="session-list glass-panel">
    <div class="header">
      <el-input 
        v-model="searchKey" 
        placeholder="搜索会话" 
        prefix-icon="Search"
        clearable 
        class="search-input"
      />
      <div class="header-actions">
        <el-button circle icon="Plus" class="add-btn" @click="$emit('show-create-group')" />
        <el-button circle icon="Setting" class="add-btn" @click="openAgentManage" title="Agent管理" />
      </div>
    </div>

    <div class="list-content custom-scrollbar">
      <!-- 系统AI助手会话（置顶，不可删除） -->
      <div 
        v-if="systemAISession" 
        class="session-item system-ai-session"
        :class="{ active: currentSessionId === systemAISession.session_id }"
        @click="handleSelectAISession(systemAISession)"
      >
        <div class="avatar-wrapper">
          <div class="item-icon ai-icon">
            <el-icon><MagicStick /></el-icon>
          </div>
        </div>
        <div class="item-info">
          <div class="item-top">
            <span class="name">{{ systemAISession.title }}</span>
            <el-tag size="small" type="primary" effect="plain">AI</el-tag>
          </div>
          <div class="item-msg text-ellipsis">您的专属智能助理</div>
        </div>
      </div>

      <!-- 用户自定义AI会话 -->
      <div 
        v-for="aiSession in aiSessions" 
        :key="'ai-' + aiSession.session_id"
        class="session-item ai-session"
        :class="{ active: currentSessionId === aiSession.session_id }"
        @click="handleSelectAISession(aiSession)"
      >
        <div class="avatar-wrapper">
          <div class="item-icon ai-icon">
            <el-icon><UserFilled /></el-icon>
          </div>
        </div>
        <div class="item-info">
          <div class="item-top">
            <span class="name">{{ aiSession.title }}</span>
            <el-tag size="small" type="info" effect="plain">AI</el-tag>
          </div>
          <div class="item-msg text-ellipsis">
            <span v-if="aiSession.agent_name" class="agent-tag">{{ aiSession.agent_name }}</span>
            {{ aiSession.summary || '点击开始对话' }}
          </div>
        </div>
      </div>

      <!-- IM会话列表 -->
      <el-empty v-if="displayList.length === 0" description="暂无会话" :image-size="60" />
      
      <div 
        v-for="item in displayList" 
        :key="'im-' + item.session_id" 
        class="session-item"
        :class="{ active: currentSessionId === item.session_id }"
        @click="handleSelect(item)"
      >
        <div class="avatar-wrapper">
          <el-avatar :src="normalizeUrl(item.peer_avatar)" shape="square" :size="44">
            {{ (item.peer_name || '?')[0] }}
          </el-avatar>
          <span class="unread-dot" v-if="getUnread(item) > 0">{{ getUnread(item) }}</span>
        </div>
        
        <div class="item-info">
          <div class="item-top">
            <span class="name">{{ item.peer_name }}</span>
            <span class="time">{{ formatTime(item.updated_at) }}</span>
          </div>
          <div class="item-msg text-ellipsis">
            {{ item.last_msg || '点击开始聊天' }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useStore } from 'vuex'
import { Search, Plus, Setting, MagicStick, UserFilled } from '@element-plus/icons-vue'
import { normalizeUrl } from '../../api/im'

const emit = defineEmits(['select-session', 'show-create-group'])

const store = useStore()
const searchKey = ref('')
const loading = ref(false)

const currentSessionId = computed(() => store.state.currentSessionId)
const sessionList = computed(() => store.state.sessionList)
const unreadMap = computed(() => store.state.unreadMap)

// 系统AI助手会话
const systemAISession = computed(() => store.state.systemAISession)

// 用户自定义AI会话（过滤掉系统会话）
const aiSessions = computed(() => 
  store.state.aiSessions.filter(s => s.session_type !== 'system_global')
)

const displayList = computed(() => {
  if (!searchKey.value) return sessionList.value
  return sessionList.value.filter(item => {
    const name = item.peer_name || ''
    return name.toLowerCase().includes(searchKey.value.toLowerCase())
  })
})

const getUnread = (session) => {
    const peerId = session.peer_id
    return unreadMap.value[peerId] || 0
}

const formatTime = (timeStr) => {
    if (!timeStr) return ''
    const date = new Date(timeStr)
    const now = new Date()
    if (date.toDateString() === now.toDateString()) {
        return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    }
    return date.toLocaleDateString()
}

// 选择AI会话
const handleSelectAISession = (session) => {
  emit('select-session', { ...session, type: 'ai' })
}

// 选择IM会话
const handleSelect = (session) => {
    emit('select-session', { ...session, type: 'im' })
}

// 打开Agent管理弹窗
const openAgentManage = () => {
  store.commit('setShowAgentManage', true)
}

const loadSessions = async () => {
    loading.value = true
    try {
        await store.dispatch('loadSessions')
    } finally {
        loading.value = false
    }
}

onMounted(async () => {
    loadSessions()
    await store.dispatch('loadSystemAISession')
    await store.dispatch('loadAISessions')
})
</script>

<style scoped>
.session-list {
  width: 280px;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.3);
  border-right: 1px solid rgba(255, 255, 255, 0.3);
}

.header {
  padding: 20px;
  display: flex;
  gap: 10px;
  align-items: center;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.search-input {
  flex: 1;
}

.search-input :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.6);
  box-shadow: none;
  border-radius: 20px;
}

.add-btn {
    background: transparent;
    border: 1px solid rgba(0,0,0,0.1);
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
  background: rgba(64, 158, 255, 0.15);
  border-right: 3px solid #409EFF;
}

/* 系统AI助手会话特殊样式 */
.system-ai-session {
  background: linear-gradient(135deg, rgba(138, 43, 226, 0.1) 0%, rgba(65, 105, 225, 0.05) 100%);
  border-left: 3px solid #8a2be2;
}

.system-ai-session.active {
  background: linear-gradient(135deg, rgba(138, 43, 226, 0.2) 0%, rgba(65, 105, 225, 0.1) 100%);
  border-right: 3px solid #8a2be2;
}

/* AI会话图标样式 */
.item-icon {
  width: 44px;
  height: 44px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  color: white;
}

.ai-session .ai-icon {
  background: linear-gradient(135deg, #FF9A9E 0%, #FECFEF 100%);
}

.system-ai-session .ai-icon {
  background: linear-gradient(135deg, #8a2be2 0%, #4169e1 100%);
}

.avatar-wrapper {
  position: relative;
}

.unread-dot {
  position: absolute;
  top: -5px;
  right: -5px;
  background: #f56c6c;
  color: white;
  font-size: 10px;
  padding: 0 5px;
  border-radius: 10px;
  min-width: 16px;
  text-align: center;
  border: 1px solid #fff;
}

.item-info {
  flex: 1;
  min-width: 0;
}

.item-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
  gap: 8px;
}

.name {
  font-weight: 500;
  color: #303133;
  font-size: 14px;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.time {
  font-size: 12px;
  color: #909399;
  flex-shrink: 0;
}

.item-msg {
  font-size: 12px;
  color: #909399;
}

.agent-tag {
  display: inline-block;
  background: rgba(138, 43, 226, 0.1);
  color: #8a2be2;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 11px;
  margin-right: 6px;
  font-weight: 500;
}

.text-ellipsis {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
