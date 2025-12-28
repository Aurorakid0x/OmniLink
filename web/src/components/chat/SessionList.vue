<template>
  <div class="session-list glass-panel">
    <div class="search-bar">
      <el-input
        v-model="searchQuery"
        placeholder="搜索"
        class="custom-search-input"
        :prefix-icon="Search"
      />
    </div>

    <el-scrollbar class="list-container">
      <div 
        v-for="session in filteredSessions" 
        :key="session.id"
        class="session-item"
        :class="{ active: currentSessionId === session.id }"
        @click="$emit('select-session', session)"
      >
        <div class="avatar-wrapper">
          <el-avatar :size="44" :src="session.avatar" shape="square" class="session-avatar" />
          <div v-if="session.unread > 0" class="unread-badge">{{ session.unread }}</div>
        </div>
        
        <div class="session-info">
          <div class="top-row">
            <span class="nickname">{{ session.name }}</span>
            <span class="time">{{ session.time }}</span>
          </div>
          <div class="bottom-row">
            <span class="last-msg">{{ session.lastMsg }}</span>
          </div>
        </div>
      </div>
    </el-scrollbar>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { Search } from '@element-plus/icons-vue'

const props = defineProps({
  sessions: {
    type: Array,
    default: () => []
  },
  currentSessionId: {
    type: [String, Number],
    default: null
  }
})

const emit = defineEmits(['select-session'])
const searchQuery = ref('')

const filteredSessions = computed(() => {
  if (!searchQuery.value) return props.sessions
  return props.sessions.filter(s => s.name.includes(searchQuery.value))
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

.search-bar {
  padding: 20px;
}

/* Customize Element Input for Glassmorphism */
:deep(.custom-search-input .el-input__wrapper) {
  background-color: rgba(255, 255, 255, 0.5);
  box-shadow: none;
  border-radius: 20px;
  padding: 4px 15px;
  transition: all 0.3s;
}

:deep(.custom-search-input .el-input__wrapper.is-focus) {
  background-color: rgba(255, 255, 255, 0.8);
  box-shadow: 0 0 0 1px #2c3e50 inset;
}

.list-container {
  flex: 1;
  padding: 0 10px;
}

.session-item {
  display: flex;
  padding: 15px 10px;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s ease;
  margin-bottom: 5px;
}

.session-item:hover {
  background: rgba(255, 255, 255, 0.4);
}

.session-item.active {
  background: rgba(44, 62, 80, 0.08); /* Light dark tint */
  backdrop-filter: blur(5px);
}

.avatar-wrapper {
  position: relative;
  margin-right: 12px;
}

.session-avatar {
  border-radius: 10px;
}

.unread-badge {
  position: absolute;
  top: -5px;
  right: -5px;
  background-color: #f56c6c;
  color: white;
  font-size: 10px;
  height: 16px;
  min-width: 16px;
  padding: 0 4px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2px solid #fff;
}

.session-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  overflow: hidden;
}

.top-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.nickname {
  font-weight: 600;
  color: #2c3e50;
  font-size: 14px;
}

.time {
  font-size: 12px;
  color: #999;
}

.bottom-row {
  display: flex;
}

.last-msg {
  font-size: 12px;
  color: #666;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
