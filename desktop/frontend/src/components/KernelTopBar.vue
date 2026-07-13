<script setup lang="ts">
import { readinessLabel } from '../display'

defineProps<{
  readiness: string
  sessionId: string
  inspectorOpen: boolean
}>()

defineEmits<{
  checkReady: []
  toggleInspector: []
  toggleProvider: []
}>()
</script>

<template>
  <header class="topbar">
    <div class="topbar-status">
      <strong>Genesis</strong>
    </div>
    <div class="topbar-actions">
      <el-tooltip :content="readinessLabel(readiness)">
        <el-button circle text class="connection-indicator" :aria-label="readinessLabel(readiness)" @click="$emit('checkReady')"><span :class="`connection-dot connection-dot--${readiness}`" /></el-button>
      </el-tooltip>
      <el-button text @click="$emit('toggleProvider')">模型</el-button>
      <el-button text @click="$emit('toggleInspector')">{{ inspectorOpen ? '收起设置' : '设置' }}</el-button>
    </div>
  </header>
</template>
