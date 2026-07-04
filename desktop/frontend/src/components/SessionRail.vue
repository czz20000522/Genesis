<script setup lang="ts">
import { computed } from 'vue'
import type { SessionListItem } from '../api/kernelApi'
import { sessionLabel, sessionStatus } from '../display'

const props = defineProps<{
  sessionId: string
  sessions: SessionListItem[]
}>()

defineEmits<{
  newSession: []
  selectSession: [sessionId: string]
}>()

function titleFor(session: SessionListItem) {
  return String(session.title || session.session_id || '未命名会话').trim()
}

function subtitleFor(session: SessionListItem, currentSessionId: string) {
  const status = sessionStatus(String(session.session_id || ''), currentSessionId)
  const updated = String(session.updated_at || '').trim()
  return updated ? `${status} · ${updated.slice(0, 16).replace('T', ' ')}` : status
}

const hasCurrentSessionItem = computed(() => props.sessions.some((session) => session.session_id === props.sessionId))
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
        :key="session.session_id"
        type="button"
        class="session-link"
        :class="{ 'session-link-active': session.session_id === sessionId }"
        :title="titleFor(session)"
        @click="$emit('selectSession', String(session.session_id || ''))"
      >
        <span>{{ titleFor(session) }}</span>
        <small>{{ subtitleFor(session, sessionId) }}</small>
      </button>
      <button
        v-if="!hasCurrentSessionItem"
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
