<script setup lang="ts">
defineProps<{
  readiness: string
  sessionId: string
  error: string
  inspectorOpen: boolean
}>()

defineEmits<{
  checkReady: []
  loadApprovals: []
  toggleInspector: []
}>()
</script>

<template>
  <header class="topbar">
    <div class="topbar-status">
      <strong>Genesis</strong>
      <span>连接 {{ readiness }}</span>
      <span>{{ sessionId || '未选择会话' }}</span>
      <p v-if="error" class="error">{{ error }}</p>
    </div>
    <div class="topbar-actions">
      <button type="button" @click="$emit('checkReady')">检查连接</button>
      <button type="button" @click="$emit('loadApprovals')">审批</button>
      <button type="button" @click="$emit('toggleInspector')">{{ inspectorOpen ? '收起详情' : '详情' }}</button>
    </div>
  </header>
</template>
