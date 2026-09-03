<template>
  <div class="page">
    <div class="search-bar">
      <el-button type="success" @click="visible = true">新增授权</el-button>
    </div>
    <el-table :data="list" border stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="user_id" label="用户ID" width="90" />
      <el-table-column prop="target_type" label="资源类型" width="140">
        <template #default="{ row }">
          <el-tag>{{ typeMap[row.target_type] || row.target_type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="target_id" label="资源ID" width="90" />
      <el-table-column label="操作" width="120">
        <template #default="{ row }">
          <el-button size="small" type="danger" @click="remove(row)">撤销</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" title="新增授权" width="460px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="用户">
          <el-select v-model="form.userId" filterable style="width:100%">
            <el-option v-for="u in users" :key="u.id" :label="`${u.username}(${u.nickname})`" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="资源类型">
          <el-select v-model="form.targetType" style="width:100%" @change="onTypeChange">
            <el-option label="主机" value="host" />
            <el-option label="凭证" value="credential" />
            <el-option label="主机分组" value="hostGroup" />
          </el-select>
        </el-form-item>
        <el-form-item label="资源">
          <el-select v-model="form.targetIds" multiple style="width:100%">
            <el-option v-for="t in targets" :key="t.id" :label="t.label" :value="t.id" />
          </el-select>
        </el-form-item>
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
const users = ref<any[]>([])
const targets = ref<any[]>([])
const visible = ref(false)
const typeMap: any = { host: '主机', credential: '凭证', hostGroup: '主机分组' }
const form = reactive<any>({ userId: null, targetType: 'host', targetIds: [] })

async function load() { list.value = await api.listAuths({}) }
async function loadUsers() {
  const r = await api.listUsers({ page: 1, pageSize: 200 })
  users.value = r.list
}
function onTypeChange() {
  form.targetIds = []
  if (form.targetType === 'host') {
    api.listHosts({ page: 1, pageSize: 200 }).then((r: any) => {
      targets.value = (r.list || []).map((h: any) => ({ id: h.id, label: `${h.name}(${h.ip})` }))
    })
  } else if (form.targetType === 'credential') {
    api.listCredentials().then((r: any) => {
      targets.value = r.map((c: any) => ({ id: c.id, label: `${c.name}(${c.username})` }))
    })
  } else {
    api.listHostGroups().then((r: any) => {
      targets.value = r.map((g: any) => ({ id: g.id, label: g.name }))
    })
  }
}
async function submit() {
  if (!form.userId || form.targetIds.length === 0) {
    ElMessage.warning('请选择用户与资源')
    return
  }
  await api.createAuth({ ...form })
  ElMessage.success('授权成功')
  visible.value = false
  load()
}
async function remove(row: any) {
  await ElMessageBox.confirm('确认撤销该授权？', '提示', { type: 'warning' })
  await api.deleteAuth(row.id)
  ElMessage.success('已撤销')
  load()
}
onMounted(() => { loadUsers(); load() })
</script>
