<template>
  <div class="page">
    <div class="search-bar">
      <el-button type="success" @click="openExec">快速执行</el-button>
    </div>
    <el-table :data="list" border stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="名称" />
      <el-table-column label="类型" width="90">
        <template #default="{ row }"><el-tag>{{ typeMap[row.type] }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="createdBy" label="执行人" width="110" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="createdAt" label="开始时间" width="180" />
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-button size="small" @click="showResult(row)">结果</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination class="pager" v-model:current-page="q.page" :page-size="q.pageSize" :total="total" @current-change="load" />

    <el-dialog v-model="execVisible" title="快速执行" width="560px">
      <el-form :model="exec" label-width="90px">
        <el-form-item label="引用模板">
          <el-select v-model="exec.templateId" clearable filterable placeholder="可选：从模板带出命令/脚本" style="width:100%" @change="onTemplateChange">
            <el-option v-for="t in templates" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="exec.type" style="width:100%">
            <el-option label="执行命令" value="command" />
            <el-option label="执行脚本" value="script" />
          </el-select>
        </el-form-item>
        <el-form-item label="需要 root">
          <el-switch v-model="exec.needRoot" active-text="以 root 执行" />
          <div style="color:#909399;font-size:12px;margin-left:8px">开启后后端自动 sudo 切换 root 执行（需主机已配置可提权凭证）</div>
        </el-form-item>
        <el-form-item label="主机" v-if="exec.type === 'command'">
          <el-select v-model="exec.hostIds" multiple filterable style="width:100%">
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name}(${h.ip})`" :value="h.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="命令" v-if="exec.type === 'command'">
          <el-input v-model="exec.command" type="textarea" :rows="4" placeholder="如 uptime" />
        </el-form-item>
        <el-form-item label="脚本" v-else>
          <el-select v-model="exec.scriptId" style="width:100%">
            <el-option v-for="s in scripts" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <div style="color:#909399;font-size:12px">将批量在所选主机执行（此处选择主机来自主机列表）</div>
          <el-select v-model="exec.hostIds" multiple filterable style="width:100%;margin-top:8px">
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name}(${h.ip})`" :value="h.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="execVisible = false">取消</el-button>
        <el-button type="primary" :loading="running" @click="run">执行</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="resultVisible" title="执行结果" width="680px">
      <pre class="result">{{ resultText }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api'

const list = ref<any[]>([])
const hosts = ref<any[]>([])
const scripts = ref<any[]>([])
const templates = ref<any[]>([])
const total = ref(0)
const loading = ref(false)
const execVisible = ref(false)
const resultVisible = ref(false)
const running = ref(false)
const resultText = ref('')
const typeMap: any = { command: '命令', script: '脚本', file: '文件' }
const q = reactive({ page: 1, pageSize: 10 })
const exec = reactive<any>({ type: 'command', hostIds: [], command: '', scriptId: null, templateId: null, needRoot: false })

async function load() {
  loading.value = true
  const r = await api.listExecutions({ ...q })
  list.value = r.list; total.value = r.total
  loading.value = false
}
async function loadMeta() {
  const h = await api.listHosts({ page: 1, pageSize: 200 })
  hosts.value = h.list || []
  scripts.value = await api.listScripts({})
  templates.value = await api.listTemplates()
}
function onTemplateChange(val: any) {
  if (!val) return
  const t = templates.value.find((x: any) => x.id === val)
  if (t) {
    exec.type = t.type
    exec.command = t.command || ''
    exec.scriptId = t.scriptId || null
  }
}
function openExec() { execVisible.value = true }
function statusType(s: string) {
  return { success: 'success', failed: 'danger', partial: 'warning', running: 'info', canceled: 'info' }[s] || 'info'
}
function statusText(s: string) {
  return { success: '成功', failed: '失败', partial: '部分失败', running: '执行中', canceled: '已取消' }[s] || s
}
async function run() {
  if (exec.hostIds.length === 0) { ElMessage.warning('请选择主机'); return }
  running.value = true
  const r: any = await api.quickExec({ ...exec })
  running.value = false
  execVisible.value = false
  ElMessage.success('已提交，正在执行')
  // 轮询结果
  pollResult(r.executionId)
}
async function pollResult(id: number) {
  for (let i = 0; i < 12; i++) {
    await new Promise((res) => setTimeout(res, 1500))
    const r = await api.listExecutions({ page: 1, pageSize: 50 })
    const item = (r.list || []).find((e: any) => e.id === id)
    if (item && item.status !== 'running') {
      showResult(item)
      return
    }
  }
  ElMessage.info('执行仍在后台进行，请稍后在列表中查看结果')
}
function showResult(row: any) {
  try {
    const arr = JSON.parse(row.result || '[]')
    resultText.value = arr.map((a: any) => `【${a.ip}】\n${a.error ? '错误: ' + a.error : a.output}`).join('\n\n')
  } catch (e) {
    resultText.value = row.result || '(无结果)'
  }
  resultVisible.value = true
  load()
}
onMounted(() => { loadMeta(); load() })
</script>

<style scoped>
.pager { margin-top: 12px; display: flex; justify-content: flex-end; }
.result { background: #1e1e1e; color: #d4d4d4; padding: 12px; border-radius: 4px; max-height: 420px; overflow: auto; white-space: pre-wrap; }
</style>
