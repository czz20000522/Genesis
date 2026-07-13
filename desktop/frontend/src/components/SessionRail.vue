<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChatDotRound, EditPen, Folder, FolderAdd, Plus, Search } from '@element-plus/icons-vue'
import type { SessionListItem } from '../api/kernelApi'
import type { DesktopProjectCatalogEntry, DesktopSessionCatalogEntry } from '../sessionCatalog'
import { sessionLabel, sessionStatus } from '../display'

const props = defineProps<{
  sessionId: string
  sessions: SessionListItem[]
  projects: DesktopProjectCatalogEntry[]
  catalog: DesktopSessionCatalogEntry[]
  searchQuery: string
  searchResults: SessionListItem[]
}>()

type ProjectGroup = {
  project: DesktopProjectCatalogEntry
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

const projectMenuOpen = ref(false)
const projectName = ref('')
const searchOpen = ref(false)
const catalogBySession = computed(() => new Map(props.catalog.map((entry) => [entry.sessionId, entry])))
const projectGroups = computed<ProjectGroup[]>(() => {
  return props.projects.map((project) => ({
    project,
    sessions: props.sessions.filter((session) => catalogBySession.value.get(String(session.session_id || ''))?.projectId === project.projectId),
  }))
})
const taskSessions = computed(() => props.sessions.filter((session) => catalogBySession.value.get(String(session.session_id || ''))?.kind === 'task'))
const chatSessions = computed(() => props.sessions.filter((session) => catalogBySession.value.get(String(session.session_id || ''))?.kind === 'chat'))
const otherSessions = computed(() => props.sessions.filter((session) => !catalogBySession.value.has(String(session.session_id || ''))))
const hasCurrentSessionItem = computed(() => props.sessions.some((session) => session.session_id === props.sessionId))

function createEmptyProject() {
  const name = projectName.value.trim()
  if (!name) return
  projectName.value = ''
  projectMenuOpen.value = false
  emit('createEmptyProject', name)
}

const emit = defineEmits<{
  createEmptyProject: [name: string]
  useExistingProjectFolder: []
  newProjectSession: [project: DesktopProjectCatalogEntry]
  newTask: []
  newChat: []
  selectSession: [sessionId: string]
  'update:searchQuery': [value: string]
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

    <nav class="rail-primary" aria-label="工作台入口">
      <el-button text class="rail-primary-action" @click="$emit('newTask')"><el-icon><EditPen /></el-icon> 新建任务</el-button>
      <el-button text class="rail-primary-action" @click="$emit('newChat')"><el-icon><ChatDotRound /></el-icon> 聊天</el-button>
      <el-button text class="rail-primary-action" @click="searchOpen = !searchOpen"><el-icon><Search /></el-icon> 搜索</el-button>
    </nav>

    <div class="rail-section-heading rail-projects-heading">
      <strong>项目</strong>
      <el-button text circle aria-label="新建项目" @click="projectMenuOpen = !projectMenuOpen"><el-icon><Plus /></el-icon></el-button>
    </div>
    <div v-if="projectMenuOpen" class="project-menu">
      <form @submit.prevent="createEmptyProject">
        <el-input v-model="projectName" placeholder="项目名称" aria-label="新项目名称" />
        <el-button type="primary" native-type="submit">新建</el-button>
      </form>
      <el-button plain class="project-menu-link" @click="projectMenuOpen = false; $emit('useExistingProjectFolder')"><el-icon><FolderAdd /></el-icon>使用现有文件夹</el-button>
    </div>

    <label v-if="searchOpen" class="session-search">
      <el-input :model-value="searchQuery" placeholder="搜索会话…" @update:model-value="$emit('update:searchQuery', String($event))" />
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
      <section v-for="group in projectGroups" :key="group.project.projectId" class="session-group">
        <div class="session-group-heading">
          <strong :title="group.project.root"><el-icon><Folder /></el-icon>{{ group.project.name }}</strong>
          <button type="button" :aria-label="`在 ${group.project.name} 中创建会话`" @click="$emit('newProjectSession', group.project)">+</button>
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
