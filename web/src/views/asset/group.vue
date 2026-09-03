<template>
  <div class="page">
    <div class="search-bar">
      <el-button type="success" @click="openCreate(null)">新增顶级分组</el-button>
    </div>
    <el-table :data="list" border stripe row-key="id" default-expand-all :tree-props="{ children: 'children' }">
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="remark" label="备注" />
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button size="small" @click="openCreate(row)">子项</el-button>
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" :title="form.id ? '编辑分组' : '新增分组'" width="400px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="上级"><el-input :model-value="parentName" disabled /></el-form-item>
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
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
const parentName = ref('')
const form = reactive<any>({ id: null, parentId: 0, name: '', remark: '' })

async function load() { list.value = await api.listHostGroups() }
function openCreate(p: any) {
  Object.assign(form, { id: null, parentId: p ? p.id : 0, name: '', remark: '' })
  parentName.value = p ? p.name : '顶级'
  visible.value = true
}
function openEdit(row: any) { Object.assign(form, { ...row }); parentName.value = '当前'; visible.value = true }
async function submit() {
  if (form.id) await api.updateHostGroup(form.id, form)
  else await api.createHostGroup(form)
  ElMessage.success('保存成功')
  visible.value = false
  load()
}
async function remove(row: any) {
  await ElMessageBox.confirm('确认删除该分组？', '提示', { type: 'warning' })
  await api.deleteHostGroup(row.id)
  ElMessage.success('已删除')
  load()
}
onMounted(load)
</script>
