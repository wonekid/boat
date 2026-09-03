<template>
  <div class="login-wrap">
    <el-card class="login-card">
      <div class="brand">Boat 运维管理平台</div>
      <el-form :model="form" label-width="0" @keyup.enter="onLogin" v-if="!mfaRequired">
        <el-form-item>
          <el-input v-model="form.username" placeholder="用户名" size="large" :prefix-icon="User" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="form.password" type="password" placeholder="密码" size="large" :prefix-icon="Lock" show-password />
        </el-form-item>
        <el-form-item>
          <div class="captcha-row">
            <el-input v-model="form.code" placeholder="验证码（演示）" size="large" style="flex:1" />
            <div class="captcha-box" @click="refreshCaptcha">{{ captcha }}</div>
          </div>
        </el-form-item>
        <el-button type="primary" size="large" style="width:100%" :loading="loading" @click="onLogin">登 录</el-button>
      </el-form>

      <el-form :model="mfaForm" label-width="0" @keyup.enter="onMfaVerify" v-else>
        <el-alert type="warning" :closable="false" title="已开启两步验证" description="请输入身份认证器（如 Google Authenticator）上的动态码" style="margin-bottom:16px" />
        <el-form-item>
          <el-input v-model="mfaForm.code" placeholder="6 位动态验证码" size="large" maxlength="6" />
        </el-form-item>
        <el-button type="primary" size="large" style="width:100%" :loading="loading" @click="onMfaVerify">验 证</el-button>
        <el-button size="large" style="width:100%;margin-top:8px;margin-left:0" @click="backToLogin">返回</el-button>
      </el-form>
      <div class="tip">默认账号：admin / admin123</div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { User, Lock } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import api from '@/api'
import { useUserStore } from '@/store/user'

const router = useRouter()
const userStore = useUserStore()
const form = reactive({ username: 'admin', password: 'admin123', code: '' })
const mfaForm = reactive({ code: '', mfaToken: '' })
const mfaRequired = ref(false)
const loading = ref(false)
const captcha = ref('')

async function refreshCaptcha() {
  try {
    const r = await api.captcha()
    captcha.value = r.code
  } catch (e) { /* ignore */ }
}

async function finishLogin(res: any) {
  userStore.setToken(res.token)
  userStore.setUser(res.user)
  ElMessage.success('登录成功')
  router.push('/dashboard')
}

async function onLogin() {
  loading.value = true
  try {
    const res = await api.login(form)
    if (res.mfaRequired) {
      mfaForm.mfaToken = res.mfaToken
      mfaRequired.value = true
      return
    }
    await finishLogin(res)
  } catch (e) {
    refreshCaptcha()
  } finally {
    loading.value = false
  }
}

async function onMfaVerify() {
  loading.value = true
  try {
    const res = await api.mfaVerify({ mfaToken: mfaForm.mfaToken, code: mfaForm.code })
    await finishLogin(res)
  } catch (e) {
    mfaForm.code = ''
  } finally {
    loading.value = false
  }
}

function backToLogin() {
  mfaRequired.value = false
  mfaForm.code = ''
  refreshCaptcha()
}

onMounted(refreshCaptcha)
</script>

<style scoped>
.login-wrap { height: 100%; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #1f2d3d, #304ffe); }
.login-card { width: 380px; padding: 10px 24px 24px; }
.brand { text-align: center; font-size: 20px; font-weight: 700; margin: 16px 0 24px; color: #304ffe; }
.captcha-row { display: flex; gap: 8px; align-items: center; width: 100%; }
.captcha-box { width: 110px; height: 40px; line-height: 40px; text-align: center; background: #f5f7fa; border: 1px solid #dcdfe6; border-radius: 4px; letter-spacing: 4px; font-weight: 700; cursor: pointer; user-select: none; }
.tip { text-align: center; color: #909399; font-size: 12px; margin-top: 12px; }
</style>
