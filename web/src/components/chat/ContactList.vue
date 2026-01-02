<template>
  <div class="contact-list glass-panel">
    <div class="header">
      <div class="tab-switch">
         <div class="tab-item" :class="{active: tab === 'friend'}" @click="tab='friend'">好友</div>
         <div class="tab-item" :class="{active: tab === 'group'}" @click="tab='group'">群组</div>
      </div>
      <el-button circle icon="Plus" size="small" @click="showAddDialog = true" />
    </div>

    <div class="list-content custom-scrollbar">
        <!-- 好友列表 -->
        <div v-if="tab === 'friend'">
            <!-- 新的朋友入口 -->
            <div class="new-friends-entry" @click="openNewFriends">
                <div class="icon-box">
                    <el-icon><UserFilled /></el-icon>
                </div>
                <div class="entry-name">新的朋友</div>
                <div class="badge" v-if="pendingCount > 0">{{ pendingCount }}</div>
            </div>

            <el-empty v-if="friendList.length === 0" description="暂无好友" :image-size="60" />
            <div v-for="user in friendList" :key="user.user_id" class="contact-item" @click="handleChat(user)">
                <el-avatar :src="normalizeUrl(user.avatar)" :size="40">{{ user.user_name ? user.user_name[0] : '?' }}</el-avatar>
                <div class="contact-info">
                    <div class="name">{{ user.user_name }}</div>
                </div>
            </div>
        </div>

        <!-- 群组列表 -->
        <div v-if="tab === 'group'">
            <el-empty v-if="groupList.length === 0" description="暂无群组" :image-size="60" />
            <div v-for="group in groupList" :key="group.group_id" class="contact-item" @click="handleChat(group)">
                <el-avatar :src="normalizeUrl(group.avatar)" shape="square" :size="40">{{ group.group_name ? group.group_name[0] : '?' }}</el-avatar>
                <div class="contact-info">
                    <div class="name">{{ group.group_name }}</div>
                </div>
            </div>
        </div>
    </div>
    
    <!-- 添加好友/群组弹窗 -->
    <el-dialog v-model="showAddDialog" title="添加好友" width="300px">
        <div class="add-form">
            <el-input v-model="addForm.peerId" placeholder="输入用户ID (如 U123...)" style="margin-bottom: 10px" />
            <el-input v-model="addForm.message" placeholder="验证消息 (可选)" type="textarea" :rows="2" />
        </div>
        <template #footer>
            <span class="dialog-footer">
                <el-button @click="showAddDialog = false">取消</el-button>
                <el-button type="primary" @click="handleApply">发送申请</el-button>
            </span>
        </template>
    </el-dialog>

    <!-- 新的朋友列表弹窗/抽屉 -->
    <el-drawer v-model="showNewFriends" title="新的朋友" direction="rtl" size="320px">
        <div class="new-friend-list custom-scrollbar">
            <el-empty v-if="pendingList.length === 0" description="暂无申请" />
            <div v-for="item in pendingList" :key="item.uuid || item.id" class="new-friend-item">
                <el-avatar :src="normalizeUrl(item.avatar)" :size="40">{{ (item.username || item.user_id || '?')[0] }}</el-avatar>
                <div class="info">
                    <div class="name">{{ item.nickname || item.username || item.user_id }}</div>
                    <div class="msg">{{ item.message || '请求添加你为好友' }}</div>
                </div>
                <div class="actions">
                    <el-button circle size="small" type="success" :icon="Check" @click="handleAccept(item)" />
                    <el-button circle size="small" type="danger" :icon="Close" @click="handleReject(item)" />
                </div>
            </div>
        </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { useStore } from 'vuex'
import { getUserList, loadMyJoinedGroup, normalizeUrl, checkOpenSessionAllowed, openSession, applyContact, passContactApply, refuseContactApply } from '../../api/im'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, UserFilled, Check, Close } from '@element-plus/icons-vue'

const emit = defineEmits(['start-chat'])
const store = useStore()
const tab = ref('friend')
const friendList = ref([])
const groupList = ref([])
const showAddDialog = ref(false)
const showNewFriends = ref(false)
const addForm = ref({
    peerId: '',
    message: ''
})

const pendingList = computed(() => store.state.pendingApplyList)
const pendingCount = computed(() => store.getters.pendingApplyCount)

const loadData = async () => {
    try {
        const ownerId = store.state.userInfo.uuid
        // 加载好友
        const userRes = await getUserList(ownerId)
        if (userRes.data && userRes.data.data) {
            friendList.value = userRes.data.data
        }
        
        // 加载群组
        try {
            const groupRes = await loadMyJoinedGroup(ownerId)
            if (groupRes.data && groupRes.data.data) {
                groupList.value = groupRes.data.data
            }
        } catch (e) {
            console.error('Failed to load groups', e)
        }
    } catch (e) {
        console.error(e)
    }
}

const handleChat = async (item) => {
    const sendId = store.state.userInfo.uuid
    // A2: 确保获取正确的 peerId
    const receiveId = item.user_id || item.group_id
    
    // 检查是否允许会话
    try {
        const checkRes = await checkOpenSessionAllowed({ send_id: sendId, receive_id: receiveId })
        if (checkRes.data.code === 200 && checkRes.data.data === true) {
            const sessionRes = await openSession({ send_id: sendId, receive_id: receiveId })
            if (sessionRes.data.code === 200) {
                const sess = sessionRes.data.data
                const sessionId = sess && sess.session_id ? sess.session_id : ''
                emit('start-chat', {
                    session_id: sessionId,
                    receive_id: receiveId,
                    username: (sess && sess.peer_name) ? sess.peer_name : item.user_name,
                    group_name: item.group_name,
                    avatar: (sess && sess.peer_avatar) ? sess.peer_avatar : item.avatar
                })
            }
        } else {
            ElMessage.warning(checkRes.data.message || '无法发起会话')
        }
    } catch (e) {
        ElMessage.error('发起会话失败')
        console.error(e)
    }
}

const handleApply = async () => {
    if (!addForm.value.peerId) {
        ElMessage.warning('请输入用户ID')
        return
    }
    try {
        const res = await applyContact({
            owner_id: store.state.userInfo.uuid,
            contact_id: addForm.value.peerId,
            message: addForm.value.message
        })
        if (res.data.code === 200) {
            ElMessage.success('申请已发送')
            showAddDialog.value = false
            addForm.value = { peerId: '', message: '' }
        } else {
            ElMessage.error(res.data.message || '申请失败')
        }
    } catch (e) {
        console.error(e)
        // 优雅降级：如果接口不存在
        if (e.response && e.response.status === 404) {
             ElMessage.error('发送申请失败：接口不存在')
        } else {
             ElMessage.error('发送申请失败')
        }
    }
}

const openNewFriends = () => {
    showNewFriends.value = true
    store.dispatch('loadPendingApplies')
}

const handleAccept = async (item) => {
    try {
        const res = await passContactApply({
             apply_id: item.uuid || item.id, // 兼容不同字段
             owner_id: store.state.userInfo.uuid
        })
        if (res.data.code === 200) {
            ElMessage.success('已同意')
            store.dispatch('loadPendingApplies')
            loadData() // 刷新好友列表
        } else {
            ElMessage.error(res.data.message || '操作失败')
        }
    } catch (e) {
         console.error(e)
         ElMessage.error('操作失败')
    }
}

const handleReject = async (item) => {
     try {
        const res = await refuseContactApply({
             apply_id: item.uuid || item.id,
             owner_id: store.state.userInfo.uuid
        })
        if (res.data.code === 200) {
            ElMessage.success('已拒绝')
            store.dispatch('loadPendingApplies')
        } else {
            ElMessage.error(res.data.message || '操作失败')
        }
    } catch (e) {
         console.error(e)
         ElMessage.error('操作失败')
    }
}

onMounted(() => {
    loadData()
    store.dispatch('loadPendingApplies')
})
</script>

<style scoped>
.contact-list {
  width: 280px;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.3);
  border-right: 1px solid rgba(255, 255, 255, 0.3);
}

.header {
  padding: 15px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
}

.tab-switch {
    display: flex;
    background: rgba(255,255,255,0.4);
    border-radius: 15px;
    padding: 2px;
}

.tab-item {
    padding: 5px 15px;
    border-radius: 12px;
    font-size: 13px;
    cursor: pointer;
    color: #666;
}

.tab-item.active {
    background: white;
    color: #409EFF;
    box-shadow: 0 2px 5px rgba(0,0,0,0.05);
}

.list-content {
    flex: 1;
    overflow-y: auto;
    padding: 10px 0;
}

.contact-item {
    padding: 10px 20px;
    display: flex;
    align-items: center;
    gap: 12px;
    cursor: pointer;
    transition: all 0.2s;
}

.contact-item:hover {
    background: rgba(255, 255, 255, 0.4);
}

.name {
    font-size: 14px;
    color: #333;
}

.new-friends-entry {
    padding: 10px 20px;
    display: flex;
    align-items: center;
    gap: 12px;
    cursor: pointer;
    transition: all 0.2s;
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.new-friends-entry:hover {
    background: rgba(255, 255, 255, 0.4);
}

.icon-box {
    width: 40px;
    height: 40px;
    border-radius: 4px;
    background: #fa9d3b;
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
    font-size: 20px;
}

.entry-name {
    font-size: 14px;
    color: #333;
    flex: 1;
}

.badge {
    background-color: #f56c6c;
    color: white;
    font-size: 12px;
    padding: 2px 6px;
    border-radius: 10px;
    line-height: 1;
}

.new-friend-item {
    display: flex;
    align-items: center;
    padding: 10px;
    border-bottom: 1px solid #eee;
    gap: 10px;
}

.new-friend-item .info {
    flex: 1;
    overflow: hidden;
}

.new-friend-item .name {
    font-size: 14px;
    font-weight: 500;
}

.new-friend-item .msg {
    font-size: 12px;
    color: #999;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.new-friend-item .actions {
    display: flex;
    gap: 5px;
}
</style>
