<template>
  <div class="page">
    <div class="stat-row">
      <el-card shadow="never" class="stat">
        <div class="stat-value online">{{ overview.online || 0 }}</div>
        <div class="stat-label">在线节点</div>
      </el-card>
      <el-card shadow="never" class="stat">
        <div class="stat-value offline">{{ overview.offline || 0 }}</div>
        <div class="stat-label">离线节点</div>
      </el-card>
      <el-card shadow="never" class="stat">
        <div class="stat-value">{{ overview.runningTask || 0 }}</div>
        <div class="stat-label">执行中任务</div>
      </el-card>
      <el-card shadow="never" class="stat">
        <div class="stat-value small">
          <el-tag :type="overview.serving ? 'success' : 'danger'" size="small">
            {{ overview.serving ? '监听中' : '未启动' }}
          </el-tag>
          <span class="port">:{{ overview.port || '-' }}</span>
        </div>
        <div class="stat-label">OSP 服务（心跳 {{ overview.heartbeat || '-' }}s）</div>
      </el-card>
    </div>

    <div class="search-bar">
      <el-button type="primary" @click="openCreate">注册节点</el-button>
      <el-button @click="load" :loading="loading">刷新</el-button>
      <el-input v-model="keyword" placeholder="名称 / IP / 主机名" clearable style="width:200px" @keyup.enter="load" />
      <el-select v-model="status" placeholder="状态" clearable style="width:120px" @change="load">
        <el-option label="在线" value="online" />
        <el-option label="离线" value="offline" />
      </el-select>
      <el-switch v-model="autoRefresh" active-text="自动刷新" />
      <el-tag v-if="wsOk" type="success" size="small">实时通道已连接</el-tag>
      <el-tag v-else type="info" size="small">实时通道未连接（轮询兜底）</el-tag>
      <span class="tip">
        执行机上的 <code>osp-agent</code> 主动外连控制台 OSP 端口，无需开放入站端口；
        SSH 无法登录（密码过期 / 账户锁定 / sshd 异常）时仍可下发命令与脚本。
      </span>
    </div>

    <el-table :data="list" border stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="节点" min-width="180">
        <template #default="{ row }">
          <div class="node-name">{{ row.name }}</div>
          <div class="sub">{{ row.hostname || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'online' ? 'success' : 'info'" effect="dark">
            {{ row.status === 'online' ? '在线' : '离线' }}
          </el-tag>
          <div v-if="row.enabled === 0" class="sub"><el-tag type="danger" size="small">已禁用</el-tag></div>
        </template>
      </el-table-column>
      <el-table-column prop="ip" label="IP" width="130" />
      <el-table-column label="系统" width="120">
        <template #default="{ row }">
          <span>{{ row.os || '-' }}</span>
          <span class="sub"> / {{ row.arch || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="CPU" width="120">
        <template #default="{ row }">
          <el-progress :percentage="pct(row.cpuUsage)" :stroke-width="10" :color="colorOf(row.cpuUsage)" />
        </template>
      </el-table-column>
      <el-table-column label="内存" width="120">
        <template #default="{ row }">
          <el-progress :percentage="pct(row.memUsage)" :stroke-width="10" :color="colorOf(row.memUsage)" />
          <div class="sub" v-if="row.memTotal">{{ row.memUsed }}/{{ row.memTotal }} MB</div>
        </template>
      </el-table-column>
      <el-table-column label="磁盘" width="120">
        <template #default="{ row }">
          <el-progress :percentage="pct(row.diskUsage)" :stroke-width="10" :color="colorOf(row.diskUsage)" />
          <div class="sub" v-if="row.diskTotal">{{ row.diskUsed }}/{{ row.diskTotal }} GB</div>
        </template>
      </el-table-column>
      <el-table-column label="负载 / 运行" width="130">
        <template #default="{ row }">
          <div>{{ row.loadAvg || '-' }}</div>
          <div class="sub">{{ uptimeText(row.uptime) }}</div>
        </template>
      </el-table-column>
      <el-table-column label="标签" min-width="120">
        <template #default="{ row }">
          <el-tag v-for="t in labelList(row.labels)" :key="t" size="small" type="warning" style="margin:0 4px 4px 0">{{ t }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="version" label="Agent版本" width="100" />
      <el-table-column label="最后心跳" width="170">
        <template #default="{ row }">
          <span :class="{ stale: isStale(row) }">{{ fmt(row.lastSeenAt) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="330" fixed="right">
        <template #default="{ row }">
          <el-button size="small" type="primary" @click="openTask(row, 'command')">下发命令</el-button>
          <el-button size="small" type="success" @click="openTask(row, 'script')">下发脚本</el-button>
          <el-button size="small" @click="openInstall(row)">接入指引</el-button>
          <el-dropdown @command="(c: string) => onMore(c, row)">
            <el-button size="small">更多</el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="edit">编辑</el-dropdown-item>
                <el-dropdown-item command="token">重置令牌</el-dropdown-item>
                <el-dropdown-item command="disconnect">断开连接</el-dropdown-item>
                <el-dropdown-item :command="row.enabled === 1 ? 'disable' : 'enable'">
                  {{ row.enabled === 1 ? '禁用节点' : '启用节点' }}
                </el-dropdown-item>
                <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      class="pager"
      v-model:current-page="page"
      v-model:page-size="pageSize"
      :total="total"
      :page-sizes="[10, 20, 50, 100]"
      layout="total, sizes, prev, pager, next"
      @change="load"
    />

    <!-- 注册 / 编辑节点 -->
    <el-dialog v-model="visible" :title="form.id ? '编辑节点' : '注册执行机节点'" width="520px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="节点名称"><el-input v-model="form.name" placeholder="如 web-prod-01" /></el-form-item>
        <el-form-item label="主机名"><el-input v-model="form.hostname" placeholder="Agent 接入后自动上报，可留空" /></el-form-item>
        <el-form-item label="IP"><el-input v-model="form.ip" placeholder="Agent 接入后自动上报，可留空" /></el-form-item>
        <el-form-item label="标签">
          <el-input v-model="form.labels" placeholder="逗号分隔，如 prod,web" />
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- 令牌展示 -->
    <el-dialog v-model="tokenVisible" title="节点接入令牌" width="560px">
      <el-alert type="warning" :closable="false" title="令牌仅在此处完整展示一次，请立即复制并妥善保存" style="margin-bottom:12px" />
      <el-input v-model="tokenText" readonly />
      <template #footer>
        <el-button @click="tokenVisible = false">关闭</el-button>
        <el-button type="primary" @click="copy(tokenText)">复制令牌</el-button>
      </template>
    </el-dialog>

    <!-- 接入指引 -->
    <el-dialog v-model="installVisible" :title="`接入指引 - ${installData.nodeName || ''}`" width="760px">
      <el-descriptions :column="1" border size="small" style="margin-bottom:12px">
        <el-descriptions-item label="控制台地址">{{ installData.serverAddr }}</el-descriptions-item>
        <el-descriptions-item label="接入令牌">
          <code>{{ installData.token }}</code>
          <el-button size="small" link type="primary" @click="copy(installData.token)">复制</el-button>
        </el-descriptions-item>
      </el-descriptions>
      <el-alert type="info" :closable="false" show-icon
        description="在执行机上以 root 执行以下脚本即可完成部署：写入配置 → 下载 osp-agent → 注册为 systemd 服务并设置开机自启。若无法直接下载，请手动放置二进制后重跑脚本。" />
      <pre class="code">{{ installData.script }}</pre>
      <template #footer>
        <el-button @click="installVisible = false">关闭</el-button>
        <el-button type="primary" @click="copy(installData.script)">复制安装脚本</el-button>
        <el-button @click="copy(installData.publicKey || '')">复制服务端公钥</el-button>
      </template>
    </el-dialog>

    <!-- 下发任务 -->
    <el-dialog v-model="taskVisible" :title="taskForm.type === 'script' ? '下发脚本任务' : '下发命令任务'" width="760px">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom:12px"
        description="任务以 Agent 自身运行用户（通常为 root）执行；即便执行机 SSH 无法登录，只要 Agent 在线即可执行，常用于应急修复。" />
      <el-form :model="taskForm" label-width="100px">
        <el-form-item label="任务名称">
          <el-input v-model="taskForm.name" placeholder="留空自动生成" />
        </el-form-item>
        <el-form-item label="目标节点">
          <el-select v-model="taskForm.nodeIds" multiple filterable style="width:100%">
            <el-option v-for="n in allNodes" :key="n.id" :label="`${n.name}(${n.ip || '未接入'})`" :value="n.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="类型">
          <el-radio-group v-model="taskForm.type">
            <el-radio-button label="command">命令</el-radio-button>
            <el-radio-button label="script">脚本</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="语言">
          <el-select v-model="taskForm.lang" style="width:180px">
            <el-option label="Shell" value="shell" />
            <el-option label="Python" value="python" />
            <el-option label="PowerShell" value="powershell" />
          </el-select>
        </el-form-item>
        <el-form-item label="脚本库" v-if="taskForm.type === 'script'">
          <el-select v-model="taskForm.scriptId" clearable filterable style="width:100%" placeholder="可选：从脚本库带入">
            <el-option v-for="s in scripts" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="应急工具箱">
          <div class="rescue">
            <div class="rescue-params">
              <el-input v-model="rescueUser" placeholder="用户名" size="small" style="width:120px" />
              <el-input v-model="rescuePassword" placeholder="新密码" size="small" style="width:140px" />
            </div>
            <div class="rescue-list">
              <el-button v-for="r in rescueCmds" :key="r.name" size="small" :title="r.desc" @click="applyRescue(r)">
                {{ r.name }}
              </el-button>
            </div>
          </div>
          <div class="tip">点击后自动填充命令，{user} / {password} 会按上方输入替换</div>
        </el-form-item>
        <el-form-item :label="taskForm.type === 'script' ? '脚本内容' : '命令内容'">
          <el-input v-model="taskForm.content" type="textarea" :rows="8" placeholder="如 chage -M 99999 opsuser" />
        </el-form-item>
        <el-form-item label="执行用户">
          <el-input v-model="taskForm.runAsUser" placeholder="留空则用 Agent 运行用户（通常 root）" style="width:260px" />
        </el-form-item>
        <el-form-item label="超时(秒)">
          <el-input-number v-model="taskForm.timeout" :min="5" :max="3600" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="taskVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitTask">下发执行</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/api'

const router = useRouter()
const list = ref<any[]>([])
const allNodes = ref<any[]>([])
const scripts = ref<any[]>([])
const overview = ref<any>({})
const loading = ref(false)
const saving = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const keyword = ref('')
const status = ref('')
const autoRefresh = ref(true)
const wsOk = ref(false)

const visible = ref(false)
const tokenVisible = ref(false)
const tokenText = ref('')
const installVisible = ref(false)
const installData = reactive<any>({ nodeName: '', token: '', serverAddr: '', script: '', publicKey: '' })
const taskVisible = ref(false)
const rescueUser = ref('opsuser')
const rescuePassword = ref('')
const form = reactive<any>({ id: null, name: '', hostname: '', ip: '', labels: '', remark: '' })
const taskForm = reactive<any>({
  name: '', nodeIds: [] as number[], type: 'command', lang: 'shell',
  scriptId: null, content: '', runAsUser: '', timeout: 120,
})

// 应急工具箱：SSH 登录不进去时的高频修复动作
const rescueCmds = [
  { name: '查看密码过期状态', cmd: 'chage -l {user}', desc: '查看密码有效期与账户过期时间' },
  { name: '取消密码过期', cmd: 'chage -M 99999 {user} && chage -l {user}', desc: '将密码有效期设为永不过期，解除登录拦截' },
  { name: '重置账户密码', cmd: "echo '{user}:{password}' | chpasswd && echo password-reset-ok", desc: '无需原密码直接重置（Agent 以 root 运行时）' },
  { name: '解锁被锁定账户', cmd: 'pam_tally2 --reset -u {user} 2>/dev/null; faillog -r -u {user} 2>/dev/null; passwd -u {user} 2>/dev/null; usermod -U {user} 2>/dev/null; passwd -S {user}', desc: '清除失败登录计数并解锁账户' },
  { name: '检查账户锁定状态', cmd: 'id {user}; passwd -S {user}; pam_tally2 -u {user} 2>/dev/null; faillog -u {user} 2>/dev/null', desc: '确认账户是否存在及锁定原因' },
  { name: '恢复 SSH 登录', cmd: 'sed -i "s/^DenyUsers/#DenyUsers/" /etc/ssh/sshd_config; sshd -t && (systemctl restart sshd || service sshd restart) && echo sshd-restarted', desc: '清理 DenyUsers 并重启 sshd' },
  { name: '校验 sudoers', cmd: 'visudo -c', desc: 'sudoers 改错导致无法提权时先校验语法' },
  { name: '修复 sudoers 权限', cmd: 'chmod 440 /etc/sudoers && chown root:root /etc/sudoers && visudo -c', desc: '恢复 sudoers 文件权限后校验' },
  { name: '放行 SSH 端口', cmd: '(iptables -C INPUT -p tcp --dport 22 -j ACCEPT 2>/dev/null || iptables -I INPUT -p tcp --dport 22 -j ACCEPT) && echo ssh-port-opened', desc: '确认防火墙未拦截 22 端口' },
  { name: '查看登录失败日志', cmd: 'lastb -n 20 2>/dev/null; journalctl -u sshd -n 30 --no-pager 2>/dev/null | tail -30', desc: '排查登录失败具体原因' },
  { name: '创建应急运维账号', cmd: 'id rescueops 2>/dev/null || useradd rescueops; echo "rescueops:{password}" | chpasswd; (usermod -aG wheel rescueops 2>/dev/null || usermod -aG sudo rescueops 2>/dev/null); echo rescue-account-ready', desc: '创建带 sudo 权限的应急账号以便重新登录' },
]

let timer: any = null
let ws: WebSocket | null = null

onMounted(async () => {
  await Promise.all([load(), loadOverview(), loadNodes(), loadScripts()])
  connectWS()
  timer = setInterval(() => {
    if (!autoRefresh.value) return
    // 实时通道不可用时以轮询兜底
    if (!wsOk.value) {
      load()
      loadOverview()
    }
  }, 5000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
  if (ws) {
    ws.onclose = null
    ws.close()
  }
})

async function load() {
  loading.value = true
  try {
    const res = await api.listAgentNodes({ page: page.value, pageSize: pageSize.value, keyword: keyword.value, status: status.value })
    list.value = res.list || []
    total.value = res.total || 0
  } finally {
    loading.value = false
  }
}

async function loadOverview() {
  try {
    overview.value = await api.agentOverview()
  } catch (e) { /* 服务未启动时忽略 */ }
}

async function loadNodes() {
  try {
    const res = await api.listAgentNodes({ page: 1, pageSize: 500 })
    allNodes.value = res.list || []
  } catch (e) { /* ignore */ }
}

async function loadScripts() {
  try {
    scripts.value = await api.agentScripts() || []
  } catch (e) { /* ignore */ }
}

function connectWS() {
  const token = localStorage.getItem('boat_token')
  if (!token) return
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  let sock: WebSocket
  try {
    sock = new WebSocket(`${proto}://${location.host}/api/ws/agent?token=${encodeURIComponent(token)}`)
  } catch (e) {
    return
  }
  ws = sock
  sock.onopen = () => { wsOk.value = true }
  sock.onerror = () => { wsOk.value = false }
  sock.onclose = () => {
    wsOk.value = false
    if (autoRefresh.value) setTimeout(connectWS, 5000)
  }
  sock.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data)
      if (msg.type === 'snapshot' && Array.isArray(msg.nodes)) {
        mergeNodes(msg.nodes)
      } else if (msg.node) {
        mergeNodes([msg.node])
      }
      if (msg.type === 'node_online' || msg.type === 'node_offline') {
        loadOverview()
      }
    } catch (e) { /* 忽略非法消息 */ }
  }
}

// mergeNodes 用实时数据覆盖列表中的同 ID 节点（不改变分页顺序）
function mergeNodes(nodes: any[]) {
  const map = new Map<number, any>()
  nodes.forEach((n) => map.set(n.id, n))
  list.value = list.value.map((item) => {
    const fresh = map.get(item.id)
    return fresh ? { ...item, ...fresh } : item
  })
  allNodes.value = allNodes.value.map((item) => {
    const fresh = map.get(item.id)
    return fresh ? { ...item, ...fresh } : item
  })
}

function pct(v: any): number {
  const n = Number(v || 0)
  if (!isFinite(n)) return 0
  return Math.round(Math.min(100, Math.max(0, n)))
}

function colorOf(v: any): string {
  const n = Number(v || 0)
  if (n >= 90) return '#f56c6c'
  if (n >= 75) return '#e6a23c'
  return '#67c23a'
}

function labelList(labels: string): string[] {
  if (!labels) return []
  return labels.split(',').map((s) => s.trim()).filter(Boolean)
}

function uptimeText(sec: any): string {
  const s = Number(sec || 0)
  if (s <= 0) return '-'
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (d > 0) return `运行 ${d}天${h}小时`
  if (h > 0) return `运行 ${h}小时${m}分`
  return `运行 ${m}分钟`
}

function fmt(t: string): string {
  if (!t) return '-'
  return String(t).replace('T', ' ').slice(0, 19)
}

function isStale(row: any): boolean {
  if (row.status !== 'online' || !row.lastSeenAt) return false
  const last = new Date(String(row.lastSeenAt).replace(' ', 'T')).getTime()
  return Date.now() - last > 60000
}

function openCreate() {
  Object.assign(form, { id: null, name: '', hostname: '', ip: '', labels: '', remark: '' })
  visible.value = true
}

async function save() {
  saving.value = true
  try {
    if (form.id) {
      await api.updateAgentNode(form.id, form)
      ElMessage.success('更新成功')
      visible.value = false
    } else {
      const r = await api.createAgentNode(form)
      ElMessage.success('节点已注册')
      visible.value = false
      tokenText.value = r.token || ''
      tokenVisible.value = true
    }
    await Promise.all([load(), loadNodes(), loadOverview()])
  } finally {
    saving.value = false
  }
}

function openEdit(row: any) {
  Object.assign(form, { id: row.id, name: row.name, hostname: row.hostname, ip: row.ip, labels: row.labels, remark: row.remark })
  visible.value = true
}

async function openInstall(row: any) {
  try {
    const r = await api.agentInstall(row.id)
    Object.assign(installData, r)
    installVisible.value = true
  } catch (e) { /* 错误已统一提示 */ }
}

async function onMore(cmd: string, row: any) {
  if (cmd === 'edit') return openEdit(row)
  if (cmd === 'token') {
    await ElMessageBox.confirm('重置后旧 Agent 需更换令牌才能重新接入，确认继续？', '重置令牌', { type: 'warning' })
    const r = await api.resetAgentToken(row.id)
    tokenText.value = r.token || ''
    tokenVisible.value = true
    await load()
    return
  }
  if (cmd === 'disconnect') {
    await api.disconnectAgent(row.id)
    ElMessage.success('已断开连接')
    await load()
    return
  }
  if (cmd === 'enable' || cmd === 'disable') {
    await api.updateAgentNode(row.id, { enabled: cmd === 'enable' ? 1 : 0 })
    ElMessage.success(cmd === 'enable' ? '已启用' : '已禁用')
    await load()
    return
  }
  if (cmd === 'delete') {
    await ElMessageBox.confirm(`确认删除节点 ${row.name}？其任务结果记录将一并删除`, '删除节点', { type: 'warning' })
    await api.deleteAgentNode(row.id)
    ElMessage.success('已删除')
    await Promise.all([load(), loadNodes(), loadOverview()])
  }
}

function openTask(row: any, type: string) {
  taskForm.name = ''
  taskForm.type = type
  taskForm.lang = 'shell'
  taskForm.content = ''
  taskForm.scriptId = null
  taskForm.runAsUser = ''
  taskForm.timeout = 120
  taskForm.nodeIds = [row.id]
  taskVisible.value = true
}

function applyRescue(r: any) {
  taskForm.type = 'command'
  taskForm.lang = 'shell'
  taskForm.content = r.cmd
    .replace(/\{user\}/g, rescueUser.value || 'opsuser')
    .replace(/\{password\}/g, rescuePassword.value || 'ChangeMe@123')
  if (!taskForm.name) taskForm.name = r.name
}

async function submitTask() {
  if (!taskForm.nodeIds.length) {
    ElMessage.warning('请选择目标节点')
    return
  }
  if (!taskForm.content.trim() && !taskForm.scriptId) {
    ElMessage.warning('请填写任务内容或选择脚本')
    return
  }
  saving.value = true
  try {
    const r = await api.createAgentTask({ ...taskForm })
    ElMessage.success('任务已下发')
    taskVisible.value = false
    router.push({ name: 'AgentTask', query: { id: String(r.taskId || '') } })
  } finally {
    saving.value = false
  }
}

function copy(text: string) {
  if (!text) {
    ElMessage.warning('暂无内容可复制')
    return
  }
  navigator.clipboard?.writeText(text).then(
    () => ElMessage.success('已复制'),
    () => ElMessage.warning('复制失败，请手动选择文本')
  )
}
</script>

<style scoped>
.page { padding: 2px; }
.stat-row { display: flex; gap: 12px; margin-bottom: 12px; flex-wrap: wrap; }
.stat { min-width: 180px; flex: 1; }
.stat-value { font-size: 26px; font-weight: 700; color: #303133; }
.stat-value.small { font-size: 18px; }
.stat-value.online { color: #67c23a; }
.stat-value.offline { color: #909399; }
.stat-label { color: #909399; font-size: 12px; margin-top: 4px; }
.port { margin-left: 6px; color: #606266; font-size: 14px; }
.search-bar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
.tip { color: #909399; font-size: 12px; }
.node-name { font-weight: 600; }
.sub { color: #909399; font-size: 12px; }
.stale { color: #e6a23c; }
.pager { margin-top: 12px; justify-content: flex-end; }
.code { background: #1f2d3d; color: #d7e3f4; padding: 12px; border-radius: 4px; max-height: 320px; overflow: auto; font-size: 12px; line-height: 1.6; white-space: pre-wrap; }
.rescue { display: flex; flex-direction: column; gap: 8px; width: 100%; }
.rescue-params { display: flex; gap: 8px; }
.rescue-list { display: flex; flex-wrap: wrap; gap: 6px; }
</style>
