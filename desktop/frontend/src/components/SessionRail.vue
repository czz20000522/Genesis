<script setup lang="ts">
defineProps<{
  sessionId: string
  debugExportReady: boolean
}>()

defineEmits<{
  'update:sessionId': [value: string]
  loadTimeline: []
  selectMaterial: [event: Event]
  uploadMaterial: []
  enableDebug: []
  exportDebug: []
  downloadDebug: []
  compactContext: []
}>()
</script>

<template>
  <aside class="rail">
    <section class="panel">
      <p class="eyebrow">Session</p>
      <label>
        Session ID
        <input :value="sessionId" spellcheck="false" @input="$emit('update:sessionId', ($event.target as HTMLInputElement).value)" />
      </label>
      <button type="button" @click="$emit('loadTimeline')">Load timeline</button>
    </section>

    <section class="panel">
      <p class="eyebrow">Materials</p>
      <label>
        Material zip
        <input type="file" accept=".zip,application/zip,application/x-zip-compressed" @change="$emit('selectMaterial', $event)" />
      </label>
      <button type="button" @click="$emit('uploadMaterial')">Upload material</button>
    </section>

    <section class="panel">
      <p class="eyebrow">Session controls</p>
      <div class="detail-list">
        <button type="button" @click="$emit('enableDebug')">Enable debug</button>
        <button type="button" @click="$emit('exportDebug')">Export debug</button>
        <button type="button" :disabled="!debugExportReady" @click="$emit('downloadDebug')">Download debug JSON</button>
        <button type="button" @click="$emit('compactContext')">Compact context</button>
      </div>
    </section>
  </aside>
</template>
