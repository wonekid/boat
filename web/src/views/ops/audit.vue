<template>
  <div class="page">
    <div class="search-bar">
      <el-input v-model="q.keyword" placeholder="用户/动作" style="width:200px" @keyup.enter="load" />
      <el-button type="primary" @click="load">查询</el-button>
    </div>
    <el-table :data="list" border stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" label="用户" width="120" />
      <el-table-column prop="action" label="动作" width="140" />
      <el-table-column prop="module" label="模块" width="100" />
      <el-table-column prop="ip" label="IP" width="130" />
      <el-table-column label="结果" width="90">
        <template #default="{ row }"><el-tag :type="row.status ? 'success' : 'danger'">{{ row.status ? '成功' : '失败' }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="detail" label="详情" />
      <el-table-column prop="createdAt" label="时间" width="180" />
    </el-table>
    <el-pagination class="pager" v-model:current-page="q.page" :page-size="q.pageSize" :total="total" @current-change="load" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import api from '@/api'

const list = ref<any[]>([])
const total = ref(0)
const loading = ref(false)
const q = reactive({ page: 1, pageSize: 10, keyword: '' })

async function load() {
  loading.value = true
  const r = await api.listAudits({ ...q })
  list.value = r.list; total.value = r.total
  loading.value = false
}
onMounted(load)
</script>

<style scoped>
.pager { margin-top: 12px; display: flex; justify-content: flex-end; }
</style>
