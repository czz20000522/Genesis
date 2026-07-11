<script setup lang="ts">
import { computed } from 'vue'
import type { SessionListItem } from '../api/kernelApi'
import type { DesktopSessionCatalogEntry } from '../sessionCatalog'
import { sessionLabel, sessionStatus } from '../display'

const props = defineProps<{
  sessionId: string
  sessions: SessionListItem[]
  catalog: DesktopSessionCatalogEntry[]
  searchQuery: string
  searchResults: SessionListItem[]
}>()

defineEmits<{
  newProject: []
  newProjectSession: [root: string]
  newTask: []
  newChat: []
  selectSession: [sessionId: string]
  'update:searchQuery': [value: string]
}>()

type ProjectGroup = {
  root: string
  name: string
  sessions: SessionListItem[]
}

function titleFor(session: SessionListItem) {
  return String(session.title || session.session_id || '未命名会话').trim()
}

function subtitleFor(session: SessionListItem, currentSessionId: string) {
  const status = sessionStatus(String(session.session_id || ''), currentSessionId)
  const updated = String(session.updated_at || '').trim()
  return updated ? `${status} · ${updated.slice(0, 16).replace('T', ' ')}` : status
}

const catalogBySession = computed(() => new Map(props.catalog.map((entry) => [entry.sessionId, entry])))
const projectGroups = computed<ProjectGroup[]>(() => {
  const groups = new Map<string, ProjectGroup>()
  for (const session of props.sessions) {
    const entry = catalogBySession.value.get(String(session.session_id || ''))
    if (!entry || entry.kind !== 'project' || !entry.root) continue
    const root = entry.root
    const group = groups.get(root) ?? { root, name: entry.name || root.split(/[\\/]/).filter(Boolean).at(-1) || '项目', sessions: [] }
    group.sessions.push(session)
    groups.set(root, group)
  }
  return [...groups.values()]
})
const taskSessions = computed(() => props.sessions.filter((session) => catalogBySession.value.get(String(session.session_id || ''))?.kind === 'task'))
const chatSessions = computed(() => props.sessions.filter((session) => catalogBySession.value.get(String(session.session_id || ''))?.kind === 'chat'))
const otherSessions = computed(() => props.sessions.filter((session) => !catalogBySession.value.has(String(session.session_id || ''))))
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

    <div class="session-entry-actions" aria-label="创建入口">
      <button type="button" class="new-session-button" @click="$emit('newProject')">项目</button>
      <button type="button" class="new-session-button" @click="$emit('newTask')">任务</button>
      <button type="button" class="new-session-button" @click="$emit('newChat')">聊天</button>
    </div>

    <label class="session-search">
      搜索会话
      <input :value="searchQuery" placeholder="搜索会话…" @input="$emit('update:searchQuery', ($event.target as HTMLInputElement).value)" />
    </label>

    <nav class="session-list" aria-label="会话">
      <section v-if="searchQuery.trim()" class="session-group">
        <div class="session-group-heading"><strong>搜索结果</strong></div>
        <button v-for="session in searchResults" :key="session.session_id" type="button" class="session-link" :class="{ 'session-link-active': session.session_id === sessionId }" :title="titleFor(session)" @click="$emit('selectSession', String(session.session_id || ''))">
          <span>{{ titleFor(session) }}</span>
          <small>{{ subtitleFor(session, sessionId) }}</small>
        </button>
        <p v-if="!searchResults.length" class="session-search-empty">没有匹配的会话</p>
      </section>
      <template v-else>
      <section v-for="group in projectGroups" :key="group.root" class="session-group">
        <div class="session-group-heading">
          <strong :title="group.root">{{ group.name }}</strong>
          <button type="button" :aria-label="`在 ${group.name} 中创建会话`" @click="$emit('newProjectSession', group.root)">+</button>
        </div>
        <button
          v-for="session in group.sessions"
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
      </section>

      <section v-if="taskSessions.length" class="session-group">
        <div class="session-group-heading"><strong>任务</strong></div>
        <button v-for="session in taskSessions" :key="session.session_id" type="button" class="session-link" :class="{ 'session-link-active': session.session_id === sessionId }" :title="titleFor(session)" @click="$emit('selectSession', String(session.session_id || ''))">
          <span>{{ titleFor(session) }}</span>
          <small>{{ subtitleFor(session, sessionId) }}</small>
        </button>
      </section>

      <section v-if="chatSessions.length" class="session-group">
        <div class="session-group-heading"><strong>聊天</strong></div>
        <button v-for="session in chatSessions" :key="session.session_id" type="button" class="session-link" :class="{ 'session-link-active': session.session_id === sessionId }" :title="titleFor(session)" @click="$emit('selectSession', String(session.session_id || ''))">
          <span>{{ titleFor(session) }}</span>
          <small>{{ subtitleFor(session, sessionId) }}</small>
        </button>
      </section>

      <section v-if="otherSessions.length" class="session-group">
        <div class="session-group-heading"><strong>其他会话</strong></div>
        <button v-for="session in otherSessions" :key="session.session_id" type="button" class="session-link" :class="{ 'session-link-active': session.session_id === sessionId }" :title="titleFor(session)" @click="$emit('selectSession', String(session.session_id || ''))">
          <span>{{ titleFor(session) }}</span>
          <small>{{ subtitleFor(session, sessionId) }}</small>
        </button>
      </section>

      <button v-if="sessionId && !hasCurrentSessionItem" type="button" class="session-link session-link-active" :title="sessionLabel(sessionId)">
        <span>当前对话</span>
        <small>{{ sessionLabel(sessionId) }}</small>
      </button>
      </template>
    </nav>
  </aside>
</template>
