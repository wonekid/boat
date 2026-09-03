<template>
  <div class="page">
    <div class="search-bar">
      <el-button type="success" @click="openCreate">新增角色</el-button>
    </div>
    <el-table :data="list" border stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="code" label="编码" />
      <el-table-column prop="remark" label="备注" />
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

    <el-dialog v-model="visible" :title="form.id ? '编辑角色' : '新增角色'" width="460px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="编码"><el-input v-model="form.code" :disabled="!!form.id" /></el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" />
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" /></el-form-item>
        <el-form-item label="菜单权限">
          <el-select v-model="form.menuIds" multiple filterable style="width:100%" placeholder="分配菜单/权限">
            <el-option v-for="m in menus" :key="m.id" :label="`${m.name}${m.permission ? ' (' + m.permission + ')' : ''}`" :value="m.id" />
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
const menus = ref<any[]>([])
const visible = ref(false)
const form = reactive<any>({ id: null, name: '', code: '', status: 1, remark: '', menuIds: [] })

async function load() {
  list.value = await api.listRoles()
}
async function loadMenus() {
  // 平铺菜单用于选择
  const tree = await api.listMenus()
  const flat: any[] = []
  const walk = (arr: any[]) => arr.forEach((n) => { flat.push(n); if (n.children) walk(n.children) })
  walk(tree)
  menus.value = flat
}

function openCreate() {
  Object.assign(form, { id: null, name: '', code: '', status: 1, remark: '', menuIds: [] })
  visible.value = true
}
function openEdit(row: any) {
  Object.assign(form, { id: row.id, name: row.name, code: row.code, status: row.status, remark: row.remark, menuIds: (row.menus || []).map((m: any) => m.id) })
  visible.value = true
}
async function submit() {
  if (form.id) await api.updateRole(form.id, form)
  else await api.createRole(form)
  ElMessage.success('保存成功')
  visible.value = false
  load()
}
async function remove(row: any) {
  await ElMessageBox.confirm('确认删除该角色？', '提示', { type: 'warning' })
  await api.deleteRole(row.id)
  ElMessage.success('已删除')
  load()
}
onMounted(() => { loadMenus(); load() })
</script>
