<script setup lang="ts">
defineProps<{
  sessionId: string
  sessions: string[]
}>()

defineEmits<{
  newSession: []
  selectSession: [value: string]
  loadTimeline: []
}>()

function shortSession(value: string) {
  return value.length <= 22 ? value : `${value.slice(0, 14)}...${value.slice(-6)}`
}
</script>

<template>
  <aside class="rail">
    <div class="rail-brand">
      <div class="brand-mark">G</div>
      <div>
        <strong>Genesis</strong>
        <span>Desktop</span>
      </div>
    </div>

    <button type="button" class="new-session-button" @click="$emit('newSession')">New session</button>

    <nav class="session-list" aria-label="Sessions">
      <button
        v-for="session in sessions"
        :key="session"
        type="button"
        :class="['session-link', { 'session-link-active': session === sessionId }]"
        :title="session"
        @click="$emit('selectSession', session)"
      >
        <span>{{ shortSession(session) }}</span>
        <small>{{ session === sessionId ? 'current' : 'local' }}</small>
      </button>
    </nav>

    <div class="rail-footer">
      <button type="button" class="secondary-button" @click="$emit('loadTimeline')">Refresh timeline</button>
    </div>
  </aside>
</template>
