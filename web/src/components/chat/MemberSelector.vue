<template>
  <div class="member-selector">
    <div class="selector-header" v-if="!hideSearch">
       <el-input 
         v-model="searchKey" 
         placeholder="搜索好友" 
         prefix-icon="Search"
         clearable 
       />
    </div>
    
    <div class="selector-body custom-scrollbar">
        <el-empty v-if="filteredList.length === 0" description="暂无好友" :image-size="60" />
        <div 
          v-for="user in filteredList" 
          :key="user.user_id" 
          class="user-item"
          @click="toggleSelect(user)"
        >
            <div class="check-box">
                <el-checkbox :model-value="isSelected(user)" @click.stop="toggleSelect(user)" />
            </div>
            <el-avatar :src="normalizeUrl(user.avatar)" :size="36">
                {{ user.user_name ? user.user_name[0] : '?' }}
            </el-avatar>
            <div class="user-info">
                <div class="name">{{ user.user_name }}</div>
                <div class="uid">{{ user.user_id }}</div>
            </div>
        </div>
    </div>
    
    <div class="selector-footer">
        <span class="selected-count">已选 {{ selectedList.length }} 人</span>
        <div class="actions">
            <el-button @click="$emit('cancel')">取消</el-button>
            <el-button type="primary" :disabled="selectedList.length === 0" @click="handleConfirm">确定</el-button>
        </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getUserList, normalizeUrl } from '../../api/im'
import { useStore } from 'vuex'
import { Search } from '@element-plus/icons-vue'

const props = defineProps({
    hideSearch: Boolean,
    excludeIds: {
        type: Array,
        default: () => []
    }
})

const emit = defineEmits(['confirm', 'cancel'])

const store = useStore()
const searchKey = ref('')
const friendList = ref([])
const selectedList = ref([])

const loadFriends = async () => {
    try {
        const ownerId = store.state.userInfo.uuid
        const res = await getUserList(ownerId)
        if (res.data && res.data.data) {
            friendList.value = res.data.data
        }
    } catch (e) {
        console.error(e)
    }
}

onMounted(() => {
    loadFriends()
})

const filteredList = computed(() => {
    let list = friendList.value
    
    // 过滤掉已存在的成员
    if (props.excludeIds.length > 0) {
        list = list.filter(u => !props.excludeIds.includes(u.user_id))
    }

    if (!searchKey.value) return list
    
    const key = searchKey.value.toLowerCase()
    return list.filter(u => 
        (u.user_name && u.user_name.toLowerCase().includes(key)) ||
        (u.user_id && u.user_id.toLowerCase().includes(key))
    )
})

const isSelected = (user) => {
    return selectedList.value.some(u => u.user_id === user.user_id)
}

const toggleSelect = (user) => {
    const idx = selectedList.value.findIndex(u => u.user_id === user.user_id)
    if (idx >= 0) {
        selectedList.value.splice(idx, 1)
    } else {
        selectedList.value.push(user)
    }
}

const handleConfirm = () => {
    emit('confirm', selectedList.value)
}

</script>

<style scoped>
.member-selector {
    display: flex;
    flex-direction: column;
    height: 400px;
}

.selector-header {
    padding-bottom: 10px;
}

.selector-body {
    flex: 1;
    overflow-y: auto;
    border: 1px solid #eee;
    border-radius: 4px;
}

.user-item {
    display: flex;
    align-items: center;
    padding: 8px 10px;
    cursor: pointer;
    transition: background 0.2s;
}

.user-item:hover {
    background-color: #f5f7fa;
}

.check-box {
    margin-right: 10px;
    pointer-events: none; /* 让点击事件穿透到 user-item */
}

.user-info {
    margin-left: 10px;
    flex: 1;
    overflow: hidden;
}

.name {
    font-size: 14px;
    font-weight: 500;
    color: #333;
}

.uid {
    font-size: 12px;
    color: #999;
}

.selector-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-top: 15px;
    border-top: 1px solid #eee;
    margin-top: 10px;
}

.selected-count {
    font-size: 13px;
    color: #666;
}
</style>
