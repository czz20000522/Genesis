<script setup lang="ts">
import { sessionLabel } from '../display'

defineProps<{
  sessionId: string
}>()

defineEmits<{
  newSession: []
}>()

</script>

<template>
  <aside class="rail">
    <div class="rail-brand">
      <div class="brand-mark">G</div>
      <div>
        <strong>Genesis</strong>
        <span>桌面端</span>
      </div>
    </div>

    <button type="button" class="new-session-button" @click="$emit('newSession')">新建会话</button>

    <nav class="session-list" aria-label="会话">
      <button
        type="button"
        class="session-link session-link-active"
        :title="sessionLabel(sessionId)"
      >
        <span>当前对话</span>
        <small>{{ sessionLabel(sessionId) }}</small>
      </button>
    </nav>
  </aside>
</template>
