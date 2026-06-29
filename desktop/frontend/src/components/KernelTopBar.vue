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
      <p class="eyebrow">Genesis Desktop</p>
      <h1>Genesis</h1>
      <p class="status">kernel {{ readiness }} · {{ sessionId || 'no session' }}</p>
      <p v-if="error" class="error">{{ error }}</p>
    </div>
    <div class="topbar-actions">
      <button type="button" class="secondary-button" @click="$emit('checkReady')">Check kernel</button>
      <button type="button" class="secondary-button" @click="$emit('loadApprovals')">Approvals</button>
      <button type="button" @click="$emit('toggleInspector')">{{ inspectorOpen ? 'Hide inspector' : 'Inspector' }}</button>
    </div>
  </header>
</template>
