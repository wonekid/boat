<template>
  <div class="page">
    <div class="search-bar">
      <el-button type="primary" @click="openCreate">下发任务</el-button>
      <el-button @click="load" :loading="loading">刷新</el-button>
      <el-input v-model="keyword" placeholder="任务名称" clearable style="width:200px" @keyup.enter="load" />
      <el-select v-model="status" placeholder="状态" clearable style="width:140px" @change="load">
        <el-option label="执行中" value="running" />
        <el-option label="全部成功" value="success" />
        <el-option label="部分失败" value="partial" />
        <el-option label="全部失败" value="failed" />
        <el-option label="已取消" value="canceled" />
      </el-select>
      <el-switch v-model="autoRefresh" active-text="自动刷新" />
      <span class="tip">任务经 OSP 加密通道下发到执行机，由常驻的 osp-agent 执行并回传标准输出/错误与退出码</span>
    </div>

    <el-table :data="list" border stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="任务名称" min-width="160" />
      <el-table-column label="类型" width="120">
        <template #default="{ row }">
          <el-tag>{{ typeMap[row.type] || row.type }}</el-tag>
          <el-tag type="info" size="small" style="margin-left:4px">{{ langMap[row.lang] || row.lang }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="节点数" width="90">
        <template #default="{ row }">{{ nodeIdList(row).length }}</template>
      </el-table-column>
      <el-table-column label="进度" width="140">
        <template #default="{ row }">
          <el-progress :percentage="progressPct(row)" :stroke-width="12" :status="row.status === 'failed' ? 'exception' : undefined" />
          <div class="sub">{{ row.progress || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)">{{ statusMap[row.status] || row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="createdBy" label="下发人" width="110" />
      <el-table-column label="开始时间" width="170">
        <template #default="{ row }">{{ fmt(row.startedAt) }}</template>
      </el-table-column>
      <el-table-column label="结束时间" width="170">
        <template #default="{ row }">{{ fmt(row.finishedAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button size="small" type="primary" @click="openDetail(row)">结果</el-button>
          <el-button size="small" type="warning" :disabled="row.status !== 'running'" @click="cancelTask(row)">取消</el-button>
          <el-button size="small" type="success" :disabled="row.status === 'running'" @click="retryTask(row)">重试失败</el-button>
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

    <!-- 任务详情 + 逐节点结果 -->
    <el-drawer v-model="detailVisible" :title="`任务结果 - ${detail.task?.name || ''}`" size="68%">
      <el-descriptions :column="2" border size="small" style="margin-bottom:12px">
        <el-descriptions-item label="任务 ID">{{ detail.task?.id }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="statusType(detail.task?.status)">{{ statusMap[detail.task?.status] || detail.task?.status }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="类型">{{ typeMap[detail.task?.type] }} / {{ langMap[detail.task?.lang] }}</el-descriptions-item>
        <el-descriptions-item label="超时">{{ detail.task?.timeout }} 秒</el-descriptions-item>
        <el-descriptions-item label="执行用户">{{ detail.task?.runAsUser || 'Agent 默认用户' }}</el-descriptions-item>
        <el-descriptions-item label="下发人">{{ detail.task?.createdBy }}</el-descriptions-item>
        <el-descriptions-item label="进度">{{ detail.task?.progress }}</el-descriptions-item>
        <el-descriptions-item label="耗时">{{ durationText(detail.task) }}</el-descriptions-item>
      </el-descriptions>

      <el-tabs>
        <el-tab-pane label="执行结果">
          <el-table :data="detail.results" border size="small">
            <el-table-column prop="nodeName" label="节点" min-width="140" />
            <el-table-column prop="nodeIp" label="IP" width="130" />
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="resultType(row.status)">{{ resultMap[row.status] || row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="exitCode" label="退出码" width="90" />
            <el-table-column label="耗时" width="100">
              <template #default="{ row }">{{ row.duration ? row.duration + ' ms' : '-' }}</template>
            </el-table-column>
            <el-table-column prop="error" label="错误信息" min-width="160" show-overflow-tooltip />
            <el-table-column label="操作" width="120" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="showOutput(row)">查看输出</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
        <el-tab-pane label="任务内容">
          <pre class="code">{{ detail.task?.content }}</pre>
        </el-tab-pane>
      </el-tabs>
    </el-drawer>

    <!-- 输出详情 -->
    <el-dialog v-model="outputVisible" :title="`输出 - ${outputRow.nodeName || ''}`" width="820px">
      <el-descriptions :column="2" border size="small" style="margin-bottom:12px">
        <el-descriptions-item label="状态">
          <el-tag :type="resultType(outputRow.status)">{{ resultMap[outputRow.status] || outputRow.status }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="退出码">{{ outputRow.exitCode }}</el-descriptions-item>
        <el-descriptions-item label="耗时">{{ outputRow.duration }} ms</el-descriptions-item>
        <el-descriptions-item label="节点 IP">{{ outputRow.nodeIp || '-' }}</el-descriptions-item>
      </el-descriptions>
      <div class="out-title">标准输出</div>
      <pre class="code">{{ outputRow.stdout || '（空）' }}</pre>
      <div class="out-title error">标准错误</div>
      <pre class="code error-bg">{{ outputRow.stderr || '（空）' }}</pre>
      <div v-if="outputRow.error" class="out-title error">执行错误</div>
      <pre v-if="outputRow.error" class="code error-bg">{{ outputRow.error }}</pre>
      <template #footer>
        <el-button @click="outputVisible = false">关闭</el-button>
        <el-button type="primary" @click="copy(outputRow.stdout || '')">复制标准输出</el-button>
      </template>
    </el-dialog>

    <!-- 新建任务 -->
    <el-dialog v-model="visible" title="下发任务到执行机" width="780px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="任务名称"><el-input v-model="form.name" placeholder="留空自动生成" /></el-form-item>
        <el-form-item label="目标节点">
          <div class="node-pick">
            <div class="pick-actions">
              <el-button size="small" @click="pickOnline">全选在线节点</el-button>
              <el-button size="small" @click="form.nodeIds = []">清空</el-button>
              <el-input v-model="labelFilter" placeholder="按标签筛选，如 prod" size="small" style="width:160px" clearable />
            </div>
            <el-select v-model="form.nodeIds" multiple filterable style="width:100%">
              <el-option v-for="n in filteredNodes" :key="n.id" :label="`${n.name}(${n.ip || '未接入'})${n.status === 'online' ? '' : ' [离线]'}`" :value="n.id" />
            </el-select>
          </div>
        </el-form-item>
        <el-form-item label="类型">
          <el-radio-group v-model="form.type">
            <el-radio-button label="command">命令</el-radio-button>
            <el-radio-button label="script">脚本</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="语言">
          <el-select v-model="form.lang" style="width:180px">
            <el-option label="Shell" value="shell" />
            <el-option label="Python" value="python" />
            <el-option label="PowerShell" value="powershell" />
          </el-select>
        </el-form-item>
        <el-form-item label="脚本库" v-if="form.type === 'script'">
          <el-select v-model="form.scriptId" clearable filterable style="width:100%" placeholder="可选：从脚本库带入" @change="onScriptChange">
            <el-option v-for="s in scripts" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="form.type === 'script' ? '脚本内容' : '命令内容'">
          <el-input v-model="form.content" type="textarea" :rows="10" placeholder="如 chage -M 99999 opsuser" />
        </el-form-item>
        <el-form-item label="执行用户">
          <el-input v-model="form.runAsUser" placeholder="留空则用 Agent 运行用户（通常 root）" style="width:280px" />
        </el-form-item>
        <el-form-item label="超时(秒)">
          <el-input-number v-model="form.timeout" :min="5" :max="3600" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">下发执行</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/api'

const route = useRoute()
const list = ref<any[]>([])
const nodes = ref<any[]>([])
const scripts = ref<any[]>([])
const loading = ref(false)
const saving = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const keyword = ref('')
const status = ref('')
const autoRefresh = ref(true)
const labelFilter = ref('')

const visible = ref(false)
const detailVisible = ref(false)
const outputVisible = ref(false)
const detail = reactive<any>({ task: null, results: [] })
const outputRow = ref<any>({})
const form = reactive<any>({
  name: '', nodeIds: [] as number[], type: 'command', lang: 'shell',
  scriptId: null, content: '', runAsUser: '', timeout: 120,
})

const typeMap: any = { command: '命令', script: '脚本' }
const langMap: any = { shell: 'Shell', python: 'Python', powershell: 'PowerShell', batch: 'Batch' }
const statusMap: any = { running: '执行中', success: '全部成功', partial: '部分失败', failed: '全部失败', canceled: '已取消' }
const resultMap: any = {
  pending: '待执行', running: '执行中', success: '成功', failed: '失败',
  timeout: '超时', offline: '节点离线', canceled: '已取消',
}

let timer: any = null

onMounted(async () => {
  await Promise.all([load(), loadNodes(), loadScripts()])
  const id = String(route.query.id || '')
  if (id) {
    // 从执行机节点页跳转而来，自动展开该任务结果
    setTimeout(() => openDetail({ id: Number(id) }), 300)
  }
  timer = setInterval(() => {
    if (!autoRefresh.value) return
    if (list.value.some((r) => r.status === 'running')) {
      load()
    }
    if (detailVisible.value && detail.task?.status === 'running') {
      loadDetail(detail.task.id)
    }
  }, 3000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

watch(() => route.query.id, async (v) => {
  if (v) await openDetail({ id: Number(v) })
})

const filteredNodes = computed(() => {
  const kw = labelFilter.value.trim().toLowerCase()
  if (!kw) return nodes.value
  return nodes.value.filter((n) => (n.labels || '').toLowerCase().includes(kw))
})

async function load() {
  loading.value = true
  try {
    const res = await api.listAgentTasks({ page: page.value, pageSize: pageSize.value, keyword: keyword.value, status: status.value })
    list.value = res.list || []
    total.value = res.total || 0
  } finally {
    loading.value = false
  }
}

async function loadNodes() {
  try {
    const res = await api.listAgentNodes({ page: 1, pageSize: 500 })
    nodes.value = res.list || []
  } catch (e) { /* ignore */ }
}

async function loadScripts() {
  try {
    scripts.value = await api.agentScripts() || []
  } catch (e) { /* ignore */ }
}

function nodeIdList(row: any): number[] {
  try { return JSON.parse(row.nodeIds || '[]') } catch (e) { return [] }
}

function progressPct(row: any): number {
  const p = String(row.progress || '0/0')
  const [done, all] = p.split('/').map((v) => Number(v))
  if (!all) return 0
  return Math.round((done / all) * 100)
}

function statusType(s: string): string {
  const m: any = { running: 'warning', success: 'success', partial: 'warning', failed: 'danger', canceled: 'info' }
  return m[s] || 'info'
}

function resultType(s: string): string {
  const m: any = { success: 'success', failed: 'danger', timeout: 'warning', offline: 'info', canceled: 'info', running: 'warning', pending: 'info' }
  return m[s] || 'info'
}

function fmt(t: string): string {
  if (!t) return '-'
  return String(t).replace('T', ' ').slice(0, 19)
}

function durationText(task: any): string {
  if (!task?.startedAt) return '-'
  const start = new Date(String(task.startedAt).replace(' ', 'T')).getTime()
  const end = task.finishedAt ? new Date(String(task.finishedAt).replace(' ', 'T')).getTime() : Date.now()
  const ms = end - start
  if (ms < 1000) return ms + ' ms'
  if (ms < 60000) return (ms / 1000).toFixed(1) + ' s'
  return Math.floor(ms / 60000) + ' 分 ' + Math.round((ms % 60000) / 1000) + ' 秒'
}

function openCreate() {
  Object.assign(form, {
    name: '', nodeIds: [], type: 'command', lang: 'shell',
    scriptId: null, content: '', runAsUser: '', timeout: 120,
  })
  visible.value = true
}

function pickOnline() {
  form.nodeIds = nodes.value.filter((n) => n.status === 'online').map((n) => n.id)
}

function onScriptChange(id: number) {
  const s = scripts.value.find((x) => x.id === id)
  if (s) {
    form.content = s.content || ''
    if (s.lang) form.lang = s.lang
  }
}

async function save() {
  if (!form.nodeIds.length) {
    ElMessage.warning('请选择目标节点')
    return
  }
  if (!form.content.trim() && !form.scriptId) {
    ElMessage.warning('请填写任务内容或选择脚本')
    return
  }
  saving.value = true
  try {
    const r = await api.createAgentTask({ ...form })
    ElMessage.success('任务已下发')
    visible.value = false
    await load()
    if (r?.taskId) openDetail({ id: r.taskId })
  } finally {
    saving.value = false
  }
}

async function openDetail(row: any) {
  await loadDetail(row.id)
  detailVisible.value = true
}

async function loadDetail(id: number) {
  try {
    const r = await api.getAgentTask(id)
    detail.task = r.task
    detail.results = r.results || []
  } catch (e) { /* 错误已统一提示 */ }
}

function showOutput(row: any) {
  outputRow.value = row
  outputVisible.value = true
}

async function cancelTask(row: any) {
  await ElMessageBox.confirm('确认取消该任务？在线节点会收到取消指令，未完成的节点将标记为已取消', '取消任务', { type: 'warning' })
  await api.cancelAgentTask(row.id)
  ElMessage.success('已下发取消指令')
  await load()
  if (detail.task?.id === row.id) await loadDetail(row.id)
}

async function retryTask(row: any) {
  try {
    const r = await api.retryAgentTask(row.id)
    ElMessage.success(`已重试 ${r.count || 0} 个节点`)
    await load()
    await loadDetail(row.id)
  } catch (e) { /* 错误已统一提示 */ }
}

function copy(text: string) {
  if (!text) {
    ElMessage.warning('暂无内容')
    return
  }
  navigator.clipboard?.writeText(text).then(
    () => ElMessage.success('已复制'),
    () => ElMessage.warning('复制失败，请手动选择')
  )
}
</script>

<style scoped>
.page { padding: 2px; }
.search-bar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
.tip { color: #909399; font-size: 12px; }
.sub { color: #909399; font-size: 12px; }
.pager { margin-top: 12px; justify-content: flex-end; }
.code { background: #1f2d3d; color: #d7e3f4; padding: 12px; border-radius: 4px; max-height: 300px; overflow: auto; font-size: 12px; line-height: 1.6; white-space: pre-wrap; }
.error-bg { background: #2b1f1f; color: #f7c9c9; }
.out-title { font-weight: 600; margin: 10px 0 6px; }
.out-title.error { color: #f56c6c; }
.node-pick { width: 100%; }
.pick-actions { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }
</style>
