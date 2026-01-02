<template>
  <div class="group-info glass-panel">
      <div class="info-header">
          <h3>详情</h3>
      </div>
      
      <div class="info-content" v-if="info">
          <div class="avatar-area">
              <el-avatar :src="normalizeUrl(info.contact_avatar || info.avatar)" :size="80" />
              <div class="name">{{ info.contact_name || info.name }}</div>
              <div class="uuid">ID: {{ info.contact_id || info.uuid || info.group_id }}</div>
          </div>
          
          <el-divider />
          
          <div class="meta-area" v-if="isFriend">
              <div class="meta-row">
                  <span class="meta-label">性别</span>
                  <span class="meta-value">{{ formatGender(info.gender) }}</span>
              </div>
              <div class="meta-row">
                  <span class="meta-label">生日</span>
                  <span class="meta-value">{{ formatBirthday(info.birthday) }}</span>
              </div>
          </div>

          <div class="meta-area" v-if="isGroup">
              <div class="meta-row">
                  <span class="meta-label">群主</span>
                  <span class="meta-value">{{ info.owner_id }}</span>
              </div>
              <div class="meta-row">
                  <span class="meta-label">成员数</span>
                  <span class="meta-value">{{ info.member_cnt || groupMembers.length }}</span>
              </div>
          </div>

          <div class="desc-area">
              <label>{{ isGroup ? '群公告' : '签名' }}</label>
              <p>{{ (info.contact_signature || info.notice) || '暂无' }}</p>
          </div>
          
          <div class="members-area" v-if="isGroup">
               <div class="members-header">
                   <label>群成员 ({{ groupMembers.length }})</label>
                   <el-button link type="primary" @click="showInvite = true">邀请</el-button>
               </div>
               <div class="members-list custom-scrollbar">
                   <div v-for="m in groupMembers" :key="m.user_id" class="member-item">
                       <el-avatar :src="normalizeUrl(m.avatar)" :size="30" />
                       <span class="member-name text-ellipsis">{{ m.nickname || m.username || m.user_id }}</span>
                   </div>
               </div>
          </div>

          <div class="actions">
              <el-button type="danger" plain @click="handleDelete" v-if="isFriend">删除好友</el-button>
              
              <template v-if="isGroup">
                  <el-button type="danger" plain @click="handleDismiss" v-if="isOwner">解散群聊</el-button>
                  <el-button type="danger" plain @click="handleLeave" v-else>退出群聊</el-button>
              </template>
          </div>
      </div>
      <el-empty v-else description="加载中..." />
      
      <InviteMemberDialog 
        v-if="isGroup"
        v-model:visible="showInvite"
        :group-id="peerId"
        :existing-members="groupMembers"
        @success="handleInviteSuccess"
      />
  </div>
</template>

<script setup>
import { ref, onMounted, watch, computed } from 'vue'
import { getContactInfo, getGroupInfo, getGroupMemberList, leaveGroup, dismissGroup, normalizeUrl, deleteContact, deleteSession } from '../../api/im'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useStore } from 'vuex'
import InviteMemberDialog from './InviteMemberDialog.vue'

const props = defineProps({
  session: Object
})

const store = useStore()
const info = ref(null)
const groupMembers = ref([])
const showInvite = ref(false)

const peerId = computed(() => {
    // A2: 优先使用 peer_id
    if (props.session) {
        return props.session.peer_id || props.session.user_id || props.session.group_id || props.session.receive_id
    }
    return null
})

const isGroup = computed(() => peerId.value && peerId.value.startsWith('G'))
const isFriend = computed(() => peerId.value && peerId.value.startsWith('U'))
const isOwner = computed(() => isGroup.value && info.value && info.value.owner_id === store.state.userInfo.uuid)

const loadInfo = async () => {
    if (!peerId.value) return
    try {
        if (isFriend.value) {
            const res = await getContactInfo(peerId.value)
            if (res.data.code === 200) {
                info.value = res.data.data
            }
        } else if (isGroup.value) {
             const ownerId = store.state.userInfo.uuid
             // 获取群详情
             const gRes = await getGroupInfo({ 
                 owner_id: ownerId,
                 group_id: peerId.value 
             })
             if (gRes.data.code === 200) {
                 info.value = gRes.data.data
             }
             
             // 获取群成员
             const mRes = await getGroupMemberList({
                 owner_id: ownerId,
                 group_id: peerId.value
             })
             if (mRes.data.code === 200) {
                 groupMembers.value = mRes.data.data || []
             }
        }
    } catch (e) {
        console.error(e)
    }
}

watch(() => props.session, () => {
    info.value = null
    groupMembers.value = []
    loadInfo()
}, { immediate: true })

const handleInviteSuccess = () => {
    loadInfo()
}

const handleDelete = () => {
    ElMessageBox.confirm('确定删除该好友吗？', '提示', { type: 'warning' })
    .then(async () => {
        const res = await deleteContact({ 
            owner_id: store.state.userInfo.uuid, 
            contact_id: peerId.value 
        })
        if (res.data.code === 200) {
            ElMessage.success('已删除')
            window.location.reload()
        }
    }).catch(() => {})
}

const handleLeave = () => {
     ElMessageBox.confirm('确定退出该群聊吗？', '提示', { type: 'warning' })
    .then(async () => {
        const res = await leaveGroup({ 
            owner_id: store.state.userInfo.uuid, 
            group_id: peerId.value 
        })
        if (res.data.code === 200) {
            ElMessage.success('已退出')
            store.dispatch('loadSessions')
            // 清除当前会话
            store.commit('setCurrentSession', { sessionId: null, peerId: null })
        } else {
             ElMessage.error(res.data.msg || '操作失败')
        }
    }).catch(() => {})
}

const handleDismiss = () => {
     ElMessageBox.confirm('确定解散该群聊吗？此操作不可逆！', '警告', { type: 'error' })
    .then(async () => {
        const res = await dismissGroup({ 
            owner_id: store.state.userInfo.uuid, 
            group_id: peerId.value 
        })
        if (res.data.code === 200) {
            ElMessage.success('已解散')
            store.dispatch('loadSessions')
            store.commit('setCurrentSession', { sessionId: null, peerId: null })
        } else {
             ElMessage.error(res.data.msg || '操作失败')
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
    text-align: center;
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
    word-break: break-all;
}

.members-area {
    margin-top: 20px;
}

.members-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;
}

.members-header label {
    font-size: 12px;
    color: #666;
}

.members-list {
    max-height: 200px;
    overflow-y: auto;
    background: rgba(255,255,255,0.3);
    border-radius: 8px;
    padding: 5px;
}

.member-item {
    display: flex;
    align-items: center;
    padding: 5px;
    gap: 8px;
}

.member-name {
    font-size: 13px;
    color: #333;
    flex: 1;
}

.actions {
    margin-top: 30px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    align-items: center;
}
</style>

