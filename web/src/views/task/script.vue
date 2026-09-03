<template>
  <div class="page">
    <div class="search-bar">
      <el-input v-model="q.keyword" placeholder="名称" style="width:200px" @keyup.enter="load" />
      <el-button type="primary" @click="load">查询</el-button>
      <el-button type="success" @click="openCreate">新增脚本</el-button>
      <el-upload
        :show-file-list="false"
        accept=".sh,.bash,.py,.txt,.ps1"
        :before-upload="beforeUpload"
        :http-request="doUpload"
      >
        <el-button type="primary" icon="Upload">上传脚本</el-button>
      </el-upload>
    </div>
    <el-table :data="list" border stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="lang" label="语言" width="90">
        <template #default="{ row }"><el-tag>{{ row.lang === 'python' ? 'Python' : 'Shell' }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" />
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" :title="form.id ? '编辑脚本' : '新增脚本'" width="560px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="语言">
          <el-select v-model="form.lang" style="width:100%">
            <el-option label="Shell" value="shell" />
            <el-option label="Python" value="python" />
          </el-select>
        </el-form-item>
        <el-form-item label="内容">
          <el-input v-model="form.content" type="textarea" :rows="8" placeholder="#!/bin/bash" />
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
const q = reactive({ keyword: '' })
const form = reactive<any>({ id: null, name: '', lang: 'shell', content: '', remark: '' })

async function load() { list.value = await api.listScripts({ ...q }) }
function openCreate() {
  Object.assign(form, { id: null, name: '', lang: 'shell', content: '', remark: '' })
  visible.value = true
}
function openEdit(row: any) { Object.assign(form, { ...row }); visible.value = true }
async function submit() {
  if (form.id) await api.updateScript(form.id, form)
  else await api.createScript(form)
  ElMessage.success('保存成功')
  visible.value = false
  load()
}
async function remove(row: any) {
  await ElMessageBox.confirm('确认删除该脚本？', '提示', { type: 'warning' })
  await api.deleteScript(row.id)
  ElMessage.success('已删除')
  load()
}
// 上传脚本：校验大小后通过自定义请求提交，后端按扩展名推断语言并写入脚本库
function beforeUpload(file: any) {
  if (file.size > 1024 * 1024) {
    ElMessage.warning('脚本文件过大（上限 1MB）')
    return false
  }
  return true
}
async function doUpload(options: any) {
  const fd = new FormData()
  fd.append('file', options.file)
  try {
    await api.uploadScript(fd)
    ElMessage.success('上传成功，已写入脚本库')
    load()
    options.onSuccess()
  } catch (e) {
    options.onError(e)
  }
}
onMounted(load)
</script>
