<script setup lang="ts">
import { computed } from 'vue'
import type { KernelTimelineDetail } from '../api/kernelApi'

const props = defineProps<{
  baseUrl: string
  runtimeToken: string
  readiness: string
  detail: KernelTimelineDetail | null
  selectedDetailRef: string
  materialSummary: string[]
  debugSummaryRows: string[]
  compactionSummaryRows: string[]
  debugExportReady: boolean
}>()

defineEmits<{
  'update:baseUrl': [value: string]
  'update:runtimeToken': [value: string]
  'update:selectedDetailRef': [value: string]
  checkReady: []
  loadDetail: []
  enableDebug: []
  exportDebug: []
  downloadDebug: []
  compactContext: []
  close: []
}>()

const detailItem = computed(() => props.detail?.item ?? {})

function detailField(name: string) {
  return String(detailItem.value[name] ?? '').trim()
}
</script>

<template>
  <aside class="inspector">
    <div class="inspector-head">
      <div>
        <p class="eyebrow">Inspector</p>
        <strong>{{ readiness }}</strong>
      </div>
      <button type="button" class="secondary-button" @click="$emit('close')">Close</button>
    </div>

    <section class="panel">
      <p class="eyebrow">Settings</p>
      <label>
        Kernel URL
        <input :value="baseUrl" spellcheck="false" @input="$emit('update:baseUrl', ($event.target as HTMLInputElement).value)" />
      </label>
      <label>
        Runtime token
        <input :value="runtimeToken" type="password" spellcheck="false" @input="$emit('update:runtimeToken', ($event.target as HTMLInputElement).value)" />
      </label>
      <button type="button" class="secondary-button" @click="$emit('checkReady')">Check kernel</button>
    </section>

    <section class="panel">
      <p class="eyebrow">Session diagnostics</p>
      <label>
        Detail ref
        <input :value="selectedDetailRef" spellcheck="false" @input="$emit('update:selectedDetailRef', ($event.target as HTMLInputElement).value)" />
      </label>
      <div class="button-row">
        <button type="button" class="secondary-button" @click="$emit('loadDetail')">Load detail</button>
        <button type="button" class="secondary-button" @click="$emit('enableDebug')">Enable debug</button>
        <button type="button" class="secondary-button" @click="$emit('exportDebug')">Export debug</button>
        <button type="button" class="secondary-button" :disabled="!debugExportReady" @click="$emit('downloadDebug')">Download</button>
        <button type="button" class="secondary-button" @click="$emit('compactContext')">Compact</button>
      </div>
    </section>

    <section v-if="detail" class="detail-panel">
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
    </section>

    <section v-if="materialSummary.length" class="detail-panel">
      <p class="eyebrow">Material intake</p>
      <dl>
        <dt>Admission</dt>
        <dd>{{ materialSummary[0] }}</dd>
        <dt>Source/refusal</dt>
        <dd><code>{{ materialSummary[1] }}</code></dd>
        <dt>Operations</dt>
        <dd>{{ materialSummary[2] }}</dd>
      </dl>
    </section>

    <section v-if="compactionSummaryRows.length" class="detail-panel">
      <p class="eyebrow">Context compaction</p>
      <dl>
        <dt>Admission</dt>
        <dd>{{ compactionSummaryRows[0] }}</dd>
        <dt>Reason</dt>
        <dd>{{ compactionSummaryRows[1] || 'none' }}</dd>
      </dl>
    </section>

    <section v-if="debugSummaryRows.length" class="detail-panel">
      <p class="eyebrow">Session debug</p>
      <dl>
        <dt>Readiness</dt>
        <dd>{{ debugSummaryRows[0] }}</dd>
        <dt>Steps</dt>
        <dd>{{ debugSummaryRows[1] }}</dd>
        <dt>Input kinds</dt>
        <dd>{{ debugSummaryRows[2] }}</dd>
        <dt>Models</dt>
        <dd>{{ debugSummaryRows[3] }}</dd>
      </dl>
    </section>
  </aside>
</template>
