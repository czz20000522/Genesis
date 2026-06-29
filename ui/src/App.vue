<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  compactSessionContext,
  enableSessionDebug,
  exportSessionDebug,
  getCapabilities,
  getReady,
  getTimeline,
  runtimeTokenFromStorage,
  saveRuntimeToken,
  submitTurn,
  uploadMaterial,
  type KernelTimeline,
} from './api/kernelApi'

const sessionId = ref('webui-local')
const token = ref(runtimeTokenFromStorage())
const draft = ref('')
const error = ref('')
const busy = ref(false)
const ready = ref<Record<string, unknown> | null>(null)
const capabilities = ref<Record<string, unknown> | null>(null)
const timeline = ref<KernelTimeline | null>(null)
const lastResult = ref<Record<string, unknown> | null>(null)

const canSend = computed(() => draft.value.trim().length > 0 && !busy.value)

onMounted(() => {
  void refreshRuntime()
})

async function run(action: () => Promise<Record<string, unknown> | KernelTimeline>) {
  busy.value = true
  error.value = ''
  saveRuntimeToken(token.value)
  try {
    const result = await action()
    lastResult.value = result as Record<string, unknown>
    return result
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : 'request failed'
    return null
  } finally {
    busy.value = false
  }
}

async function refreshRuntime() {
  await run(async () => {
    ready.value = await getReady()
    capabilities.value = await getCapabilities()
    timeline.value = await getTimeline(sessionId.value)
    return { status: 'ok' }
  })
}

async function sendTurn() {
  const text = draft.value.trim()
  if (!text) return
  const response = await run(() => submitTurn(sessionId.value, text))
  if (!response) return
  draft.value = ''
  timeline.value = await getTimeline(sessionId.value)
}

async function attachMaterial(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  await run(() => uploadMaterial(sessionId.value, file))
  input.value = ''
}

async function enableDebug() {
  await run(() => enableSessionDebug(sessionId.value))
}

async function exportDebug() {
  await run(() => exportSessionDebug(sessionId.value))
}

async function compactContext() {
  await run(() => compactSessionContext(sessionId.value))
}
</script>

<template>
  <main class="shell">
    <section class="workspace">
      <header class="toolbar">
        <div>
          <h1>Genesis</h1>
          <p>Vue/Vite thin shell over kernel projections.</p>
        </div>
        <button type="button" :disabled="busy" @click="refreshRuntime">刷新</button>
      </header>

      <section class="controls" aria-label="runtime controls">
        <label>
          Runtime token
          <input v-model="token" type="password" autocomplete="off" placeholder="Bearer token" />
        </label>
        <label>
          Session
          <input v-model="sessionId" autocomplete="off" />
        </label>
      </section>

      <section v-if="error" class="notice notice--error">{{ error }}</section>

      <section class="timeline" aria-label="session timeline">
        <article v-for="item in timeline?.items ?? []" :key="String(item.item_id ?? JSON.stringify(item))" class="timeline-item">
          <strong>{{ item.kind ?? item.item_kind ?? 'item' }}</strong>
          <pre>{{ item }}</pre>
        </article>
        <p v-if="!(timeline?.items ?? []).length" class="empty">暂无 timeline。发送一条消息或刷新当前 session。</p>
      </section>

      <form class="composer" @submit.prevent="sendTurn">
        <textarea v-model="draft" rows="4" placeholder="输入给 Genesis 的消息"></textarea>
        <div class="composer-actions">
          <input type="file" @change="attachMaterial" />
          <button type="button" :disabled="busy" @click="enableDebug">启用 debug</button>
          <button type="button" :disabled="busy" @click="exportDebug">导出 debug</button>
          <button type="button" :disabled="busy" @click="compactContext">压缩上下文</button>
          <button type="submit" :disabled="!canSend">发送</button>
        </div>
      </form>
    </section>

    <aside class="inspectors" aria-label="inspection panels">
      <h2>Readiness</h2>
      <pre>{{ ready }}</pre>
      <h2>Capabilities</h2>
      <pre>{{ capabilities }}</pre>
      <h2>Last result</h2>
      <pre>{{ lastResult }}</pre>
    </aside>
  </main>
</template>
