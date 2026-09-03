<template>
  <div class="page">
    <div class="search-bar">
      <el-input v-model="q.keyword" placeholder="名称/IP" style="width:200px" @keyup.enter="load" />
      <el-button type="primary" @click="load">查询</el-button>
      <el-button type="success" @click="openCreate">新增主机</el-button>
    </div>
    <el-table :data="list" border stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="ip" label="IP" />
      <el-table-column prop="port" label="端口" width="80" />
      <el-table-column prop="os" label="系统" width="100" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }"><el-tag :type="row.status ? 'success' : 'info'">{{ row.status ? '在线' : '离线' }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" />
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination class="pager" v-model:current-page="q.page" :page-size="q.pageSize" :total="total" @current-change="load" />

    <el-dialog v-model="visible" :title="form.id ? '编辑主机' : '新增主机'" width="480px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="IP"><el-input v-model="form.ip" /></el-form-item>
        <el-form-item label="端口"><el-input-number v-model="form.port" :min="1" :max="65535" /></el-form-item>
        <el-form-item label="系统"><el-input v-model="form.os" placeholder="如 Linux/CentOS" /></el-form-item>
        <el-form-item label="分组">
          <el-select v-model="form.groupId" style="width:100%" clearable>
            <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="凭证">
          <el-select v-model="form.credentialId" style="width:100%" clearable>
            <el-option v-for="c in creds" :key="c.id" :label="`${c.name} (${c.username})`" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="连接用户"><el-input v-model="form.user" placeholder="留空使用凭证用户名" /></el-form-item>
        <el-form-item label="切换 root">
          <el-switch v-model="form.becomeRoot" active-text="sudo -i 提权" inactive-text="直登用户" />
          <div class="hint">远端禁止 root 直登时开启：以凭证用户登录后自动 sudo -i 切换 root 执行命令</div>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" @click="submit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/api'

const list = ref<any[]>([])
const groups = ref<any[]>([])
const creds = ref<any[]>([])
const total = ref(0)
const loading = ref(false)
const visible = ref(false)
const q = reactive({ page: 1, pageSize: 10, keyword: '' })
const form = reactive<any>({ id: null, name: '', ip: '', port: 22, os: '', groupId: null, credentialId: null, user: '', becomeRoot: false, remark: '' })

async function load() {
  loading.value = true
  const r = await api.listHosts({ ...q })
  list.value = r.list; total.value = r.total
  loading.value = false
}
async function loadMeta() {
  groups.value = await api.listHostGroups()
  creds.value = await api.listCredentials()
}
function openCreate() {
  Object.assign(form, { id: null, name: '', ip: '', port: 22, os: '', groupId: null, credentialId: null, user: '', becomeRoot: false, remark: '' })
  visible.value = true
}
function openEdit(row: any) { Object.assign(form, { ...row }); visible.value = true }
async function submit() {
  if (form.id) await api.updateHost(form.id, form)
  else await api.createHost(form)
  ElMessage.success('保存成功')
  visible.value = false
  load()
}
async function remove(row: any) {
  await ElMessageBox.confirm('确认删除该主机？', '提示', { type: 'warning' })
  await api.deleteHost(row.id)
  ElMessage.success('已删除')
  load()
}
onMounted(() => { loadMeta(); load() })
</script>

<style scoped>
.pager { margin-top: 12px; display: flex; justify-content: flex-end; }
.hint { font-size: 12px; color: #909399; line-height: 1.4; margin-top: 4px; }
</style>
