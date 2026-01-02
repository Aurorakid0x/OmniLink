<template>
  <el-dialog 
    title="邀请成员" 
    :model-value="visible" 
    width="500px" 
    :before-close="handleClose"
    destroy-on-close
  >
    <MemberSelector 
        ref="selectorRef"
        :exclude-ids="excludeIds"
        @confirm="handleInvite"
        @cancel="handleClose"
    />
  </el-dialog>
</template>

<script setup>
import { computed } from 'vue'
import { inviteGroupMembers } from '../../api/im'
import { useStore } from 'vuex'
import { ElMessage } from 'element-plus'
import MemberSelector from './MemberSelector.vue'

const props = defineProps({
    visible: Boolean,
    groupId: String,
    existingMembers: {
        type: Array,
        default: () => []
    }
})

const emit = defineEmits(['update:visible', 'success'])
const store = useStore()

const excludeIds = computed(() => {
    return props.existingMembers.map(m => m.user_id || m.uuid)
})

const handleClose = () => {
    emit('update:visible', false)
}

const handleInvite = async (selectedMembers) => {
    if (selectedMembers.length === 0) {
        ElMessage.warning('请至少选择一位好友')
        return
    }
    
    try {
        const ownerId = store.state.userInfo.uuid
        const res = await inviteGroupMembers({
            owner_id: ownerId,
            group_id: props.groupId,
            member_ids: selectedMembers.map(u => u.user_id)
        })
        
        if (res.data.code === 200) {
            ElMessage.success('邀请成功')
            emit('success')
            handleClose()
        } else {
            ElMessage.error(res.data.msg || '邀请失败')
        }
    } catch (e) {
        console.error(e)
        ElMessage.error('邀请失败')
    }
}
</script>
