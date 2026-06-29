<script setup lang="ts">
import { connectionErrorLabel, readinessLabel, sessionLabel } from '../display'

defineProps<{
  readiness: string
  sessionId: string
  error: string
  inspectorOpen: boolean
}>()

defineEmits<{
  checkReady: []
  toggleInspector: []
}>()
</script>

<template>
  <header class="topbar">
    <div class="topbar-status">
      <strong>Genesis</strong>
      <span>{{ readinessLabel(readiness) }}</span>
      <span>{{ sessionLabel(sessionId) }}</span>
      <p v-if="error" class="error">{{ connectionErrorLabel(error) }}</p>
    </div>
    <div class="topbar-actions">
      <button type="button" @click="$emit('checkReady')">检查连接</button>
      <button type="button" @click="$emit('toggleInspector')">{{ inspectorOpen ? '收起设置' : '设置' }}</button>
    </div>
  </header>
</template>
