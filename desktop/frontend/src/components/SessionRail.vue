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
        <span>桌面端</span>
      </div>
    </div>

    <button type="button" class="new-session-button" @click="$emit('newSession')">新建会话</button>

    <nav class="session-list" aria-label="会话">
      <button
        v-for="session in sessions"
        :key="session"
        type="button"
        :class="['session-link', { 'session-link-active': session === sessionId }]"
        :title="session"
        @click="$emit('selectSession', session)"
      >
        <span>{{ shortSession(session) }}</span>
        <small>{{ session === sessionId ? '当前' : '本地' }}</small>
      </button>
    </nav>

    <div class="rail-footer">
      <button type="button" class="secondary-button" @click="$emit('loadTimeline')">刷新时间线</button>
    </div>
  </aside>
</template>
