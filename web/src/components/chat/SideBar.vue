<template>
  <div class="sidebar glass-panel">
    <div class="avatar-section">
      <el-avatar :size="40" :src="userAvatar" class="user-avatar" />
    </div>
    
    <div class="nav-icons">
      <div 
        class="nav-item" 
        :class="{ active: activeTab === 'chat' }"
        @click="$emit('update:activeTab', 'chat')"
      >
        <el-icon :size="24"><ChatDotRound /></el-icon>
      </div>
      <div 
        class="nav-item" 
        :class="{ active: activeTab === 'contacts' }"
        @click="$emit('update:activeTab', 'contacts')"
      >
        <el-icon :size="24"><User /></el-icon>
      </div>
      <div 
        class="nav-item" 
        :class="{ active: activeTab === 'settings' }"
        @click="$emit('update:activeTab', 'settings')"
      >
        <el-icon :size="24"><Setting /></el-icon>
      </div>
    </div>

    <div class="bottom-actions">
      <!-- Future bottom actions like logout -->
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useStore } from 'vuex'
import { ChatDotRound, User, Setting } from '@element-plus/icons-vue'

const props = defineProps({
  activeTab: {
    type: String,
    default: 'chat'
  }
})

const emit = defineEmits(['update:activeTab'])
const store = useStore()

const userAvatar = computed(() => {
  return store.state.userInfo?.avatar || 'https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png'
})
</script>

<style scoped>
.sidebar {
  width: 70px; /* Slightly wider than 60px for better touch targets */
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px 0;
  border-right: 1px solid rgba(255, 255, 255, 0.3);
  background: rgba(255, 255, 255, 0.4);
}

.avatar-section {
  margin-bottom: 40px;
}

.user-avatar {
  border: 2px solid rgba(255, 255, 255, 0.8);
  box-shadow: 0 4px 10px rgba(0,0,0,0.1);
  transition: transform 0.3s;
}

.user-avatar:hover {
  transform: rotate(360deg);
}

.nav-icons {
  display: flex;
  flex-direction: column;
  gap: 30px;
  width: 100%;
  align-items: center;
}

.nav-item {
  width: 46px;
  height: 46px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #606266;
  cursor: pointer;
  transition: all 0.3s ease;
}

.nav-item:hover {
  background: rgba(255, 255, 255, 0.5);
  color: #2c3e50;
}

.nav-item.active {
  background: #2c3e50;
  color: #fff;
  box-shadow: 0 4px 12px rgba(44, 62, 80, 0.3);
}

.bottom-actions {
  margin-top: auto;
}
</style>
