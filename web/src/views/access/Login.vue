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
            </el-form-item>
          </div>
          
          <div class="form-footer">
            <el-button link class="register-link" @click="goToRegister">注册新账号</el-button>
          </div>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useStore } from 'vuex'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import axios from 'axios'

const store = useStore()
const router = useRouter()
const loginFormRef = ref(null)
const loading = ref(false)

const loginForm = reactive({
  username: '',
  password: ''
})

// 验证规则
const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 20, message: '用户名长度应在 3 到 20 个字符之间', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_]+$/, message: '用户名只能包含字母、数字和下划线', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于 6 位', trigger: 'blur' }
  ]
}

const handleLogin = async () => {
  if (!loginFormRef.value) return
  
  await loginFormRef.value.validate(async (valid) => {
    if (valid) {
      loading.value = true
      try {
        const backendUrl = store.state.backendUrl
        const response = await axios.post(`${backendUrl}/login`, {
          username: loginForm.username,
          password: loginForm.password
        })

        if (response.data) {
           const userInfo = response.data.data || response.data
           store.commit('setUserInfo', userInfo)
           ElMessage.success('登录成功')
           router.push('/chat')
        } else {
           ElMessage.error('登录失败：服务器无响应')
        }
      } catch (error) {
        console.error(error)
        const errorMsg = error.response?.data?.msg || error.message || '登录请求失败'
        ElMessage.error(errorMsg)
      } finally {
        loading.value = false
      }
    }
  })
}

const goToRegister = () => {
  router.push('/register')
}
</script>

<style scoped>
/* 引入行书/书法字体 */
@import url('https://fonts.googleapis.com/css2?family=Ma+Shan+Zheng&family=Zhi+Mang+Xing&display=swap');

.login-container {
  min-height: 100vh;
  width: 100%;
  position: relative;
  overflow: hidden;
  /* 背景：黑白油墨国画风格 */
  background-color: #f5f7fa;
  background-image: 
    radial-gradient(at 10% 10%, rgba(0,0,0,0.08) 0px, transparent 50%),
    radial-gradient(at 90% 90%, rgba(0,0,0,0.05) 0px, transparent 50%),
    linear-gradient(135deg, #ffffff 0%, #e6e9f0 100%);
}

/* 模拟水墨晕染层 */
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
  z-index: 1;
  width: 100%;
  height: 100vh;
  display: flex;
  flex-direction: column;
  justify-content: flex-end; /* 让内容靠下，方便卡片遮挡标题 */
  align-items: center;
  padding-bottom: 15vh; /* 底部留白，控制卡片位置 */
}

/* 标题区域 */
.title-section {
  position: absolute;
  top: 45%; /* 标题整体位置偏上 */
  left: 50%;
  transform: translate(-50%, -50%);
  text-align: center;
  width: 100%;
  z-index: 0; /* 在卡片下方 */
  pointer-events: none; /* 不阻挡点击 */
}

.main-title {
  /* 字体设置：行书/书法感 */
  font-family: 'Ma Shan Zheng', 'Zhi Mang Xing', 'STXingkai', '华文行楷', cursive;
  /* 巨大尺寸，铺满屏幕 */
  font-size: 25vw; 
  line-height: 1;
  margin: 0;
  padding: 0;
  white-space: nowrap;
  
  /* 泼墨质感：深色渐变 + 纹理 */
  color: transparent;
  background: linear-gradient(180deg, #2c3e50 0%, #000000 60%, #434343 100%);
  -webkit-background-clip: text;
  background-clip: text;
  
  /* 增加墨迹晕染的模糊感 */
  filter: blur(0.5px) contrast(120%);
  opacity: 0.85;
  
  /* 墨水滴落/流动动画感 (轻微浮动) */
  animation: float-ink 8s ease-in-out infinite;
}

@keyframes float-ink {
  0%, 100% { transform: translateY(0) scale(1); }
  50% { transform: translateY(-10px) scale(1.02); }
}

.sub-title {
  font-family: 'Noto Serif SC', serif;
  font-size: 1.5rem;
  color: #555;
  letter-spacing: 1em;
  margin-top: -2vw; /* 紧贴大标题底部 */
  font-weight: bold;
  opacity: 0.6;
}

/* 玻璃卡片 */
.glass-card {
  position: relative;
  z-index: 10; /* 确保在标题上方 */
  width: 90%;
  max-width: 900px;
  
  /* 玻璃拟态：半透明，稍微厚重一点以遮挡文字 */
  background: rgba(255, 255, 255, 0.65);
  backdrop-filter: blur(25px);
  -webkit-backdrop-filter: blur(25px);
  
  border: 1px solid rgba(255, 255, 255, 0.8);
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.1);
  border-radius: 60px; /* 更大的圆角 */
  padding: 30px 60px;
  
  /* 关键：稍微上移，遮挡住标题底部一点点 */
  margin-bottom: 5vh; 
  transform: translateY(0);
  transition: all 0.4s ease;
}

.glass-card:hover {
  background: rgba(255, 255, 255, 0.8);
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.15);
  transform: translateY(-5px);
}

.form-body {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}

.custom-input {
  flex: 1;
  margin-bottom: 0 !important;
}

/* 输入框内部样式 */
:deep(.el-input__wrapper) {
  background-color: transparent; /* 透明以透出玻璃感 */
  box-shadow: none !important;
  border-bottom: 2px solid rgba(0, 0, 0, 0.2);
  border-radius: 0;
  padding: 10px 5px;
  transition: all 0.3s;
}

:deep(.el-input__wrapper.is-focus) {
  border-bottom: 2px solid #000; /* 聚焦变为纯黑，呼应水墨 */
}

:deep(.el-input__inner) {
  font-size: 1.2rem;
  color: #1a1a1a;
  font-weight: 500;
}

/* 按钮样式：黑白风格中的点缀，或者保持黑白 */
.login-button {
  width: 100%;
  height: 50px;
  border-radius: 30px;
  font-size: 1.1rem;
  font-weight: bold;
  border: none;
  /* 黑色油墨按钮 */
  background: linear-gradient(135deg, #2b2b2b 0%, #000000 100%);
  color: #fff;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.3);
  transition: all 0.3s ease;
}

.login-button:hover {
  background: linear-gradient(135deg, #000000 0%, #2b2b2b 100%);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.4);
  transform: scale(1.02);
}

.form-footer {
  text-align: center;
  margin-top: 15px;
}

.register-link {
  color: #666;
  font-size: 0.9rem;
  font-weight: 500;
}

.register-link:hover {
  color: #000;
}

/* 响应式适配 */
@media screen and (max-width: 768px) {
  .main-title {
    font-size: 25vw; /* 手机上更大 */
    writing-mode: vertical-rl; /* 手机上尝试竖排，或者保持横排但换行 */
    text-orientation: upright;
    writing-mode: horizontal-tb; /* 保持横排比较安全 */
    white-space: normal;
    line-height: 1.1;
  }
  
  .title-section {
    top: 35%; /* 手机上标题位置 */
  }

  .content-wrapper {
    padding-bottom: 5vh;
  }

  .glass-card {
    padding: 30px 20px;
    width: 92%;
    border-radius: 30px;
    margin-bottom: 20px;
  }

  .form-body {
    flex-direction: column;
    gap: 20px;
  }

  .custom-input, .action-item {
    width: 100%;
  }
}
</style>
