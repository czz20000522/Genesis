<script setup lang="ts">
import { connectionErrorLabel, readinessLabel, sessionLabel } from '../display'

defineProps<{
  readiness: string
  sessionId: string
  error: string
  inspectorOpen: boolean
  localModel: string
  localModelRunning: boolean
  localModelStarting: boolean
  providerSummary: string
}>()

defineEmits<{
  checkReady: []
  toggleInspector: []
  toggleLocalModel: []
  toggleProvider: []
}>()
</script>

<template>
  <header class="topbar">
    <div class="topbar-status">
      <strong>Genesis</strong>
      <span>{{ readinessLabel(readiness) }}</span>
      <span>{{ sessionLabel(sessionId) }}</span>
      <span class="local-model-status">{{ localModel }}</span>
      <span class="provider-status">{{ providerSummary }}</span>
      <p v-if="error" class="error">{{ connectionErrorLabel(error) }}</p>
    </div>
    <div class="topbar-actions">
      <button type="button" @click="$emit('checkReady')">检查连接</button>
      <button type="button" class="secondary-button" :disabled="localModelStarting" @click="$emit('toggleLocalModel')">{{ localModelStarting ? '正在启动…' : localModelRunning ? '停止本地模型' : '启动本地模型' }}</button>
      <button type="button" class="secondary-button" @click="$emit('toggleProvider')">模型</button>
      <button type="button" @click="$emit('toggleInspector')">{{ inspectorOpen ? '收起设置' : '设置' }}</button>
    </div>
  </header>
</template>
