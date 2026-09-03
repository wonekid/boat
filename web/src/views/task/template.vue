<template>
  <div class="page">
    <div class="search-bar">
      <el-button type="success" @click="openCreate">新增模板</el-button>
    </div>
    <el-table :data="list" border stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="名称" />
      <el-table-column label="类型" width="100">
        <template #default="{ row }"><el-tag>{{ typeMap[row.type] }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="command" label="命令/脚本" show-overflow-tooltip />
      <el-table-column prop="timeout" label="超时(秒)" width="100" />
      <el-table-column prop="remark" label="备注" />
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" :title="form.id ? '编辑模板' : '新增模板'" width="520px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.type" style="width:100%">
            <el-option label="执行命令" value="command" />
            <el-option label="执行脚本" value="script" />
            <el-option label="分发文件" value="file" />
          </el-select>
        </el-form-item>
        <el-form-item label="命令" v-if="form.type === 'command'">
          <el-input v-model="form.command" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="脚本" v-else-if="form.type === 'script'">
          <el-select v-model="form.scriptId" style="width:100%">
            <el-option v-for="s in scripts" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="凭证">
          <el-select v-model="form.credentialId" clearable style="width:100%">
            <el-option v-for="c in creds" :key="c.id" :label="`${c.name}(${c.username})`" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="超时"><el-input-number v-model="form.timeout" :min="1" /></el-form-item>
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
const scripts = ref<any[]>([])
const creds = ref<any[]>([])
const visible = ref(false)
const typeMap: any = { command: '命令', script: '脚本', file: '文件' }
const form = reactive<any>({ id: null, name: '', type: 'command', command: '', scriptId: null, credentialId: null, timeout: 300, remark: '' })

async function load() { list.value = await api.listTemplates() }
async function loadMeta() {
  scripts.value = await api.listScripts({})
  creds.value = await api.listCredentials()
}
function openCreate() {
  Object.assign(form, { id: null, name: '', type: 'command', command: '', scriptId: null, credentialId: null, timeout: 300, remark: '' })
  visible.value = true
}
function openEdit(row: any) { Object.assign(form, { ...row }); visible.value = true }
async function submit() {
  if (form.id) await api.updateTemplate(form.id, form)
  else await api.createTemplate(form)
  ElMessage.success('保存成功')
  visible.value = false
  load()
}
async function remove(row: any) {
  await ElMessageBox.confirm('确认删除该模板？', '提示', { type: 'warning' })
  await api.deleteTemplate(row.id)
  ElMessage.success('已删除')
  load()
}
onMounted(() => { loadMeta(); load() })
</script>
