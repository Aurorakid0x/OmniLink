<template>
  <div class="sidebar glass-panel">
    <div class="logo-area">
      <div class="logo-icon">O</div>
    </div>
    
    <div class="nav-items">
      <div 
        class="nav-item" 
        :class="{ active: activeTab === 'chat' }"
        @click="$emit('update:activeTab', 'chat')"
      >
        <el-icon><ChatDotRound /></el-icon>
        <span class="badge" v-if="totalUnread > 0">{{ totalUnread > 99 ? '99+' : totalUnread }}</span>
      </div>
      
      <div 
        class="nav-item" 
        :class="{ active: activeTab === 'contacts' }"
        @click="$emit('update:activeTab', 'contacts')"
      >
        <el-icon><User /></el-icon>
        <span class="badge" v-if="pendingApplyCount > 0">{{ pendingApplyCount > 99 ? '99+' : pendingApplyCount }}</span>
      </div>

      <div 
        class="nav-item" 
        :class="{ active: activeTab === 'me' }"
        @click="$emit('update:activeTab', 'me')"
      >
        <el-icon><Setting /></el-icon>
      </div>
    </div>

    <div class="bottom-actions">
       <div class="nav-item logout" @click="handleLogout">
        <el-icon><SwitchButton /></el-icon>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ChatDotRound, User, Setting, SwitchButton } from '@element-plus/icons-vue'
import { useStore } from 'vuex'
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'

const props = defineProps({
  activeTab: String
})

defineEmits(['update:activeTab'])

const store = useStore()
const router = useRouter()
const totalUnread = computed(() => store.getters.totalUnread)
const pendingApplyCount = computed(() => store.getters.pendingApplyCount)

const handleLogout = () => {
  ElMessageBox.confirm('确定要退出登录吗?', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    store.commit('clearAuth')
    store.dispatch('disconnectWebSocket')
    router.push('/login')
  }).catch(() => {})
}
</script>

<style scoped>
.sidebar {
  width: 70px;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px 0;
  background: rgba(255, 255, 255, 0.5);
  border-right: 1px solid rgba(255, 255, 255, 0.3);
}

.logo-icon {
  width: 40px;
  height: 40px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 10px;
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: bold;
  font-size: 20px;
  margin-bottom: 40px;
  box-shadow: 0 4px 10px rgba(118, 75, 162, 0.3);
}

.nav-items {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.nav-item {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: #606266;
  transition: all 0.3s ease;
  position: relative;
}

.nav-item:hover {
  background: rgba(255, 255, 255, 0.5);
  color: #409EFF;
}

.nav-item.active {
  background: #409EFF;
  color: white;
  box-shadow: 0 4px 12px rgba(64, 158, 255, 0.3);
}

.nav-item .el-icon {
  font-size: 22px;
}

.badge {
  position: absolute;
  top: -5px;
  right: -5px;
  background-color: #f56c6c;
  color: white;
  font-size: 10px;
  padding: 2px 5px;
  border-radius: 10px;
  line-height: 1;
  border: 2px solid #fff;
}

.logout:hover {
    color: #f56c6c;
    background: rgba(245, 108, 108, 0.1);
}
</style>
