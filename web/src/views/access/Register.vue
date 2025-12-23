<template>
  <div class="login-container">
    <!-- 背景水墨装饰 -->
    <div class="ink-bg-layer"></div>
    
    <div class="content-wrapper">
      <div class="title-section">
        <h1 class="main-title">Register</h1>
        <p class="sub-title">加入 · 连接 · 无界</p>
      </div>

      <div class="glass-card">
        <el-form 
          ref="registerFormRef"
          :model="registerForm"
          :rules="rules"
          class="login-form"
          :inline="false"
        >
          <div class="form-body">
            <div class="input-group">
              <el-form-item prop="username" class="custom-input">
                <el-input 
                  v-model="registerForm.username" 
                  placeholder="账号 (3-20位字母数字下划线)"
                  :prefix-icon="User"
                />
              </el-form-item>
              
              <el-form-item prop="nickname" class="custom-input">
                <el-input 
                  v-model="registerForm.nickname" 
                  placeholder="昵称 (2-10位字符)"
                  :prefix-icon="Postcard"
                />
              </el-form-item>
            </div>

            <div class="input-group">
              <el-form-item prop="password" class="custom-input">
                <el-input 
                  v-model="registerForm.password" 
                  type="password" 
                  placeholder="密码 (至少6位)"
                  :prefix-icon="Lock"
                  show-password
                />
              </el-form-item>

              <el-form-item prop="confirmPassword" class="custom-input">
                <el-input 
                  v-model="registerForm.confirmPassword" 
                  type="password" 
                  placeholder="确认密码"
                  :prefix-icon="CircleCheck"
                  show-password
                  @keyup.enter="handleRegister"
                />
              </el-form-item>
            </div>

            <el-form-item class="action-item">
              <el-button 
                type="primary" 
                :loading="loading" 
                class="login-button" 
                @click="handleRegister"
              >
                注 册
              </el-button>
            </el-form-item>
          </div>
          
          <div class="form-footer">
            <el-button link class="register-link" @click="goToLogin">已有账号？去登录</el-button>
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
import { User, Lock, Postcard, CircleCheck } from '@element-plus/icons-vue'
import axios from 'axios'

const store = useStore()
const router = useRouter()
const registerFormRef = ref(null)
const loading = ref(false)

const registerForm = reactive({
  username: '',
  nickname: '',
  password: '',
  confirmPassword: ''
})

// 验证确认密码
const validatePass2 = (rule, value, callback) => {
  if (value === '') {
    callback(new Error('请再次输入密码'))
  } else if (value !== registerForm.password) {
    callback(new Error('两次输入密码不一致!'))
  } else {
    callback()
  }
}

// 验证规则
const rules = {
  username: [
    { required: true, message: '请输入账号', trigger: 'blur' },
    { min: 3, max: 20, message: '账号长度应在 3 到 20 个字符之间', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_]+$/, message: '账号只能包含字母、数字和下划线', trigger: 'blur' }
  ],
  nickname: [
    { required: true, message: '请输入昵称', trigger: 'blur' },
    { min: 2, max: 10, message: '昵称长度应在 2 到 10 个字符之间', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于 6 位', trigger: 'blur' }
  ],
  confirmPassword: [
    { validator: validatePass2, trigger: 'blur' }
  ]
}

const handleRegister = async () => {
  if (!registerFormRef.value) return
  
  await registerFormRef.value.validate(async (valid) => {
    if (valid) {
      loading.value = true
      try {
        const backendUrl = store.state.backendUrl
        const response = await axios.post(`${backendUrl}/register`, {
          username: registerForm.username,
          nickname: registerForm.nickname,
          password: registerForm.password
        })

        if (response.data && response.data.code === 200) {
           ElMessage.success('注册成功，请登录')
           router.push('/login')
        } else {
           const msg = response.data?.msg || response.data?.message || '注册失败'
           ElMessage.error(msg)
        }
      } catch (error) {
        console.error(error)
        const errorMsg = error.response?.data?.msg || error.message || '注册请求失败'
        ElMessage.error(errorMsg)
      } finally {
        loading.value = false
      }
    }
  })
}

const goToLogin = () => {
  router.push('/login')
}
</script>

<style scoped>
/* 复用 Login.vue 的样式 */
@import url('https://fonts.googleapis.com/css2?family=Ma+Shan+Zheng&family=Zhi+Mang+Xing&display=swap');

.login-container {
  min-height: 100vh;
  width: 100%;
  position: relative;
  overflow: hidden;
  background-color: #f5f7fa;
  background-image: 
    radial-gradient(at 10% 10%, rgba(0,0,0,0.08) 0px, transparent 50%),
    radial-gradient(at 90% 90%, rgba(0,0,0,0.05) 0px, transparent 50%),
    linear-gradient(135deg, #ffffff 0%, #e6e9f0 100%);
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
  z-index: 1;
  width: 100%;
  height: 100vh;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  align-items: center;
  padding-bottom: 15vh;
}

.title-section {
  position: absolute;
  top: 45%;
  left: 50%;
  transform: translate(-50%, -50%);
  text-align: center;
  width: 100%;
  z-index: 0;
  pointer-events: none;
}

.main-title {
  font-family: 'Ma Shan Zheng', 'Zhi Mang Xing', cursive;
  font-size: 20vw; /* Register 字符较长，稍微调小一点点以适配 */
  line-height: 1;
  margin: 0;
  padding: 0;
  white-space: nowrap;
  color: transparent;
  background: linear-gradient(180deg, #2c3e50 0%, #000000 60%, #434343 100%);
  -webkit-background-clip: text;
  background-clip: text;
  filter: blur(0.5px) contrast(120%);
  opacity: 0.85;
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
  margin-top: -1vw;
  font-weight: bold;
  opacity: 0.6;
}

.glass-card {
  position: relative;
  z-index: 10;
  width: 90%;
  max-width: 900px;
  background: rgba(255, 255, 255, 0.65);
  backdrop-filter: blur(25px);
  -webkit-backdrop-filter: blur(25px);
  border: 1px solid rgba(255, 255, 255, 0.8);
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.1);
  border-radius: 60px;
  padding: 30px 60px;
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
  flex-direction: column; /* 注册项较多，垂直布局更合理，或者两行两列 */
  gap: 15px;
}

.input-group {
  display: flex;
  flex-direction: row;
  gap: 20px;
}

.custom-input {
  flex: 1;
  margin-bottom: 0 !important;
}

:deep(.el-input__wrapper) {
  background-color: transparent;
  box-shadow: none !important;
  border-bottom: 2px solid rgba(0, 0, 0, 0.2);
  border-radius: 0;
  padding: 10px 5px;
  transition: all 0.3s;
}

:deep(.el-input__wrapper.is-focus) {
  border-bottom: 2px solid #000;
}

:deep(.el-input__inner) {
  font-size: 1.1rem;
  color: #1a1a1a;
  font-weight: 500;
}

.action-item {
  margin-top: 10px;
  margin-bottom: 0 !important;
}

.login-button {
  width: 100%;
  height: 50px;
  border-radius: 30px;
  font-size: 1.1rem;
  font-weight: bold;
  border: none;
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

@media screen and (max-width: 768px) {
  .main-title {
    font-size: 20vw;
  }
  
  .glass-card {
    padding: 30px 20px;
    width: 92%;
    border-radius: 30px;
  }

  .input-group {
    flex-direction: column;
    gap: 15px;
  }
}
</style>
