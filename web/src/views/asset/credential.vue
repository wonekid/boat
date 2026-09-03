<template>
  <div class="page">
    <div class="search-bar">
      <el-button type="success" @click="openCreate">新增凭证</el-button>
    </div>
    <el-table :data="list" border stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="username" label="用户名" />
      <el-table-column label="类型" width="100">
        <template #default="{ row }"><el-tag>{{ row.type === 1 ? '密码' : '密钥' }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" />
      <el-table-column label="操作" width="220">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" @click="test(row)">测试</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" :title="form.id ? '编辑凭证' : '新增凭证'" width="460px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="用户名"><el-input v-model="form.username" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.type" style="width:100%">
            <el-option label="密码" :value="1" />
            <el-option label="密钥" :value="2" />
          </el-select>
        </el-form-item>
        <el-form-item label="密码" v-if="form.type === 1">
          <el-input v-model="form.authPassword" type="password" show-password placeholder="留空不修改" />
        </el-form-item>
        <el-form-item label="私钥" v-else>
          <el-input v-model="form.privateKey" type="textarea" :rows="4" placeholder="PEM 私钥，留空不修改" />
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
const visible = ref(false)
const form = reactive<any>({ id: null, name: '', username: '', type: 1, authPassword: '', privateKey: '', remark: '' })

async function load() { list.value = await api.listCredentials() }
function openCreate() {
  Object.assign(form, { id: null, name: '', username: '', type: 1, authPassword: '', privateKey: '', remark: '' })
  visible.value = true
}
function openEdit(row: any) {
  Object.assign(form, { id: row.id, name: row.name, username: row.username, type: row.type, authPassword: '', privateKey: '', remark: row.remark })
  visible.value = true
}
async function submit() {
  if (form.id) await api.updateCredential(form.id, form)
  else await api.createCredential(form)
  ElMessage.success('保存成功')
  visible.value = false
  load()
}
async function test(row: any) {
  try {
    await api.testCredential(row.id)
    ElMessage.success('凭证可用')
  } catch (e) {
    ElMessage.error('测试失败')
  }
}
async function remove(row: any) {
  await ElMessageBox.confirm('确认删除该凭证？', '提示', { type: 'warning' })
  await api.deleteCredential(row.id)
  ElMessage.success('已删除')
  load()
}
onMounted(load)
</script>
