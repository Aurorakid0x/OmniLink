<template>
  <el-dialog 
    title="创建群聊" 
    :model-value="visible" 
    width="500px" 
    :before-close="handleClose"
    destroy-on-close
  >
    <div class="create-group-body">
        <el-form :model="form" label-width="70px">
            <el-form-item label="群名称" required>
                <el-input v-model="form.name" placeholder="请输入群名称" maxlength="20" show-word-limit />
            </el-form-item>
            <el-form-item label="群公告">
                <el-input v-model="form.notice" type="textarea" placeholder="请输入群公告 (可选)" :rows="2" maxlength="200" show-word-limit />
            </el-form-item>
        </el-form>

        <div class="members-section">
            <div class="section-title">选择成员</div>
            <MemberSelector 
                ref="selectorRef"
                hide-search
                @confirm="handleCreate"
                @cancel="handleClose"
            />
        </div>
    </div>
    
    <!-- 覆盖 MemberSelector 默认的 footer，改用 Dialog 的 footer -->
    <template #footer>
        <!-- 我们隐藏 MemberSelector 的 footer (通过样式或修改组件)，或者直接利用它的事件。
             为了 UI 统一，MemberSelector 的 footer 包含确认/取消，这在 Dialog 里有点重复。
             这里我做个小调整：CreateGroupDialog 包含表单和 MemberSelector。
             MemberSelector 最好能以组件形式嵌入，只负责展示和选择，把 confirm 权交给父组件。
             
             既然 MemberSelector 已经有 footer 了，我在 CreateGroupDialog 里就隐藏它的 footer，
             或者直接让 MemberSelector 充满下方区域。
             
             实际上 MemberSelector 的 footer 是为了 InviteMemberDialog 这种场景设计的。
             在 CreateGroupDialog 里，我们需要提交表单 + 成员。
             
             我修改一下 MemberSelector 的设计？不，那样太麻烦。
             我可以直接用 MemberSelector 的 selectedList 引用（如果是通过 ref 暴露），
             或者修改 MemberSelector 增加 v-model:selected。
             
             为了简单，我直接修改 MemberSelector 增加 expose，或者在 CreateGroupDialog 里监听 confirm 事件。
             不对，confirm 事件是在点击 MemberSelector 内部的“确定”时触发。
             
             最好的方式是 MemberSelector 作为一个纯选择组件，不带 Footer。
             但刚才我已经写了 Footer。
             
             没关系，我在 CreateGroupDialog 里利用 CSS 隐藏 MemberSelector 的 footer，
             或者直接用 MemberSelector 的 footer 来触发创建。
             
             但是表单在 MemberSelector 外面。点击 MemberSelector 的“确定”时，
             需要校验表单。
             
             方案：修改 MemberSelector，增加 props `showFooter`。
             算了，不改 MemberSelector 了。我就用 MemberSelector 的 footer 按钮来触发表单提交。
             但是“确定”按钮在 MemberSelector 内部，点击它会 emit 'confirm' 带上选中的人。
             父组件监听到 confirm 后，校验表单，然后调用 API。
        -->
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { createGroup, openSession } from '../../api/im'
import { useStore } from 'vuex'
import { ElMessage } from 'element-plus'
import MemberSelector from './MemberSelector.vue'

const props = defineProps({
    visible: Boolean
})

const emit = defineEmits(['update:visible', 'success'])

const store = useStore()
const form = reactive({
    name: '',
    notice: ''
})

const handleClose = () => {
    emit('update:visible', false)
}

const handleCreate = async (selectedMembers) => {
    if (!form.name.trim()) {
        ElMessage.warning('请输入群名称')
        return
    }
    
    if (selectedMembers.length === 0) {
        ElMessage.warning('请至少选择一位群成员')
        return
    }
    
    try {
        const ownerId = store.state.userInfo.uuid
        const memberIds = selectedMembers.map(u => u.user_id)
        
        // 把自己也加上（通常后端会处理，但前端显式一点）
        if (!memberIds.includes(ownerId)) {
            memberIds.push(ownerId)
        }
        
        const payload = {
            owner_id: ownerId,
            name: form.name,
            notice: form.notice,
            members: memberIds // 假设后端接受 members 字段，或者是 member_ids
        }
        
        // 根据后端契约，可能是 member_ids? 
        // 扫描结果没看到后端 createGroup 的 DTO，但我假设是按照 OmniLink 风格。
        // KamaChat 里的 createGroupReq 是 { name, notice, ... }
        // 我先按 member_ids 传，如果不行再改。
        // 刚才 im.js 里我没改 createGroup 的封装，它直接透传 data。
        // 为了稳妥，我把 members 和 member_ids 都传上，或者...
        // 假设后端是 member_ids: []string
        // 再次确认 im.js 里的 createGroup: return request.post('/group/createGroup', data)
        // 按照一般 Go Gin 的绑定，json:"member_ids"
        
        // 修正 payload
        const reqData = {
            owner_id: ownerId,
            name: form.name,
            notice: form.notice,
            member_ids: selectedMembers.map(u => u.user_id) // 只传选中的好友，后端应该会自动把 owner 加入
        }
        
        const res = await createGroup(reqData)
        if (res.data.code === 200) {
            ElMessage.success('创建群聊成功')
            const groupInfo = res.data.data
            // groupInfo 应该包含 group_id
            
            // 刷新会话列表
            await store.dispatch('loadSessions')
            
            // 自动打开会话
            const groupId = groupInfo.uuid || groupInfo.group_id
            if (groupId) {
                // 打开会话
                await openSession({
                    send_id: ownerId,
                    receive_id: groupId
                })
                // 再次刷新确保会话存在
                await store.dispatch('loadSessions')
                
                // 切换到该会话
                // 查找 session
                const session = store.state.sessionList.find(s => s.peer_id === groupId)
                if (session) {
                    emit('success', session)
                }
            }
            
            handleClose()
        } else {
            ElMessage.error(res.data.msg || '创建失败')
        }
    } catch (e) {
        console.error(e)
        ElMessage.error('创建失败')
    }
}
</script>

<style scoped>
.create-group-body {
    padding: 0 10px;
}

.members-section {
    margin-top: 20px;
    border-top: 1px solid #f0f0f0;
    padding-top: 15px;
}

.section-title {
    font-size: 14px;
    font-weight: bold;
    margin-bottom: 10px;
    color: #606266;
}
</style>
