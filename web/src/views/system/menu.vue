<template>
  <div class="page">
    <div class="search-bar">
      <el-button type="success" @click="openCreate(null)">新增顶级菜单</el-button>
    </div>
    <el-table :data="list" border stripe row-key="id" default-expand-all :tree-props="{ children: 'children' }">
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="type" label="类型" width="90">
        <template #default="{ row }">
          <el-tag>{{ ['', '目录', '菜单', '按钮'][row.type] || row.type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="permission" label="权限标识" />
      <el-table-column prop="path" label="路径" />
      <el-table-column prop="component" label="组件" />
      <el-table-column prop="sort" label="排序" width="70" />
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button size="small" @click="openCreate(row)">子项</el-button>
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" :title="form.id ? '编辑菜单' : '新增菜单'" width="460px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="上级"><el-input :model-value="parentName" disabled /></el-form-item>
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.type" style="width:100%">
            <el-option label="目录" :value="1" />
            <el-option label="菜单" :value="2" />
            <el-option label="按钮" :value="3" />
          </el-select>
        </el-form-item>
        <el-form-item label="权限标识"><el-input v-model="form.permission" placeholder="如 system:user:list" /></el-form-item>
        <el-form-item label="路径"><el-input v-model="form.path" /></el-form-item>
        <el-form-item label="组件"><el-input v-model="form.component" /></el-form-item>
        <el-form-item label="图标"><el-input v-model="form.icon" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="form.sort" :min="0" /></el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" />
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
const visible = ref(false)
const parentName = ref('')
const form = reactive<any>({ id: null, parentId: 0, name: '', type: 2, permission: '', path: '', component: '', icon: '', sort: 0, status: 1 })

async function load() {
  list.value = await api.listMenus()
}
function openCreate(parent: any) {
  Object.assign(form, { id: null, parentId: parent ? parent.id : 0, name: '', type: parent ? 2 : 1, permission: '', path: '', component: '', icon: '', sort: 0, status: 1 })
  parentName.value = parent ? parent.name : '顶级'
  visible.value = true
}
function openEdit(row: any) {
  Object.assign(form, { ...row })
  parentName.value = '当前'
  visible.value = true
}
async function submit() {
  if (form.id) await api.updateMenu(form.id, form)
  else await api.createMenu(form)
  ElMessage.success('保存成功')
  visible.value = false
  load()
}
async function remove(row: any) {
  await ElMessageBox.confirm('确认删除该菜单？', '提示', { type: 'warning' })
  await api.deleteMenu(row.id)
  ElMessage.success('已删除')
  load()
}
onMounted(load)
</script>
