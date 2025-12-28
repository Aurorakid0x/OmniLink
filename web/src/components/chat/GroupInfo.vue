<template>
  <div class="group-info glass-panel">
    <div class="section-title">群公告</div>
    <div class="announcement">
      {{ announcement }}
    </div>
    
    <div class="section-title">群成员 ({{ members.length }})</div>
    <el-scrollbar class="members-list">
      <div class="member-grid">
        <div v-for="member in members" :key="member.id" class="member-item">
          <el-avatar :size="40" :src="member.avatar" class="member-avatar" />
          <span class="member-name">{{ member.name }}</span>
        </div>
        <div class="member-item add-btn">
          <div class="add-icon">+</div>
          <span class="member-name">邀请</span>
        </div>
      </div>
    </el-scrollbar>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const props = defineProps({
  session: {
    type: Object,
    default: null
  }
})

// Mock Data
const announcement = ref("欢迎来到 OmniLink 开发交流群！请保持文明交流。")
const members = ref([
  { id: 1, name: 'Alice', avatar: 'https://cube.elemecdn.com/3/7c/3ea6beec64369c2642b92c6726f1epng.png' },
  { id: 2, name: 'Bob', avatar: 'https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png' },
  { id: 3, name: 'Charlie', avatar: 'https://cube.elemecdn.com/3/7c/3ea6beec64369c2642b92c6726f1epng.png' },
  { id: 4, name: 'David', avatar: 'https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png' },
  { id: 5, name: 'Eve', avatar: 'https://cube.elemecdn.com/3/7c/3ea6beec64369c2642b92c6726f1epng.png' },
])
</script>

<style scoped>
.group-info {
  width: 240px;
  border-left: 1px solid rgba(0,0,0,0.05);
  background: rgba(255, 255, 255, 0.4);
  padding: 20px;
  display: flex;
  flex-direction: column;
}

.section-title {
  font-weight: 600;
  color: #2c3e50;
  margin-bottom: 10px;
  margin-top: 10px;
  font-size: 14px;
}

.section-title:first-child {
  margin-top: 0;
}

.announcement {
  font-size: 13px;
  color: #666;
  line-height: 1.5;
  background: rgba(255, 255, 255, 0.5);
  padding: 10px;
  border-radius: 8px;
  margin-bottom: 20px;
}

.members-list {
  flex: 1;
}

.member-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 15px;
}

.member-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 50px;
}

.member-name {
  font-size: 12px;
  color: #666;
  margin-top: 5px;
  text-align: center;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
}

.add-btn .add-icon {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  border: 1px dashed #999;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #999;
  cursor: pointer;
  transition: all 0.3s;
}

.add-btn .add-icon:hover {
  border-color: #2c3e50;
  color: #2c3e50;
}
</style>
