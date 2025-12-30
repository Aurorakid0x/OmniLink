<template>
  <div class="group-info glass-panel">
      <div class="info-header">
          <h3>详情</h3>
      </div>
      
      <div class="info-content" v-if="info">
          <div class="avatar-area">
              <el-avatar :src="normalizeUrl(info.contact_avatar)" :size="80" />
              <div class="name">{{ info.contact_name }}</div>
              <div class="uuid">ID: {{ info.contact_id }}</div>
          </div>
          
          <el-divider />
          
          <div class="meta-area">
              <div class="meta-row">
                  <span class="meta-label">性别</span>
                  <span class="meta-value">{{ formatGender(info.gender) }}</span>
              </div>
              <div class="meta-row">
                  <span class="meta-label">生日</span>
                  <span class="meta-value">{{ formatBirthday(info.birthday) }}</span>
              </div>
          </div>

          <div class="desc-area">
              <label>签名</label>
              <p>{{ info.contact_signature || '暂无' }}</p>
          </div>
          
          <div class="actions">
              <el-button type="danger" plain @click="handleDelete" v-if="isFriend">删除好友</el-button>
              <el-button type="danger" plain @click="handleLeave" v-if="isGroup">退出群聊</el-button>
          </div>
      </div>
      <el-empty v-else description="加载中..." />
  </div>
</template>

<script setup>
import { ref, onMounted, watch, computed } from 'vue'
import { getContactInfo, normalizeUrl, deleteContact, deleteSession } from '../../api/im'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useStore } from 'vuex'

const props = defineProps({
  session: Object
})

const store = useStore()
const info = ref(null)

const peerId = computed(() => {
    // A2: 优先使用 peer_id
    if (props.session) {
        return props.session.peer_id || props.session.user_id || props.session.group_id || props.session.receive_id
    }
    return null
})

const isGroup = computed(() => peerId.value && peerId.value.startsWith('G'))
const isFriend = computed(() => peerId.value && peerId.value.startsWith('U'))

const loadInfo = async () => {
    if (!peerId.value) return
    try {
        const res = await getContactInfo(peerId.value)
        if (res.data.code === 200) {
            info.value = res.data.data
        }
    } catch (e) {
        console.error(e)
    }
}

watch(() => props.session, () => {
    info.value = null
    loadInfo()
}, { immediate: true })

const handleDelete = () => {
    ElMessageBox.confirm('确定删除该好友吗？', '提示', { type: 'warning' })
    .then(async () => {
        const res = await deleteContact({ 
            owner_id: store.state.userInfo.uuid, 
            contact_id: peerId.value 
        })
        if (res.data.code === 200) {
            ElMessage.success('已删除')
            // 简单处理：刷新页面或清理会话
            window.location.reload()
        }
    }).catch(() => {})
}

const formatGender = (g) => {
    if (g === 0) return '男'
    if (g === 1) return '女'
    return '未知'
}

const formatBirthday = (b) => {
    if (!b) return '暂无'
    if (typeof b === 'string' && b.length === 8) {
        return `${b.slice(0, 4)}-${b.slice(4, 6)}-${b.slice(6, 8)}`
    }
    return b
}

const handleLeave = () => {
     ElMessage.info('退群功能待完善')
}

</script>

<style scoped>
.group-info {
  width: 240px;
  height: 100%;
  border-left: 1px solid rgba(255, 255, 255, 0.3);
  background: rgba(255, 255, 255, 0.2);
  display: flex;
  flex-direction: column;
}

.info-header {
    padding: 20px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.2);
}

.info-content {
    padding: 20px;
    flex: 1;
    overflow-y: auto;
}

.avatar-area {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
}

.name {
    font-size: 18px;
    font-weight: 600;
}

.uuid {
    font-size: 12px;
    color: #999;
}

.meta-area {
    margin-bottom: 15px;
    background: rgba(255,255,255,0.35);
    border-radius: 8px;
    padding: 10px;
}

.meta-row {
    display: flex;
    justify-content: space-between;
    font-size: 13px;
    color: #333;
    line-height: 22px;
}

.meta-label {
    color: #666;
}

.desc-area label {
    font-size: 12px;
    color: #666;
    margin-bottom: 5px;
    display: block;
}

.desc-area p {
    font-size: 14px;
    color: #333;
    background: rgba(255,255,255,0.4);
    padding: 10px;
    border-radius: 8px;
}

.actions {
    margin-top: 30px;
    display: flex;
    justify-content: center;
}
</style>
