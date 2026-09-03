<template>
  <div class="page replay-page">
    <div class="toolbar">
      <span class="title">会话录像回放</span>
      <el-tag v-if="meta" type="info">#{{ id }} · {{ meta.username }}@{{ meta.hostIp }}</el-tag>
      <div class="spacer" />
      <el-button :disabled="!events.length" @click="toggle">{{ playing ? '暂停' : '继续' }}</el-button>
      <el-button :disabled="!events.length" @click="replay">重新播放</el-button>
      <el-select v-model="speed" style="width:110px" @change="onSpeed">
        <el-option v-for="s in [1, 2, 4, 8]" :key="s" :label="s + 'x'" :value="s" />
      </el-select>
    </div>
    <div ref="termRef" class="term"></div>
    <div v-if="error" class="err">{{ error }}</div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Terminal } from 'xterm'
import 'xterm/css/xterm.css'
import api from '@/api'

const route = useRoute()
const id = route.params.id as string
const termRef = ref<HTMLElement>()
const meta = ref<any>(null)
const error = ref('')
const playing = ref(false)
const speed = ref(2)

let term: Terminal
let events: any[] = []
let idx = 0
let timer: number | null = null

onMounted(async () => {
  term = new Terminal({ fontSize: 14, cursorBlink: false, theme: { background: '#1e1e1e' } })
  term.open(termRef.value as HTMLElement)
  await load()
})
onBeforeUnmount(() => {
  stopTimer()
  if (term) term.dispose()
})

async function load() {
  try {
    const text: string = await api.getSessionRecording(id)
    const lines = text.split('\n').map((l) => l.trim()).filter(Boolean)
    if (!lines.length) {
      error.value = '录像内容为空'
      return
    }
    try {
      meta.value = JSON.parse(lines[0])
    } catch {
      error.value = '录像格式无法识别'
      return
    }
    const w = meta.value.width || 80
    const h = meta.value.height || 24
    term.resize(w, h)
    events = []
    for (const ln of lines.slice(1)) {
      try {
        const e = JSON.parse(ln)
        if (Array.isArray(e) && e.length >= 3) events.push(e)
      } catch { /* 跳过异常帧 */ }
    }
    if (!events.length) {
      error.value = '录像无有效帧'
      return
    }
    replay()
  } catch (e: any) {
    error.value = '获取录像失败：' + (e?.message || e)
  }
}

function stopTimer() {
  if (timer !== null) {
    clearTimeout(timer)
    timer = null
  }
}

function replay() {
  stopTimer()
  term.reset()
  idx = 0
  playing.value = true
  scheduleNext()
}

function scheduleNext() {
  if (!playing.value || idx >= events.length) {
    if (idx >= events.length) playing.value = false
    return
  }
  const ev = events[idx]
  const delay = (Number(ev[0]) || 0) * 1000 / speed.value
  timer = window.setTimeout(() => {
    const data = ev[2]
    if (typeof data === 'string') term.write(data)
    idx++
    scheduleNext()
  }, Math.max(delay, 0))
}

function toggle() {
  if (playing.value) {
    playing.value = false
    stopTimer()
  } else {
    if (idx >= events.length) {
      replay()
    } else {
      playing.value = true
      scheduleNext()
    }
  }
}

function onSpeed() {
  // 速度变更即时生效：若正在播放，从下一帧起按新速度
  if (playing.value && timer === null) scheduleNext()
}
</script>

<style scoped>
.replay-page { display: flex; flex-direction: column; height: 100%; }
.toolbar { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.toolbar .title { font-weight: 600; }
.toolbar .spacer { flex: 1; }
.term { flex: 1; background: #1e1e1e; padding: 8px; border-radius: 4px; min-height: 420px; overflow: auto; }
.err { color: #f56c6c; margin-top: 12px; }
</style>
