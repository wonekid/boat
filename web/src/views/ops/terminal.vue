<template>
  <div class="page terminal-page">
    <div class="toolbar">
      <el-select v-model="hostId" placeholder="选择主机" filterable style="width: 280px" :disabled="connected">
        <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} (${h.ip})`" :value="h.id" />
      </el-select>
      <el-button type="primary" :disabled="!hostId || connected" @click="connect">连接</el-button>
      <el-button type="danger" :disabled="!connected" @click="disconnect">断开</el-button>
      <span class="status" :class="{ on: connected }">{{ connected ? '已连接' : '未连接' }}</span>
    </div>
    <div ref="termRef" class="term"></div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'
import api from '@/api'

const hosts = ref<any[]>([])
const hostId = ref<number>()
const connected = ref(false)
const termRef = ref<HTMLElement>()
let term: Terminal
let fit: FitAddon
let ws: WebSocket | null = null

onMounted(async () => {
  term = new Terminal({ fontSize: 14, cursorBlink: true, theme: { background: '#1e1e1e' } })
  fit = new FitAddon()
  term.open(termRef.value as HTMLElement)
  fit.fit()
  term.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(data)
    }
  })
  window.addEventListener('resize', onResize)
  try {
    const r = await api.listHosts({ page: 1, pageSize: 200 })
    hosts.value = r.list || []
  } catch (e) { /* ignore */ }
})

function onResize() {
  if (!fit) return
  fit.fit()
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
  }
}

function connect() {
  if (!hostId.value) return
  const token = localStorage.getItem('boat_token')
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const url = `${proto}://${location.host}/api/ws/terminal?hostId=${hostId.value}&token=${token}&cols=${term.cols}&rows=${term.rows}`
  ws = new WebSocket(url)
  // 后端终端数据以二进制帧下发，必须显式声明 binaryType，否则 ev.data 为 Blob 导致 term.write 静默失败
  ws.binaryType = 'arraybuffer'
  ws.onopen = () => {
    connected.value = true
    term.clear()
  }
  ws.onmessage = (ev) => {
    if (typeof ev.data === 'string') {
      // 控制消息 / 错误文本
      term.write(ev.data)
    } else {
      // 终端字节流（ArrayBuffer -> UTF-8 文本）
      term.write(new TextDecoder('utf-8').decode(ev.data as ArrayBuffer))
    }
  }
  ws.onclose = () => {
    connected.value = false
    term.write('\r\n\x1b[33m[连接已关闭]\x1b[0m\r\n')
  }
  ws.onerror = () => {
    term.write('\r\n\x1b[31m[连接错误]\x1b[0m\r\n')
  }
}

function disconnect() {
  if (ws) {
    ws.close()
    ws = null
  }
  connected.value = false
}

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  disconnect()
  if (term) term.dispose()
})
</script>

<style scoped>
.terminal-page { display: flex; flex-direction: column; height: 100%; }
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; }
.status { color: #909399; }
.status.on { color: #67c23a; }
.term { flex: 1; background: #1e1e1e; padding: 8px; border-radius: 4px; min-height: 420px; }
</style>
