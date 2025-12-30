<template>
  <div class="login-container">
    <!-- 背景水墨装饰 -->
    <div class="ink-bg-layer"></div>
    
    <div class="content-wrapper">
      <div class="title-section">
        <h1 class="main-title">OmniLink</h1>
        <p class="sub-title">连接 · 泼墨 · 创想</p>
      </div>

      <div class="glass-card">
        <el-form 
          ref="loginFormRef"
          :model="loginForm"
          :rules="rules"
          class="login-form"
          :inline="false"
        >
          <div class="form-body">
            <el-form-item prop="username" class="custom-input">
              <el-input 
                v-model="loginForm.username" 
                placeholder="账号 / 用户名"
                :prefix-icon="User"
              />
            </el-form-item>
            
            <el-form-item prop="password" class="custom-input">
              <el-input 
                v-model="loginForm.password" 
                type="password" 
                placeholder="密码"
                :prefix-icon="Lock"
                show-password
                @keyup.enter="handleLogin"
              />
            </el-form-item>

            <el-form-item class="action-item">
              <el-button 
                type="primary" 
                :loading="loading" 
                class="login-button" 
                @click="handleLogin"
              >
                登 录
              </el-button>
              <div class="extra-links">
                <router-link to="/register" class="link-text">注册新账号</router-link>
              </div>
            </el-form-item>
          </div>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useStore } from 'vuex'
import { User, Lock } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const router = useRouter()
const store = useStore()
const loginFormRef = ref(null)
const loading = ref(false)

const loginForm = reactive({
  username: '',
  password: ''
})

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

const handleLogin = async () => {
  if (!loginFormRef.value) return
  
  await loginFormRef.value.validate(async (valid) => {
    if (valid) {
      loading.value = true
      try {
        const response = await request.post('/login', {
          username: loginForm.username,
          password: loginForm.password
        })

        const resData = response.data
        if (resData.code === 200) {
           const loginData = resData.data
           
           if (loginData && loginData.token) {
             store.commit('setToken', loginData.token)
             store.commit('setUserInfo', loginData)
             ElMessage.success('登录成功')
             
             // 建立 WebSocket 连接
             store.dispatch('connectWebSocket')
             
             router.push('/chat')
           } else {
             ElMessage.error('登录异常：未返回 Token')
           }
        } else {
          ElMessage.error(resData.message || '登录失败')
        }
      } catch (error) {
        console.error(error)
      } finally {
        loading.value = false
      }
    }
  })
}
</script>

<style scoped>
/* Reuse existing styles or add new ones */
.login-container {
  height: 100vh;
  width: 100vw;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #f5f7fa;
  /* Add gradients similar to Chat.vue */
  background-image: 
    radial-gradient(at 10% 10%, rgba(0,0,0,0.08) 0px, transparent 50%),
    radial-gradient(at 90% 90%, rgba(0,0,0,0.05) 0px, transparent 50%),
    linear-gradient(135deg, #ffffff 0%, #e6e9f0 100%);
  overflow: hidden;
  position: relative;
}

.ink-bg-layer {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 0;
  opacity: 0.6;
  background-image: 
    radial-gradient(circle at 30% 40%, rgba(0,0,0,0.1) 0%, transparent 40%),
    radial-gradient(circle at 70% 20%, rgba(0,0,0,0.08) 0%, transparent 35%);
  filter: blur(40px);
  pointer-events: none;
}

.content-wrapper {
  position: relative;
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 30px;
}

.title-section {
  text-align: center;
}

.main-title {
  font-size: 3rem;
  font-weight: 200;
  color: #303133;
  letter-spacing: 4px;
  margin: 0;
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
}

.sub-title {
  font-size: 1rem;
  color: #909399;
  letter-spacing: 8px;
  margin-top: 10px;
  font-weight: 300;
}

.glass-card {
  width: 380px;
  padding: 40px;
  background: rgba(255, 255, 255, 0.4);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.6);
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.1);
  border-radius: 20px;
}

.custom-input :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.5);
  box-shadow: none;
  border: 1px solid rgba(255, 255, 255, 0.3);
  border-radius: 8px;
  height: 44px;
  transition: all 0.3s;
}

.custom-input :deep(.el-input__wrapper:hover),
.custom-input :deep(.el-input__wrapper.is-focus) {
  background: rgba(255, 255, 255, 0.8);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

.login-button {
  width: 100%;
  height: 44px;
  border-radius: 8px;
  font-size: 16px;
  letter-spacing: 2px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border: none;
  box-shadow: 0 4px 15px rgba(118, 75, 162, 0.3);
  transition: all 0.3s;
}

.login-button:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(118, 75, 162, 0.4);
}

.extra-links {
  margin-top: 15px;
  text-align: center;
}

.link-text {
  color: #606266;
  font-size: 14px;
  text-decoration: none;
  transition: color 0.3s;
}

.link-text:hover {
  color: #409EFF;
}
</style>
