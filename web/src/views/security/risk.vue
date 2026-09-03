<template>
  <div class="page">
    <div class="search-bar">
      <el-button type="success" @click="openCreate">新增规则</el-button>
    </div>
    <el-table :data="list" border stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="pattern" label="匹配内容" />
      <el-table-column prop="matchType" label="模式" width="90">
        <template #default="{ row }"><el-tag>{{ matchMap[row.matchType] }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="riskLevel" label="风险" width="90">
        <template #default="{ row }"><el-tag :type="levelType(row.riskLevel)">{{ row.riskLevel }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="action" label="动作" width="90">
        <template #default="{ row }"><el-tag :type="row.action === 'block' ? 'danger' : 'warning'">{{ row.action === 'block' ? '拦截' : '告警' }}</el-tag></template>
      </el-table-column>
      <el-table-column label="启用" width="80">
        <template #default="{ row }"><el-switch :model-value="!!row.enabled" @change="(v:any)=>toggle(row, v)" /></template>
      </el-table-column>
      <el-table-column label="内置" width="70">
        <template #default="{ row }"><el-tag v-if="row.builtin" type="info">内置</el-tag><span v-else>-</span></template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" :disabled="!!row.builtin" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" :title="form.id ? '编辑规则' : '新增规则'" width="480px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="匹配内容"><el-input v-model="form.pattern" /></el-form-item>
        <el-form-item label="匹配模式">
          <el-select v-model="form.matchType" style="width:100%">
            <el-option label="精确" value="exact" />
            <el-option label="前缀" value="prefix" />
            <el-option label="正则" value="regex" />
          </el-select>
        </el-form-item>
        <el-form-item label="风险等级">
          <el-select v-model="form.riskLevel" style="width:100%">
            <el-option label="低" value="low" />
            <el-option label="中" value="medium" />
            <el-option label="高" value="high" />
            <el-option label="严重" value="critical" />
          </el-select>
        </el-form-item>
        <el-form-item label="动作">
          <el-select v-model="form.action" style="width:100%">
            <el-option label="拦截" value="block" />
            <el-option label="告警" value="warn" />
          </el-select>
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
const matchMap: any = { exact: '精确', prefix: '前缀', regex: '正则' }
const form = reactive<any>({ id: null, name: '', pattern: '', matchType: 'prefix', riskLevel: 'high', action: 'block', enabled: 1, remark: '' })

function levelType(l: string) {
  return { low: 'info', medium: 'warning', high: 'danger', critical: 'danger' }[l] || 'info'
}
async function load() { list.value = await api.listRisks() }
function openCreate() {
  Object.assign(form, { id: null, name: '', pattern: '', matchType: 'prefix', riskLevel: 'high', action: 'block', enabled: 1, remark: '' })
  visible.value = true
}
function openEdit(row: any) { Object.assign(form, { ...row }); visible.value = true }
async function submit() {
  if (form.id) await api.updateRisk(form.id, form)
  else await api.createRisk(form)
  ElMessage.success('保存成功')
  visible.value = false
  load()
}
async function toggle(row: any, v: any) {
  row.enabled = v ? 1 : 0
  await api.updateRisk(row.id, { ...row, enabled: row.enabled })
  ElMessage.success('已更新')
}
async function remove(row: any) {
  await ElMessageBox.confirm('确认删除该规则？', '提示', { type: 'warning' })
  await api.deleteRisk(row.id)
  ElMessage.success('已删除')
  load()
}
onMounted(load)
</script>
