<template>
  <div class="page">
    <div class="search-bar">
      <el-input v-model="q.keyword" placeholder="IP/用户名" style="width:200px" @keyup.enter="load" />
      <el-button type="primary" @click="load">查询</el-button>
    </div>
    <el-table :data="list" border stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" label="用户" width="120" />
      <el-table-column prop="hostName" label="主机" />
      <el-table-column prop="hostIp" label="IP" />
      <el-table-column prop="protocol" label="协议" width="80" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }"><el-tag :type="row.status ? 'success' : 'info'">{{ row.status ? '进行中' : '已结束' }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="duration" label="时长(秒)" width="100" />
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button size="small" type="primary" :disabled="row.status || !row.recordPath" @click="replay(row)">回放</el-button>
          <el-button size="small" type="danger" :disabled="!row.status" @click="terminate(row)">终止</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination class="pager" v-model:current-page="q.page" :page-size="q.pageSize" :total="total" @current-change="load" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/api'

const list = ref<any[]>([])
const total = ref(0)
const loading = ref(false)
const router = useRouter()
const q = reactive({ page: 1, pageSize: 10, keyword: '' })

async function load() {
  loading.value = true
  const r = await api.listSessions({ ...q })
  list.value = r.list; total.value = r.total
  loading.value = false
}
async function terminate(row: any) {
  await ElMessageBox.confirm('确认强制终止该会话？', '提示', { type: 'warning' })
  await api.terminateSession(row.id)
  ElMessage.success('已终止')
  load()
}
function replay(row: any) {
  router.push('/ops/replay/' + row.id)
}
onMounted(load)
</script>

<style scoped>
.pager { margin-top: 12px; display: flex; justify-content: flex-end; }
</style>
