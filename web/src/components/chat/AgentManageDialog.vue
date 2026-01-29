<template>
  <el-dialog
    v-model="dialogVisible"
    title="Agent 管理"
    width="800px"
    append-to-body
  >
    <div class="agent-manage-content">
      <!-- Agent列表 -->
      <div class="agent-list-section">
        <div class="section-header">
          <h4>我的 Agent</h4>
          <el-button size="small" type="primary" @click="openCreateAgent">
            <el-icon><Plus /></el-icon> 创建新 Agent
          </el-button>
        </div>

        <div class="agent-grid">
          <div 
            v-for="agent in agents" 
            :key="agent.agent_id"
            class="agent-card"
            :class="{ selected: selectedAgent?.agent_id === agent.agent_id }"
            @click="selectAgent(agent)"
          >
            <div class="agent-card-header">
              <el-icon class="agent-icon"><UserFilled /></el-icon>
              <el-tag v-if="agent.is_system_global" size="small" type="primary">系统</el-tag>
            </div>
            <div class="agent-card-body">
              <h5>{{ agent.name }}</h5>
              <p class="agent-desc">{{ agent.description || '暂无描述' }}</p>
              <div class="agent-meta">
                <el-tag size="small" effect="plain">
                  {{ agent.kb_type === 'global' ? '全局知识库' : '私有知识库' }}
                </el-tag>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Agent会话列表 -->
      <div class="agent-sessions-section" v-if="selectedAgent">
        <div class="section-header">
          <h4>{{ selectedAgent.name }} 的会话</h4>
          <el-button 
            size="small" 
            @click="createSessionForAgent"
            :disabled="selectedAgent.is_system_global"
          >
            <el-icon><Plus /></el-icon> 新建会话
          </el-button>
        </div>

        <el-empty v-if="agentSessions.length === 0" description="暂无会话" />
        <div v-else class="session-list-mini">
          <div 
            v-for="session in agentSessions" 
            :key="session.session_id"
            class="session-item-mini"
          >
            <span>{{ session.title }}</span>
            <el-button 
              link 
              type="danger" 
              size="small"
              v-if="session.is_deletable"
              @click="deleteSession(session.session_id)"
            >
              删除
            </el-button>
          </div>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useStore } from 'vuex'
import { Plus, UserFilled } from '@element-plus/icons-vue'
import { getAgents, getSessions, createSession } from '../../api/ai'
import { ElMessage } from 'element-plus'

const props = defineProps({
  visible: Boolean
})

const emit = defineEmits(['update:visible'])

const store = useStore()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val)
})

const agents = ref([])
const selectedAgent = ref(null)
const agentSessions = ref([])

// 加载Agents
const loadAgents = async () => {
  try {
    const res = await getAgents()
    if (res.data && res.data.code === 200) {
      agents.value = res.data.data?.agents || []
    }
  } catch (error) {
    console.error('Failed to load agents:', error)
  }
}

// 选择Agent
const selectAgent = async (agent) => {
  selectedAgent.value = agent
  
  // 加载该Agent的会话列表
  try {
    const res = await getSessions({ agent_id: agent.agent_id })
    if (res.data && res.data.code === 200) {
      agentSessions.value = res.data.data?.sessions || []
    }
  } catch (error) {
    console.error('Failed to load agent sessions:', error)
  }
}

// 创建会话
const createSessionForAgent = async () => {
  if (!selectedAgent.value) return
  
  try {
    const res = await createSession({
      agent_id: selectedAgent.value.agent_id,
      title: '新对话'
    })
    if (res.data && res.data.code === 200) {
      ElMessage.success('会话创建成功')
      selectAgent(selectedAgent.value) // 刷新会话列表
      // 刷新主界面的AI会话列表
      store.dispatch('loadAISessions')
    }
  } catch (error) {
    ElMessage.error('创建会话失败')
  }
}

// 删除会话
const deleteSession = async (sessionId) => {
  // TODO: 实现删除会话逻辑（需要后端API）
  ElMessage.info('删除功能开发中')
}

// 打开创建Agent弹窗
const openCreateAgent = () => {
  // TODO: 实现创建Agent逻辑（复用 Assistant.vue 的实现）
  ElMessage.info('创建Agent功能开发中')
}

// 监听弹窗打开
watch(dialogVisible, (val) => {
  if (val) {
    loadAgents()
  }
})
</script>

<style scoped>
.agent-manage-content {
  display: flex;
  gap: 20px;
  min-height: 400px;
}

.agent-list-section {
  flex: 1;
}

.agent-sessions-section {
  flex: 1;
  border-left: 1px solid #eee;
  padding-left: 20px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.section-header h4 {
  margin: 0;
}

.agent-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 15px;
}

.agent-card {
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  padding: 15px;
  cursor: pointer;
  transition: all 0.3s;
}

.agent-card:hover {
  border-color: #8a2be2;
  box-shadow: 0 4px 12px rgba(138, 43, 226, 0.1);
}

.agent-card.selected {
  border-color: #8a2be2;
  background: rgba(138, 43, 226, 0.05);
}

.agent-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.agent-icon {
  font-size: 24px;
  color: #8a2be2;
}

.agent-card-body h5 {
  margin: 0 0 8px;
  font-size: 16px;
}

.agent-desc {
  font-size: 12px;
  color: #999;
  margin-bottom: 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.agent-meta {
  display: flex;
  gap: 5px;
}

.session-list-mini {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.session-item-mini {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px;
  background: #f5f5f5;
  border-radius: 4px;
}

.session-item-mini:hover {
  background: #e8e8e8;
}
</style>
