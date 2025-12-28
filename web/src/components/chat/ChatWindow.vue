<template>
  <div class="chat-window glass-panel" v-if="session">
    <!-- Header -->
    <div class="chat-header">
      <div class="header-info">
        <span class="chat-title">{{ session.name }}</span>
        <span class="chat-status" v-if="session.online">Online</span>
      </div>
      <div class="header-actions">
        <el-button link>
          <el-icon :size="20"><Phone /></el-icon>
        </el-button>
        <el-button link>
          <el-icon :size="20"><VideoCamera /></el-icon>
        </el-button>
        <el-button link @click="$emit('toggle-right-sidebar')">
          <el-icon :size="20"><More /></el-icon>
        </el-button>
      </div>
    </div>

    <!-- Message Area -->
    <el-scrollbar class="message-area" ref="scrollbarRef">
      <div class="message-list">
        <div 
          v-for="(msg, index) in messages" 
          :key="msg.id" 
          class="message-row"
          :class="{ 'message-mine': msg.isMine }"
        >
          <!-- Time Divider -->
          <div v-if="showTime(msg, index)" class="time-divider">
            <span>{{ formatTime(msg.timestamp) }}</span>
          </div>

          <div class="message-content-wrapper">
            <el-avatar 
              v-if="!msg.isMine" 
              :size="36" 
              :src="session.avatar" 
              class="msg-avatar"
            />
            
            <div class="bubble-container">
              <div class="message-bubble">
                <span v-if="msg.type === 'text'">{{ msg.content }}</span>
                <el-image 
                  v-else-if="msg.type === 'image'" 
                  :src="msg.content" 
                  :preview-src-list="[msg.content]"
                  class="msg-image"
                />
              </div>
            </div>

            <el-avatar 
              v-if="msg.isMine" 
              :size="36" 
              :src="currentUserAvatar" 
              class="msg-avatar"
            />
          </div>
        </div>
      </div>
    </el-scrollbar>

    <!-- Input Area -->
    <div class="input-area">
      <div class="toolbar">
        <el-icon class="tool-icon"><Emoji /></el-icon>
        <el-icon class="tool-icon"><Picture /></el-icon>
        <el-icon class="tool-icon"><Folder /></el-icon>
      </div>
      <div class="textarea-wrapper">
        <textarea 
          v-model="inputText" 
          placeholder="输入消息..." 
          @keydown.enter.prevent="handleSend"
        ></textarea>
      </div>
      <div class="send-action">
        <el-button type="primary" class="send-btn" @click="handleSend">发送</el-button>
      </div>
    </div>
  </div>
  <div class="empty-state" v-else>
    <div class="empty-content">
      <img src="https://cdni.iconscout.com/illustration/premium/thumb/chat-bubble-3392336-2826721.png" alt="No Chat" class="empty-img"/>
      <p>选择一个会话开始聊天</p>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, computed } from 'vue'
import { useStore } from 'vuex'
import { Phone, VideoCamera, More, Picture, Folder } from '@element-plus/icons-vue'
// Mock Emoji icon as it might not be in standard set or needs specific import, using star for now or finding closest
import { Star as Emoji } from '@element-plus/icons-vue' 

const props = defineProps({
  session: {
    type: Object,
    default: null
  },
  messages: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['send-message', 'toggle-right-sidebar'])
const store = useStore()

const inputText = ref('')
const scrollbarRef = ref(null)

const currentUserAvatar = computed(() => {
  return store.state.userInfo?.avatar || 'https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png'
})

const handleSend = () => {
  if (!inputText.value.trim()) return
  emit('send-message', inputText.value)
  inputText.value = ''
}

const scrollToBottom = () => {
  nextTick(() => {
    if (scrollbarRef.value) {
      const wrap = scrollbarRef.value.wrapRef
      wrap.scrollTop = wrap.scrollHeight
    }
  })
}

watch(() => props.messages, () => {
  scrollToBottom()
}, { deep: true })

const showTime = (current, index) => {
  if (index === 0) return true
  const prev = props.messages[index - 1]
  return current.timestamp - prev.timestamp > 5 * 60 * 1000 // 5 mins
}

const formatTime = (ts) => {
  const date = new Date(ts)
  return `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`
}
</script>

<style scoped>
.chat-window {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.5);
  position: relative;
}

.chat-header {
  height: 60px;
  padding: 0 20px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  display: flex;
  justify-content: space-between;
  align-items: center;
  backdrop-filter: blur(10px);
}

.chat-title {
  font-size: 18px;
  font-weight: 600;
  color: #2c3e50;
  margin-right: 10px;
}

.chat-status {
  font-size: 12px;
  color: #67c23a;
}

.message-area {
  flex: 1;
  padding: 20px;
}

.message-list {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding-bottom: 20px;
}

.time-divider {
  text-align: center;
  margin: 10px 0;
}

.time-divider span {
  font-size: 12px;
  color: #999;
  background: rgba(0,0,0,0.05);
  padding: 2px 8px;
  border-radius: 10px;
}

.message-row {
  display: flex;
  flex-direction: column;
}

.message-content-wrapper {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  max-width: 70%;
}

.message-row.message-mine {
  align-items: flex-end;
}

.message-row.message-mine .message-content-wrapper {
  flex-direction: row-reverse;
}

.message-bubble {
  padding: 10px 15px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.5;
  position: relative;
  box-shadow: 0 2px 5px rgba(0,0,0,0.05);
  word-break: break-all;
}

/* Receiver Bubble (Left) */
.message-row:not(.message-mine) .message-bubble {
  background: #fff;
  color: #333;
  border-top-left-radius: 2px;
}

/* Sender Bubble (Right) - Ink Style */
.message-row.message-mine .message-bubble {
  background: #2c3e50; /* Ink color */
  color: #fff;
  border-top-right-radius: 2px;
}

.msg-image {
  max-width: 200px;
  border-radius: 8px;
}

/* Input Area */
.input-area {
  height: 160px;
  border-top: 1px solid rgba(0, 0, 0, 0.05);
  padding: 10px 20px;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.6);
}

.toolbar {
  display: flex;
  gap: 15px;
  margin-bottom: 10px;
}

.tool-icon {
  font-size: 20px;
  color: #606266;
  cursor: pointer;
  transition: color 0.3s;
}

.tool-icon:hover {
  color: #2c3e50;
}

.textarea-wrapper {
  flex: 1;
}

textarea {
  width: 100%;
  height: 100%;
  border: none;
  background: transparent;
  resize: none;
  font-size: 14px;
  font-family: inherit;
  outline: none;
  color: #333;
}

.send-action {
  display: flex;
  justify-content: flex-end;
}

.send-btn {
  background-color: #2c3e50;
  border-color: #2c3e50;
  padding: 8px 25px;
}

.send-btn:hover {
  background-color: #34495e;
  border-color: #34495e;
}

.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.3);
}

.empty-content {
  text-align: center;
  color: #999;
}

.empty-img {
  width: 150px;
  margin-bottom: 20px;
  opacity: 0.5;
}
</style>
