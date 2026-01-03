<template>
  <div class="chat-window glass-panel" v-if="session">
    <!-- Header -->
    <div class="chat-header">
      <div class="header-info">
        <span class="chat-name">{{ sessionName }}</span>
        <span class="chat-status" v-if="isGroup">({{ groupMemberCount }}人)</span>
      </div>
      <div class="header-actions">
        <el-button circle icon="More" @click="$emit('toggle-right-sidebar')" />
      </div>
    </div>

    <!-- Message List -->
    <div class="message-area custom-scrollbar" ref="msgListRef" @scroll="handleScroll">
        <div v-for="msg in messages" :key="msg.uuid || msg.id" class="message-row" :class="{ 'is-mine': isMine(msg) }">
            <el-avatar :src="normalizeUrl(msg.send_avatar)" :size="36" class="msg-avatar">
                {{ msg.send_name ? msg.send_name[0] : '?' }}
            </el-avatar>
            
            <div class="msg-content-wrapper">
                <div class="msg-sender" v-if="isGroup && !isMine(msg)">{{ msg.send_name }}</div>
                
                <!-- Text Message -->
                <div class="msg-bubble" v-if="msg.type === 0 || msg.type === undefined">
                    {{ msg.content }}
                </div>
                
                <!-- Image Message -->
                <div class="msg-image" v-else-if="msg.type === 1 || (msg.type === 2 && isImage(msg.file_type || msg.url))">
                    <el-image 
                        :src="normalizeUrl(msg.url)" 
                        :preview-src-list="[normalizeUrl(msg.url)]"
                        fit="cover"
                        class="chat-image"
                    />
                </div>

                <!-- File Message -->
                <div class="msg-file" v-else-if="msg.type === 2">
                    <div class="file-icon">
                        <el-icon><Document /></el-icon>
                    </div>
                    <div class="file-info">
                        <div class="file-name">{{ msg.file_name || '未知文件' }}</div>
                        <div class="file-size">{{ msg.file_size }}</div>
                    </div>
                    <a :href="normalizeUrl(msg.url)" target="_blank" class="download-btn">
                        <el-icon><Download /></el-icon>
                    </a>
                </div>

                 <!-- Call Message (Placeholder) -->
                <div class="msg-bubble system" v-else-if="msg.type === 3">
                    [通话消息]
                </div>
            </div>
        </div>
    </div>

    <!-- Input Area -->
    <div class="input-area" v-if="sessionAllowed">
      <div class="toolbar">
        <el-upload
            class="upload-demo"
            action="#"
            :show-file-list="false"
            :http-request="handleUpload"
            :disabled="uploading"
        >
            <el-button circle icon="Folder" size="small" :loading="uploading" />
        </el-upload>
        <!-- Emoji placeholder -->
        <el-button circle icon="Picture" size="small" @click="triggerImageUpload" />
      </div>
      
      <el-input
        v-model="inputText"
        type="textarea"
        :rows="3"
        resize="none"
        placeholder="输入消息..."
        @keydown.enter.prevent="handleSend"
      />
      
      <div class="send-actions">
        <span class="tip">Enter 发送</span>
        <el-button type="primary" round @click="handleSend" :disabled="!inputText.trim()">发送</el-button>
      </div>
    </div>
    <div class="input-area blocked" v-else>
        <div class="block-msg">{{ blockReason }}</div>
    </div>
  </div>
  <div class="empty-window glass-panel" v-else>
      <el-empty description="选择一个会话开始聊天" />
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { useStore } from 'vuex'
import { More, Document, Download, Folder, Picture } from '@element-plus/icons-vue'
import { normalizeUrl, uploadFile, getGroupInfo, checkOpenSessionAllowed } from '../../api/im'
import { ElMessage } from 'element-plus'

const props = defineProps({
  session: Object,
  messages: Array
})

const emit = defineEmits(['send-message', 'toggle-right-sidebar', 'load-more'])

const store = useStore()
const inputText = ref('')
const msgListRef = ref(null)
const uploading = ref(false)

const isGroup = computed(() => {
    // A2: 使用 peer_id 判断
    if (props.session && props.session.peer_type) {
        return props.session.peer_type === 'G'
    }
    return props.session && (props.session.group_id || (props.session.receive_id && props.session.receive_id.startsWith('G')))
})

const sessionName = computed(() => {
    // A2: 优先使用 peer_name
    return props.session ? (props.session.peer_name || props.session.username || props.session.group_name) : ''
})

const groupMemberCount = ref(0)
const sessionAllowed = ref(true)
const blockReason = ref('')

const groupId = computed(() => {
    if (!props.session) return ''
    return props.session.peer_id || props.session.group_id || props.session.receive_id || ''
})

let groupCountReqSeq = 0
watch(groupId, async (id) => {
    if (!id || !id.startsWith('G')) {
        groupMemberCount.value = 0
        return
    }

    const ownerId = store.state.userInfo && store.state.userInfo.uuid
    if (!ownerId) {
        groupMemberCount.value = 0
        return
    }

    const seq = ++groupCountReqSeq
    try {
        const res = await getGroupInfo({ owner_id: ownerId, group_id: id })
        if (seq !== groupCountReqSeq) return
        if (res && res.data && res.data.code === 200 && res.data.data) {
            groupMemberCount.value = res.data.data.member_cnt || 0
            return
        }
        groupMemberCount.value = 0
    } catch (e) {
        if (seq !== groupCountReqSeq) return
        groupMemberCount.value = 0
    }
}, { immediate: true })

watch(() => props.session, async (sess) => {
    if (!sess) {
        sessionAllowed.value = true
        blockReason.value = ''
        return
    }
    const ownerId = store.state.userInfo?.uuid
    if (!ownerId) return

    // Get peer ID
    const peerId = sess.peer_id || sess.receive_id || sess.group_id
    if (!peerId) return

    // Don't check if it's the user themselves
    if (peerId === ownerId) return

    try {
        const res = await checkOpenSessionAllowed({
            send_id: ownerId,
            receive_id: peerId
        })
        if (res.data.code === 200) {
             if (res.data.data === false) {
                  sessionAllowed.value = false
                  blockReason.value = "无法发起会话"
             } else {
                 sessionAllowed.value = true
                 blockReason.value = ''
             }
        } else {
            sessionAllowed.value = false
            blockReason.value = res.data.message || "无法发起会话"
        }
    } catch (e) {
        console.error(e)
        sessionAllowed.value = false
        blockReason.value = "无法发起会话"
    }
}, { immediate: true })

const isMine = (msg) => {
    return msg.send_id === store.state.userInfo.uuid
}

const isImage = (typeOrUrl) => {
    if (!typeOrUrl) return false
    const imgExts = ['jpg', 'jpeg', 'png', 'gif', 'webp']
    return imgExts.some(ext => typeOrUrl.toLowerCase().includes(ext))
}

const loadingMore = ref(false)
const pendingPrepend = ref(null)

const isNearBottom = () => {
    const el = msgListRef.value
    if (!el) return true
    return el.scrollHeight - (el.scrollTop + el.clientHeight) < 80
}

const scrollToBottom = () => {
    nextTick(() => {
        if (msgListRef.value) {
            msgListRef.value.scrollTop = msgListRef.value.scrollHeight
        }
    })
}

const handleScroll = () => {
    const el = msgListRef.value
    if (!el || loadingMore.value) return
    if (el.scrollTop <= 20) {
        loadingMore.value = true
        pendingPrepend.value = {
            scrollHeight: el.scrollHeight,
            scrollTop: el.scrollTop,
        }
        emit('load-more')
        setTimeout(() => {
            if (loadingMore.value && pendingPrepend.value) {
                loadingMore.value = false
                pendingPrepend.value = null
            }
        }, 1200)
    }
}

watch(
    () => props.messages && props.messages.length,
    (newLen, oldLen) => {
        if (!msgListRef.value) return
        if (pendingPrepend.value) {
            nextTick(() => {
                const el = msgListRef.value
                if (!el || !pendingPrepend.value) return
                const diff = el.scrollHeight - pendingPrepend.value.scrollHeight
                el.scrollTop = pendingPrepend.value.scrollTop + diff
                pendingPrepend.value = null
                loadingMore.value = false
            })
            return
        }
        if (newLen > oldLen && isNearBottom()) {
            scrollToBottom()
        }
    }
)

watch(() => props.session, () => {
    pendingPrepend.value = null
    loadingMore.value = false
    scrollToBottom()
})

const handleSend = () => {
    if (!inputText.value.trim()) return
    emit('send-message', {
        type: 0,
        content: inputText.value
    })
    inputText.value = ''
}

const handleUpload = async (options) => {
    const { file } = options
    uploading.value = true
    try {
        const formData = new FormData()
        formData.append('file', file)
        
        const res = await uploadFile(formData)
        
        // A4: 优先使用后端返回的 url
        let fileUrl = ''
        if (res.data && (res.data.url || res.data.path || res.data.data)) {
            // 尝试从常见字段获取
             fileUrl = res.data.url || res.data.path || res.data.data
             // 如果返回的是对象，尝试取 url 属性
             if (typeof fileUrl === 'object' && fileUrl.url) {
                 fileUrl = fileUrl.url
             }
        }

        if (!fileUrl) {
            // 降级：手动拼接
            const fileName = file.name
            fileUrl = `/static/files/${fileName}`
        }
        
        // 兼容不同的成功状态码
        if (res.data.code === 200 || res.data.code === 0) { 
             const fileName = file.name
             const fileType = file.type
             
             emit('send-message', {
                 type: 2,
                 url: fileUrl,
                 file_name: fileName,
                 file_size: (file.size / 1024).toFixed(2) + 'KB',
                 file_type: fileType
             })
        } else {
            ElMessage.error('上传失败')
        }
    } catch (e) {
        ElMessage.error('上传出错')
        console.error(e)
    } finally {
        uploading.value = false
    }
}

const triggerImageUpload = () => {
    document.querySelector('.upload-demo input').click()
}
</script>

<style scoped>
.chat-window {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.4);
}

.empty-window {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(255, 255, 255, 0.4);
}

.chat-header {
  height: 60px;
  padding: 0 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.3);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.chat-name {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.message-area {
  flex: 1;
  padding: 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.message-row {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}

.message-row.is-mine {
  flex-direction: row-reverse;
}

.msg-content-wrapper {
  max-width: 60%;
  display: flex;
  flex-direction: column;
}

.is-mine .msg-content-wrapper {
  align-items: flex-end;
}

.msg-sender {
  font-size: 12px;
  color: #909399;
  margin-bottom: 4px;
}

.msg-bubble {
  background: white;
  padding: 10px 14px;
  border-radius: 12px;
  border-top-left-radius: 2px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.05);
  font-size: 14px;
  line-height: 1.5;
  word-break: break-all;
}

.is-mine .msg-bubble {
  background: #409EFF;
  color: white;
  border-top-left-radius: 12px;
  border-top-right-radius: 2px;
}

.chat-image {
    border-radius: 8px;
    max-width: 200px;
    border: 1px solid rgba(0,0,0,0.1);
}

.msg-file {
    background: white;
    padding: 10px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 200px;
    box-shadow: 0 2px 8px rgba(0,0,0,0.05);
}

.file-icon {
    font-size: 24px;
    color: #409EFF;
}

.file-info {
    flex: 1;
    overflow: hidden;
}

.file-name {
    font-size: 14px;
    color: #333;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.file-size {
    font-size: 12px;
    color: #999;
}

.download-btn {
    color: #666;
    cursor: pointer;
}

.download-btn:hover {
    color: #409EFF;
}

.input-area {
  padding: 15px;
  background: rgba(255, 255, 255, 0.5);
  border-top: 1px solid rgba(255, 255, 255, 0.3);
}

.toolbar {
  display: flex;
  gap: 10px;
  margin-bottom: 10px;
}

.send-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  margin-top: 10px;
  gap: 10px;
}

.tip {
  font-size: 12px;
  color: #909399;
}

.input-area.blocked {
    justify-content: center;
    align-items: center;
    background: rgba(245, 247, 250, 0.6);
    display: flex;
}

.block-msg {
    color: #909399;
    font-size: 14px;
}
</style>
