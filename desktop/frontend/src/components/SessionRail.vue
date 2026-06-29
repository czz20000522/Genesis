<script setup lang="ts">
import { sessionLabel, sessionStatus } from '../display'

defineProps<{
  sessionId: string
  sessions: string[]
}>()

defineEmits<{
  newSession: []
  selectSession: [value: string]
  loadTimeline: []
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
        v-for="session in sessions"
        :key="session"
        type="button"
        :class="['session-link', { 'session-link-active': session === sessionId }]"
        :title="sessionLabel(session)"
        @click="$emit('selectSession', session)"
      >
        <span>{{ session === sessionId ? '当前会话' : '本地会话' }}</span>
        <small>{{ sessionStatus(session, sessionId) }} · 刚刚</small>
      </button>
    </nav>

    <div class="rail-footer">
      <button type="button" class="secondary-button" @click="$emit('loadTimeline')">重新载入</button>
    </div>
  </aside>
</template>
