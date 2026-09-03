<template>
  <div class="page">
    <div class="search-bar">
      <el-select v-model="q.status" placeholder="状态" clearable style="width:140px" @change="load">
        <el-option label="待审批" value="pending" />
        <el-option label="已执行" value="executed" />
        <el-option label="已拒绝" value="rejected" />
        <el-option label="已撤回" value="canceled" />
      </el-select>
      <el-input v-model="q.keyword" placeholder="申请人/事由/命令" style="width:220px" clearable @keyup.enter="load" @clear="load" />
      <el-button type="primary" @click="load">查询</el-button>
      <el-button type="success" @click="openSubmit">提交审批</el-button>
    </div>

    <el-table :data="list" border stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="requesterName" label="申请人" width="110" />
      <el-table-column label="类型" width="90">
        <template #default="{ row }"><el-tag>{{ typeMap[row.type] }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="command" label="命令/脚本" show-overflow-tooltip />
      <el-table-column prop="reason" label="事由" show-overflow-tooltip />
      <el-table-column label="状态" width="100">
        <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="submittedAt" label="提交时间" width="170" />
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button size="small" @click="showDetail(row)">详情</el-button>
          <template v-if="row.status === 'pending'">
            <el-button size="small" type="success" @click="approve(row)" v-if="canApprove">通过</el-button>
            <el-button size="small" type="danger" @click="reject(row)" v-if="canApprove">拒绝</el-button>
            <el-button size="small" @click="cancel(row)" v-if="row.requesterId === myId">撤回</el-button>
          </template>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination class="pager" v-model:current-page="q.page" :page-size="q.pageSize" :total="total" @current-change="load" />

    <!-- 提交审批 -->
    <el-dialog v-model="submitVisible" title="提交执行审批" width="600px">
      <el-alert type="warning" :closable="false" style="margin-bottom:12px"
        title="该操作将进入审批流：审批人通过后才会实际执行，并关联交易执行记录。" />
      <el-form :model="form" label-width="90px">
        <el-form-item label="类型">
          <el-select v-model="form.type" style="width:100%">
            <el-option label="执行命令" value="command" />
            <el-option label="执行脚本" value="script" />
          </el-select>
        </el-form-item>
        <el-form-item label="主机">
          <el-select v-model="form.hostIds" multiple filterable style="width:100%">
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name}(${h.ip})`" :value="h.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="命令" v-if="form.type === 'command'">
          <el-input v-model="form.command" type="textarea" :rows="4" placeholder="如 systemctl restart nginx" />
        </el-form-item>
        <el-form-item label="脚本" v-else>
          <el-select v-model="form.scriptId" style="width:100%">
            <el-option v-for="s in scripts" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="申请事由">
          <el-input v-model="form.reason" type="textarea" :rows="2" placeholder="请说明执行目的（必填，便于审批）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="submitVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">提交</el-button>
      </template>
    </el-dialog>

    <!-- 详情 -->
    <el-dialog v-model="detailVisible" title="审批详情" width="680px">
      <el-descriptions :column="1" border v-if="detail">
        <el-descriptions-item label="ID">{{ detail.id }}</el-descriptions-item>
        <el-descriptions-item label="申请人">{{ detail.requesterName }}</el-descriptions-item>
        <el-descriptions-item label="类型">{{ typeMap[detail.type] }}</el-descriptions-item>
        <el-descriptions-item label="命令/脚本">{{ detail.command }}</el-descriptions-item>
        <el-descriptions-item label="目标主机">{{ hostNames(detail.hostIds) }}</el-descriptions-item>
        <el-descriptions-item label="申请事由">{{ detail.reason }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="statusType(detail.status)">{{ statusText(detail.status) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="审批人">{{ detail.approverName || '—' }}</el-descriptions-item>
        <el-descriptions-item label="审批意见">{{ detail.comment || '—' }}</el-descriptions-item>
        <el-descriptions-item label="提交时间">{{ detail.submittedAt || '—' }}</el-descriptions-item>
        <el-descriptions-item label="处理时间">{{ detail.decidedAt || '—' }}</el-descriptions-item>
        <el-descriptions-item label="关联交易" v-if="detail.executionId">
          <el-link type="primary" @click="goExec(detail.executionId)">执行记录 #{{ detail.executionId }}</el-link>
        </el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
        <template v-if="detail.status === 'pending' && canApprove">
          <el-button type="success" :loading="acting" @click="approve(detail, true)">通过并执行</el-button>
          <el-button type="danger" :loading="acting" @click="reject(detail, true)">拒绝</el-button>
        </template>
        <el-button v-if="detail.status === 'pending' && detail.requesterId === myId" @click="cancel(detail, true)">撤回</el-button>
      </template>
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
const total = ref(0)
const loading = ref(false)
const canApprove = ref(false)
const myId = ref<number>(0)
const typeMap: any = { command: '命令', script: '脚本', file: '文件' }

const q = reactive({ page: 1, pageSize: 10, status: '', keyword: '' })
const form = reactive<any>({ type: 'command', hostIds: [], command: '', scriptId: null, reason: '' })
const submitVisible = ref(false)
const submitting = ref(false)
const detailVisible = ref(false)
const detail = ref<any>(null)
const acting = ref(false)

async function load() {
  loading.value = true
  const r = await api.listApprovals({ ...q })
  list.value = r.list || []; total.value = r.total || 0
  loading.value = false
}
async function loadMeta() {
  const p = await api.profile()
  myId.value = p.id || 0
  // 是否具备审批权限：绑定了 task:approval:approve 的菜单（超级管理员恒为 true）
  canApprove.value = (p.roles || []).some((role: any) =>
    (role.menus || []).some((m: any) => m.permission === 'task:approval:approve'))
  const h = await api.listHosts({ page: 1, pageSize: 200 })
  hosts.value = h.list || []
  scripts.value = await api.listScripts({})
}
function openSubmit() { form.type = 'command'; form.hostIds = []; form.command = ''; form.scriptId = null; form.reason = ''; submitVisible.value = true }
async function submit() {
  if (form.hostIds.length === 0) { ElMessage.warning('请选择目标主机'); return }
  if (form.type === 'command' && !form.command.trim()) { ElMessage.warning('请输入命令'); return }
  if (form.type === 'script' && !form.scriptId) { ElMessage.warning('请选择脚本'); return }
  if (!form.reason.trim()) { ElMessage.warning('请填写申请事由'); return }
  submitting.value = true
  await api.createApproval({ ...form })
  submitting.value = false
  submitVisible.value = false
  ElMessage.success('已提交，等待审批')
  load()
}
async function showDetail(row: any) {
  detail.value = await api.getApproval(row.id)
  detailVisible.value = true
}
function statusType(s: string) {
  return { pending: 'warning', executed: 'success', rejected: 'danger', canceled: 'info' }[s] || 'info'
}
function statusText(s: string) {
  return { pending: '待审批', executed: '已执行', rejected: '已拒绝', canceled: '已撤回' }[s] || s
}
function hostNames(json: string) {
  let ids: number[] = []
  try { ids = JSON.parse(json || '[]') } catch (e) { ids = [] }
  return ids.map((id) => {
    const h = hosts.value.find((x) => x.id === id)
    return h ? `${h.name}(${h.ip})` : `#${id}`
  }).join('、') || '—'
}
async function approve(row: any, fromDetail = false) {
  await ElMessageBox.confirm(`确认通过并立即执行「${row.reason || row.command}」？`, '审批通过', { type: 'warning' })
  acting.value = true
  const r: any = await api.approveApproval(row.id)
  acting.value = false
  ElMessage.success('已通过，任务执行中')
  if (fromDetail && r.executionId) { detailVisible.value = false; /* 可跳转执行记录 */ }
  load()
}
async function reject(row: any, fromDetail = false) {
  const { value } = await ElMessageBox.prompt('请填写拒绝理由', '审批拒绝', { inputType: 'textarea' })
  acting.value = true
  await api.rejectApproval(row.id, { comment: value || '' })
  acting.value = false
  ElMessage.success('已拒绝')
  if (fromDetail) detailVisible.value = false
  load()
}
async function cancel(row: any, fromDetail = false) {
  await ElMessageBox.confirm('确认撤回该审批申请？', '撤回', { type: 'warning' })
  await api.cancelApproval(row.id)
  ElMessage.success('已撤回')
  if (fromDetail) detailVisible.value = false
  load()
}
function goExec(id: number) {
  detailVisible.value = false
  // 跳转到任务执行页查看结果
  window.location.hash = '#/task/execution'
  ElMessage.info(`已关联执行记录 #${id}`)
}
onMounted(() => { loadMeta(); load() })
</script>

<style scoped>
.search-bar { display: flex; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
.pager { margin-top: 12px; display: flex; justify-content: flex-end; }
</style>
