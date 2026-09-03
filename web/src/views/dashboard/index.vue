<template>
  <div class="page">
    <el-row :gutter="16">
      <el-col :span="4" v-for="c in cards" :key="c.label">
        <el-card shadow="hover" class="stat">
          <div class="num">{{ c.value }}</div>
          <div class="label">{{ c.label }}</div>
        </el-card>
      </el-col>
    </el-row>
    <el-card class="chart-card" shadow="never">
      <div class="chart-title">近 7 天操作审计趋势</div>
      <div ref="chartRef" class="chart"></div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, nextTick } from 'vue'
import * as echarts from 'echarts'
import api from '@/api'

const cards = ref<any[]>([])
const chartRef = ref<HTMLElement>()

onMounted(async () => {
  const d = await api.dashboard()
  const s = d.stats
  cards.value = [
    { label: '主机总数', value: s.hostTotal },
    { label: '在线主机', value: s.hostOnline },
    { label: '用户总数', value: s.userTotal },
    { label: '会话总数', value: s.sessionTotal },
    { label: '审计记录', value: s.auditTotal },
    { label: '启用高危规则', value: s.riskTotal },
  ]
  await nextTick()
  renderChart(d.trend)
})

function renderChart(trend: any[]) {
  if (!chartRef.value) return
  const chart = echarts.init(chartRef.value)
  chart.setOption({
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'category', data: trend.map((t) => t.date) },
    yAxis: { type: 'value' },
    series: [{ name: '操作次数', type: 'line', smooth: true, data: trend.map((t) => t.count), areaStyle: {} }],
  })
  window.addEventListener('resize', () => chart.resize())
}
</script>

<style scoped>
.stat { text-align: center; }
.num { font-size: 28px; font-weight: 700; color: #304ffe; }
.label { color: #909399; margin-top: 4px; }
.chart-card { margin-top: 16px; }
.chart-title { font-weight: 600; margin-bottom: 12px; }
.chart { height: 320px; }
</style>
