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
      <el-button circle icon="Plus" class="add-btn" @click="$emit('show-create-group')" />
    </div>

    <div class="list-content custom-scrollbar">
      <el-empty v-if="displayList.length === 0" description="暂无会话" :image-size="60" />
      
      <div 
        v-for="item in displayList" 
        :key="item.session_id" 
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
import { Search, Plus } from '@element-plus/icons-vue'
import { normalizeUrl } from '../../api/im'

const emit = defineEmits(['select-session', 'show-create-group'])

const store = useStore()
const searchKey = ref('')
const loading = ref(false)

const currentSessionId = computed(() => store.state.currentSessionId)
const sessionList = computed(() => store.state.sessionList)
const unreadMap = computed(() => store.state.unreadMap)

const displayList = computed(() => {
  if (!searchKey.value) return sessionList.value
  return sessionList.value.filter(item => {
    const name = item.peer_name || ''
    return name.toLowerCase().includes(searchKey.value.toLowerCase())
  })
})

const getUnread = (session) => {
    // A6: 基于 peer_id
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

const handleSelect = (session) => {
    emit('select-session', session)
}

const loadSessions = async () => {
    loading.value = true
    try {
        await store.dispatch('loadSessions')
    } finally {
        loading.value = false
    }
}

onMounted(() => {
    loadSessions()
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
</style>
