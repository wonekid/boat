<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="logo">Boat 运维平台</div>
      <el-menu :default-active="activeMenu" router class="menu" background-color="#1f2d3d" text-color="#bfcbd9" active-text-color="#409eff">
        <template v-for="m in menus" :key="m.path">
          <el-sub-menu v-if="m.children" :index="m.path">
            <template #title>
              <el-icon><component :is="m.icon" /></el-icon>
              <span>{{ m.title }}</span>
            </template>
            <el-menu-item v-for="c in m.children" :key="c.path" :index="'/' + c.path">
              <el-icon v-if="c.icon"><component :is="c.icon" /></el-icon>
              <span>{{ c.title }}</span>
            </el-menu-item>
          </el-sub-menu>
          <el-menu-item v-else :index="'/' + m.path">
            <el-icon v-if="m.icon"><component :is="m.icon" /></el-icon>
            <span>{{ m.title }}</span>
          </el-menu-item>
        </template>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="title">{{ currentTitle }}</div>
        <div class="right">
          <el-dropdown @command="onCommand">
            <span class="user">{{ userStore.user?.nickname || userStore.user?.username || '未登录' }}</span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="security">安全设置</el-dropdown-item>
                <el-dropdown-item command="pwd">修改密码</el-dropdown-item>
                <el-dropdown-item command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>

    <el-dialog v-model="pwdVisible" title="修改密码" width="400px">
      <el-form :model="pwdForm" label-width="90px">
        <el-form-item label="原密码"><el-input v-model="pwdForm.oldPassword" type="password" /></el-form-item>
        <el-form-item label="新密码"><el-input v-model="pwdForm.newPassword" type="password" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="pwdVisible = false">取消</el-button>
        <el-button type="primary" @click="submitPwd">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="secVisible" title="安全设置" width="460px">
      <el-divider content-position="left">两步验证（MFA / TOTP）</el-divider>
      <template v-if="!mfaEnabled">
        <el-alert type="info" :closable="false" title="尚未启用两步验证" description="启用后登录需输入动态码，大幅提升账号安全性" style="margin-bottom:12px" />
        <el-button type="primary" :loading="mfaBusy" @click="setupMfa">生成密钥并启用</el-button>
        <template v-if="mfaOtpauth">
          <div class="qr-wrap">
            <img :src="mfaQr" alt="MFA QR" class="qr" />
          </div>
          <p class="mfa-secret">密钥(手动输入): <code>{{ mfaSecret }}</code></p>
          <el-form :model="mfaForm" label-width="0" @keyup.enter="enableMfa">
            <el-form-item>
              <el-input v-model="mfaForm.code" placeholder="输入认证器中的 6 位动态码" maxlength="6" />
            </el-form-item>
            <el-button type="success" :loading="mfaBusy" @click="enableMfa">确认启用</el-button>
          </el-form>
        </template>
      </template>
      <template v-else>
        <el-alert type="success" :closable="false" title="两步验证已启用" style="margin-bottom:12px" />
        <el-form :model="mfaForm" label-width="0" @keyup.enter="disableMfa">
          <el-form-item>
            <el-input v-model="mfaForm.code" placeholder="输入动态码以关闭两步验证" maxlength="6" />
          </el-form-item>
          <el-button type="danger" :loading="mfaBusy" @click="disableMfa">关闭两步验证</el-button>
        </el-form>
      </template>
    </el-dialog>
  </el-container>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import QRCode from 'qrcode'
import { useUserStore } from '@/store/user'
import api from '@/api'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const mfaEnabled = computed(() => !!userStore.user?.mfaEnabled)

const menus = [
  { title: '仪表盘', path: 'dashboard', icon: 'Odometer' },
  { title: '系统管理', path: 'system', icon: 'Setting', children: [
    { title: '用户管理', path: 'system/user', icon: 'User' },
    { title: '角色管理', path: 'system/role', icon: 'Avatar' },
    { title: '菜单管理', path: 'system/menu', icon: 'Menu' },
    { title: '部门管理', path: 'system/dept', icon: 'OfficeBuilding' },
  ]},
  { title: '资产管理', path: 'asset', icon: 'Monitor', children: [
    { title: '主机管理', path: 'asset/host', icon: 'Cpu' },
    { title: '凭证管理', path: 'asset/credential', icon: 'Key' },
    { title: '主机分组', path: 'asset/group', icon: 'Folder' },
    { title: '授权管理', path: 'asset/auth', icon: 'Connection' },
  ]},
  { title: '运维中心', path: 'ops', icon: 'Terminal', children: [
    { title: 'Web终端', path: 'ops/terminal', icon: 'VideoPlay' },
    { title: '会话审计', path: 'ops/session', icon: 'Film' },
    { title: '操作日志', path: 'ops/audit', icon: 'Document' },
  ]},
  { title: '安全中心', path: 'security', icon: 'Lock', children: [
    { title: '高危指令', path: 'security/risk', icon: 'Warning' },
  ]},
  { title: '任务编排', path: 'task', icon: 'Calendar', children: [
    { title: '脚本库', path: 'task/script', icon: 'Notebook' },
    { title: '任务模板', path: 'task/template', icon: 'Files' },
    { title: '任务执行', path: 'task/execution', icon: 'Promotion' },
    { title: '定时任务', path: 'task/schedule', icon: 'Timer' },
    { title: '操作审批', path: 'task/approval', icon: 'Stamp' },
  ]},
]

const activeMenu = computed(() => route.path)
const currentTitle = computed(() => (route.meta.title as string) || '')

const pwdVisible = ref(false)
const pwdForm = reactive({ oldPassword: '', newPassword: '' })

// 安全设置 / MFA
const secVisible = ref(false)
const mfaBusy = ref(false)
const mfaOtpauth = ref('')
const mfaQr = ref('')
const mfaSecret = ref('')
const mfaForm = reactive({ code: '' })

onMounted(async () => {
  try {
    const u = await api.profile()
    userStore.setUser(u)
  } catch (e) {
    /* 忽略，路由守卫已处理 */
  }
})

function onCommand(cmd: string) {
  if (cmd === 'logout') {
    userStore.logout()
    router.push('/login')
  } else if (cmd === 'pwd') {
    pwdVisible.value = true
  } else if (cmd === 'security') {
    mfaForm.code = ''
    mfaOtpauth.value = ''
    secVisible.value = true
  }
}

async function submitPwd() {
  await api.changePwd(pwdForm)
  ElMessage.success('修改成功')
  pwdVisible.value = false
}

async function setupMfa() {
  mfaBusy.value = true
  try {
    const r = await api.mfaSetup()
    mfaSecret.value = r.secret
    mfaOtpauth.value = r.otpauth
    mfaQr.value = await QRCode.toDataURL(r.otpauth)
  } catch (e) { /* ignore */ }
  finally {
    mfaBusy.value = false
  }
}

async function enableMfa() {
  mfaBusy.value = true
  try {
    await api.mfaEnable({ code: mfaForm.code })
    ElMessage.success('MFA 已启用')
    const u = await api.profile()
    userStore.setUser(u)
    mfaForm.code = ''
    mfaOtpauth.value = ''
  } catch (e) { /* ignore */ }
  finally {
    mfaBusy.value = false
  }
}

async function disableMfa() {
  mfaBusy.value = true
  try {
    await api.mfaDisable({ code: mfaForm.code })
    ElMessage.success('MFA 已关闭')
    const u = await api.profile()
    userStore.setUser(u)
    mfaForm.code = ''
  } catch (e) { /* ignore */ }
  finally {
    mfaBusy.value = false
  }
}
</script>

<style scoped>
.layout { height: 100%; }
.aside { background: #1f2d3d; }
.logo { height: 56px; line-height: 56px; text-align: center; color: #fff; font-weight: 600; background: #18222c; }
.menu { border-right: none; height: calc(100% - 56px); }
.header { background: #fff; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #ebeef5; }
.title { font-size: 16px; font-weight: 600; }
.user { cursor: pointer; }
.main { background: #f0f2f5; }
.qr-wrap { text-align: center; margin: 12px 0; }
.qr { width: 200px; height: 200px; background: #fff; border: 1px solid #dcdfe6; padding: 6px; border-radius: 4px; }
.mfa-secret { font-size: 12px; color: #606266; word-break: break-all; margin-bottom: 12px; }
</style>
