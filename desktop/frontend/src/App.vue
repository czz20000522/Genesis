<script setup lang="ts">
import { computed, ref } from 'vue'
import { getReady, getTimeline, getTimelineDetail, kernelConfig, saveKernelConfig, type KernelTimeline, type KernelTimelineDetail } from './api/kernelApi'
import { timelineDetailEntries } from './timelineDetail'

const config = ref(kernelConfig())
const readiness = ref('unchecked')
const error = ref('')
const sessionId = ref('')
const selectedDetailRef = ref('')
const timeline = ref<KernelTimeline | null>(null)
const detail = ref<KernelTimelineDetail | null>(null)
const detailEntries = computed(() => timelineDetailEntries(timeline.value?.items))
const detailItem = computed(() => detail.value?.item ?? {})

async function checkReady() {
  error.value = ''
  saveKernelConfig(config.value)
  try {
    const payload = await getReady(config.value)
    readiness.value = String(payload.readiness ?? payload.status ?? 'unknown')
  } catch (err) {
    readiness.value = 'not_ready'
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function loadTimeline() {
  error.value = ''
  saveKernelConfig(config.value)
  detail.value = null
  try {
    timeline.value = await getTimeline(config.value, sessionId.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function loadDetail(detailRef = selectedDetailRef.value) {
  error.value = ''
  saveKernelConfig(config.value)
  selectedDetailRef.value = detailRef
  try {
    detail.value = await getTimelineDetail(config.value, sessionId.value, detailRef)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

function detailField(name: string) {
  return String(detailItem.value[name] ?? '').trim()
}
</script>

<template>
  <main>
    <section class="shell">
      <header>
        <p class="eyebrow">Genesis Desktop</p>
        <h1>Local kernel shell</h1>
      </header>

      <label>
        Kernel URL
        <input v-model="config.baseUrl" spellcheck="false" />
      </label>

      <label>
        Runtime token
        <input v-model="config.runtimeToken" type="password" spellcheck="false" />
      </label>

      <button type="button" @click="checkReady">Check kernel</button>

      <p class="status">readiness: {{ readiness }}</p>
      <p v-if="error" class="error">{{ error }}</p>

      <div class="divider"></div>

      <label>
        Session ID
        <input v-model="sessionId" spellcheck="false" />
      </label>

      <button type="button" @click="loadTimeline">Load timeline</button>

      <div v-if="detailEntries.length" class="detail-list">
        <button v-for="entry in detailEntries" :key="entry.detailRef" type="button" @click="loadDetail(entry.detailRef)">
          {{ entry.label }}
        </button>
      </div>

      <label>
        Detail ref
        <input v-model="selectedDetailRef" spellcheck="false" />
      </label>

      <button type="button" @click="loadDetail()">Load detail</button>

      <aside v-if="detail" class="detail-panel">
        <p class="eyebrow">Timeline detail</p>
        <h2>{{ detailField('kind') || 'detail' }}</h2>
        <dl>
          <template v-if="detailField('tool')">
            <dt>Tool</dt>
            <dd>{{ detailField('tool') }}</dd>
          </template>
          <template v-if="detailField('command_preview')">
            <dt>Command</dt>
            <dd><code>{{ detailField('command_preview') }}</code></dd>
          </template>
          <template v-if="detailField('duration_ms')">
            <dt>Duration</dt>
            <dd>{{ detailField('duration_ms') }} ms</dd>
          </template>
          <template v-if="detailField('output_truncation')">
            <dt>Truncation</dt>
            <dd>{{ detailField('output_truncation') }}</dd>
          </template>
          <template v-if="detailField('visible_output') || detailField('output_preview')">
            <dt>Output</dt>
            <dd><pre>{{ detailField('visible_output') || detailField('output_preview') }}</pre></dd>
          </template>
        </dl>
      </aside>
    </section>
  </main>
</template>
