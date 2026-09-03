<template>
  <div class="page">
    <div class="search-bar">
      <el-button type="primary" @click="openCreate">新建定时任务</el-button>
      <span class="tip">Cron 标准 5 段：分 时 日 月 周，如 <code>0 2 * * *</code> 每天 02:00，<code>*/5 * * * *</code> 每 5 分钟</span>
    </div>

    <el-table :data="list" border stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="名称" min-width="120" />
      <el-table-column label="类型" width="90">
        <template #default="{ row }">
          <el-tag>{{ typeMap[row.type] }}</el-tag>
          <el-tag v-if="row.needRoot" type="danger" size="small" style="margin-left:4px">root</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="cron" label="Cron" width="140">
        <template #default="{ row }"><code>{{ row.cron }}</code></template>
      </el-table-column>
      <el-table-column label="目标主机" min-width="160">
        <template #default="{ row }">
          <el-tag v-for="hid in hostIdList(row)" :key="hid" size="small" style="margin:0 4px 4px 0">{{ hostName(hid) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="启用" width="80">
        <template #default="{ row }">
          <el-switch :model-value="row.enabled === 1" @change="() => toggle(row)" />
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }"><el-tag :type="row.status === 'running' ? 'warning' : 'info'">{{ row.status === 'running' ? '执行中' : '空闲' }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="lastRunAt" label="上次运行" width="170" />
      <el-table-column prop="nextRunAt" label="下次运行" width="170" />
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="success" @click="runNow(row)">执行</el-button>
          <el-button size="small" @click="showResult(row)">结果</el-button>
          <el-button size="small" type="danger" @click="del(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" :title="form.id ? '编辑定时任务' : '新建定时任务'" width="600px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称">
          <el-input v-model="form.name" placeholder="任务名称" />
        </el-form-item>
        <el-form-item label="引用模板">
          <el-select v-model="form.templateId" clearable filterable placeholder="可选：从模板带出命令/脚本" style="width:100%" @change="onTemplateChange">
            <el-option v-for="t in templates" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.type" style="width:100%">
            <el-option label="执行命令" value="command" />
            <el-option label="执行脚本" value="script" />
          </el-select>
        </el-form-item>
        <el-form-item label="需要 root">
          <el-switch v-model="form.needRoot" active-text="定时以 root 执行" />
          <div style="color:#909399;font-size:12px;margin-left:8px">开启后每次调度自动 sudo 切换 root 执行</div>
        </el-form-item>
        <el-form-item label="命令" v-if="form.type === 'command'">
          <el-input v-model="form.command" type="textarea" :rows="4" placeholder="如 uptime" />
        </el-form-item>
        <el-form-item label="脚本" v-else>
          <el-select v-model="form.scriptId" style="width:100%">
            <el-option v-for="s in scripts" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="目标主机">
          <el-select v-model="form.hostIds" multiple filterable style="width:100%">
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name}(${h.ip})`" :value="h.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="Cron">
          <el-input v-model="form.cron" placeholder="0 2 * * *" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" :active-value="1" :inactive-value="0" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="resultVisible" title="最近一次执行结果" width="680px">
      <pre class="result">{{ resultText }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/api'

const list = ref<any[]>([])
const hosts = ref<any[]>([])
const scripts = ref<any[]>([])
const templates = ref<any[]>([])
const loading = ref(false)
const visible = ref(false)
const resultVisible = ref(false)
const saving = ref(false)
const resultText = ref('')
const typeMap: any = { command: '命令', script: '脚本' }

const form = reactive<any>({ id: null, name: '', templateId: null, type: 'command', command: '', scriptId: null, hostIds: [], cron: '', enabled: 1, remark: '', needRoot: false })

function hostIdList(row: any): number[] {
  try { return JSON.parse(row.hostIds || '[]') } catch (e) { return [] }
}
function hostName(id: number) {
  const h = hosts.value.find((x) => x.id === id)
  return h ? `${h.name}(${h.ip})` : `#${id}`
}

async function load() {
  loading.value = true
  list.value = await api.listSchedules()
  loading.value = false
}
async function loadMeta() {
  const h = await api.listHosts({ page: 1, pageSize: 200 })
  hosts.value = h.list || []
  scripts.value = await api.listScripts({})
  templates.value = await api.listTemplates()
}
function resetForm() {
  Object.assign(form, { id: null, name: '', templateId: null, type: 'command', command: '', scriptId: null, hostIds: [], cron: '', enabled: 1, remark: '', needRoot: false })
}
function onTemplateChange(val: any) {
  if (!val) return
  const t = templates.value.find((x: any) => x.id === val)
  if (t) {
    form.type = t.type
    form.command = t.command || ''
    form.scriptId = t.scriptId || null
  }
}
function openCreate() { resetForm(); visible.value = true }
function openEdit(row: any) {
  resetForm()
  Object.assign(form, {
    id: row.id, name: row.name, templateId: row.templateId, type: row.type, command: row.command,
    scriptId: row.scriptId, hostIds: hostIdList(row), cron: row.cron,
    enabled: row.enabled, remark: row.remark, needRoot: row.needRoot,
  })
  visible.value = true
}
async function save() {
  if (!form.name) { ElMessage.warning('请填写名称'); return }
  if (!form.cron) { ElMessage.warning('请填写 Cron'); return }
  if (form.hostIds.length === 0) { ElMessage.warning('请选择目标主机'); return }
  saving.value = true
  if (form.id) {
    await api.updateSchedule(form.id, { ...form })
    ElMessage.success('更新成功')
  } else {
    await api.createSchedule({ ...form })
    ElMessage.success('创建成功')
  }
  saving.value = false
  visible.value = false
  load()
}
async function toggle(row: any) {
  await api.toggleSchedule(row.id)
  ElMessage.success('操作成功')
  load()
}
async function runNow(row: any) {
  const r: any = await api.runSchedule(row.id)
  ElMessage.success('已触发执行')
  if (r && r.executionId) pollResult(r.executionId)
  load()
}
async function del(row: any) {
  try {
    await ElMessageBox.confirm(`确认删除定时任务「${row.name}」？`, '提示', { type: 'warning' })
  } catch (e) {
    return
  }
  await api.deleteSchedule(row.id)
  ElMessage.success('已删除')
  load()
}
async function pollResult(id: number) {
  for (let i = 0; i < 12; i++) {
    await new Promise((res) => setTimeout(res, 1500))
    const r = await api.listExecutions({ page: 1, pageSize: 50 })
    const item = (r.list || []).find((e: any) => e.id === id)
    if (item && item.status !== 'running') { showResult(item); return }
  }
}
function showResult(row: any) {
  try {
    const arr = JSON.parse(row.lastResult || row.result || '[]')
    if (Array.isArray(arr) && arr.length) {
      resultText.value = arr.map((a: any) => `【${a.ip}】\n${a.error ? '错误: ' + a.error : a.output}`).join('\n\n')
    } else {
      resultText.value = row.lastResult || row.result || '(无结果)'
    }
  } catch (e) {
    resultText.value = row.lastResult || row.result || '(无结果)'
  }
  resultVisible.value = true
}
onMounted(() => { loadMeta(); load() })
</script>

<style scoped>
.search-bar { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.tip { color: #909399; font-size: 12px; }
.tip code { background: #f4f4f5; padding: 0 4px; border-radius: 3px; }
.result { background: #1e1e1e; color: #d4d4d4; padding: 12px; border-radius: 4px; max-height: 420px; overflow: auto; white-space: pre-wrap; }
</style>
