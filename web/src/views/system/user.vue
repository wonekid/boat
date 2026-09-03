<template>
  <div class="page">
    <div class="search-bar">
      <el-input v-model="q.keyword" placeholder="用户名/昵称" style="width:200px" @keyup.enter="load" />
      <el-button type="primary" @click="load">查询</el-button>
      <el-button type="success" @click="openCreate">新增</el-button>
    </div>
    <el-table :data="list" border stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" label="用户名" />
      <el-table-column prop="nickname" label="昵称" />
      <el-table-column prop="email" label="邮箱" />
      <el-table-column prop="phone" label="手机" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }"><el-tag :type="row.status ? 'success' : 'info'">{{ row.status ? '启用' : '禁用' }}</el-tag></template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination class="pager" v-model:current-page="q.page" :page-size="q.pageSize" :total="total" @current-change="load" />

    <el-dialog v-model="visible" :title="form.id ? '编辑用户' : '新增用户'" width="460px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="用户名"><el-input v-model="form.username" :disabled="!!form.id" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="form.password" placeholder="留空则不修改（新增默认123456）" /></el-form-item>
        <el-form-item label="昵称"><el-input v-model="form.nickname" /></el-form-item>
        <el-form-item label="邮箱"><el-input v-model="form.email" /></el-form-item>
        <el-form-item label="手机"><el-input v-model="form.phone" /></el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.roleIds" multiple style="width:100%">
            <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
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
const roles = ref<any[]>([])
const total = ref(0)
const loading = ref(false)
const visible = ref(false)
const q = reactive({ page: 1, pageSize: 10, keyword: '' })
const form = reactive<any>({ id: null, username: '', password: '', nickname: '', email: '', phone: '', status: 1, roleIds: [] })

async function load() {
  loading.value = true
  const r = await api.listUsers({ ...q })
  list.value = r.list
  total.value = r.total
  loading.value = false
}

async function loadRoles() {
  roles.value = await api.listRoles()
}

function openCreate() {
  Object.assign(form, { id: null, username: '', password: '', nickname: '', email: '', phone: '', status: 1, roleIds: [] })
  visible.value = true
}

function openEdit(row: any) {
  Object.assign(form, { id: row.id, username: row.username, password: '', nickname: row.nickname, email: row.email, phone: row.phone, status: row.status, roleIds: (row.roles || []).map((r: any) => r.id) })
  visible.value = true
}

async function submit() {
  if (form.id) {
    await api.updateUser(form.id, form)
  } else {
    await api.createUser(form)
  }
  ElMessage.success('保存成功')
  visible.value = false
  load()
}

async function remove(row: any) {
  await ElMessageBox.confirm('确认删除该用户？', '提示', { type: 'warning' })
  await api.deleteUser(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(() => { loadRoles(); load() })
</script>

<style scoped>
.pager { margin-top: 12px; justify-content: flex-end; display: flex; }
</style>
