<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { compactSessionContext, decideApproval, enableSessionDebug, getReady, getSession, getSessionDebug, getTimeline, getTimelineDetail, kernelConfig, listSessions, pickMaterialFile, saveKernelConfig, submitTurnStream, uploadMaterial, type ApprovalProjection, type ApprovalDecision, type ContextCompactionResponse, type KernelTimeline, type KernelTimelineDetail, type MaterialFileSelection, type MaterialIntakeProjection, type SessionDebugExport, type SessionListItem, type TurnResponse } from './api/kernelApi'
import ConversationPane from './components/ConversationPane.vue'
import InspectorDrawer from './components/InspectorDrawer.vue'
import KernelTopBar from './components/KernelTopBar.vue'
import SessionRail from './components/SessionRail.vue'
import { compactionSummary } from './compactionView'
import { debugExportText, debugSummary } from './debugExport'
import { materialIntakeSummary } from './materialIntake'
import { isBlankSessionDraft } from './sessionDraft'
import { timelineDetailEntries } from './timelineDetail'
import { timelineRows, type TimelineRow } from './timelineView'

const config = ref(kernelConfig())
const readiness = ref('unchecked')
const error = ref('')
const sessionId = ref(newDesktopSessionId())
const sessions = ref<SessionListItem[]>([])
const messageText = ref('')
const lastTurn = ref<TurnResponse | null>(null)
const pendingApprovals = ref<ApprovalProjection[]>([])
const selectedDetailRef = ref('')
const timeline = ref<KernelTimeline | null>(null)
const detail = ref<KernelTimelineDetail | null>(null)
const selectedFile = ref<File | MaterialFileSelection | null>(null)
const material = ref<MaterialIntakeProjection | null>(null)
const debugExport = ref<SessionDebugExport | null>(null)
const compaction = ref<ContextCompactionResponse | null>(null)
const inspectorOpen = ref(false)
const liveUserText = ref('')
const liveAssistantText = ref('')
const liveStreaming = ref(false)

const conversationRows = computed(() => timelineRows(timeline.value?.items))
const displayedRows = computed(() => {
  const rows: TimelineRow[] = [...conversationRows.value]
  if (liveUserText.value) {
    rows.push({
      id: 'live-user',
      kind: 'user',
      text: liveUserText.value,
      meta: '',
      detailRef: '',
      detailAvailable: false,
    })
  }
  if (liveStreaming.value || liveAssistantText.value) {
    rows.push({
      id: 'live-assistant',
      kind: 'assistant',
      text: liveAssistantText.value || '正在生成...',
      meta: liveStreaming.value ? '正在生成' : '',
      detailRef: '',
      detailAvailable: false,
      streaming: liveStreaming.value,
    })
  }
  return rows
})
const detailEntries = computed(() => timelineDetailEntries(timeline.value?.items))
const selectedFileName = computed(() => selectedFile.value ? ('name' in selectedFile.value ? selectedFile.value.name : selectedFile.value.filename) : '')
const materialSummary = computed(() => material.value ? materialIntakeSummary(material.value) : [])
const debugSummaryRows = computed(() => debugExport.value ? debugSummary(debugExport.value) : [])
const compactionSummaryRows = computed(() => compaction.value ? compactionSummary(compaction.value) : [])

function currentSession() {
  const session = sessionId.value.trim()
  if (!session) {
    error.value = '请先选择或新建会话'
    return ''
  }
  return session
}

function newSession() {
  if (isCurrentSessionBlank()) {
    resetSessionViewState()
    return
  }
  const next = newDesktopSessionId()
  sessionId.value = next
  resetSessionViewState()
}

function resetSessionViewState() {
  timeline.value = null
  detail.value = null
  lastTurn.value = null
  pendingApprovals.value = []
  selectedDetailRef.value = ''
  selectedFile.value = null
  material.value = null
  debugExport.value = null
  compaction.value = null
  messageText.value = ''
  liveUserText.value = ''
  liveAssistantText.value = ''
  liveStreaming.value = false
  error.value = ''
  inspectorOpen.value = false
}

function isCurrentSessionBlank() {
  return isBlankSessionDraft({
    messageText: messageText.value,
    timelineRowCount: conversationRows.value.length,
    selectedFileName: selectedFileName.value,
    hasMaterial: Boolean(material.value),
    hasDebugExport: Boolean(debugExport.value),
    hasCompaction: Boolean(compaction.value),
    hasLastTurn: Boolean(lastTurn.value),
  })
}

async function checkReady() {
  error.value = ''
  saveKernelConfig(config.value)
  try {
    const payload = await getReady(config.value)
    readiness.value = String(payload.readiness ?? payload.status ?? 'unknown')
    await loadSessions()
  } catch (err) {
    readiness.value = 'not_ready'
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function loadSessions() {
  saveKernelConfig(config.value)
  const payload = await listSessions(config.value)
  sessions.value = (payload.items ?? []).filter((item) => String(item.session_id || '').trim())
}

async function selectSession(nextSessionId: string) {
  const next = String(nextSessionId || '').trim()
  if (!next || next === sessionId.value) return
  sessionId.value = next
  resetSessionViewState()
  await loadTimeline()
}

async function loadTimeline() {
  error.value = ''
  saveKernelConfig(config.value)
  detail.value = null
  const session = currentSession()
  if (!session) return
  try {
    timeline.value = await getTimeline(config.value, session)
    await loadSessionApproval(session)
    lastTurn.value = null
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function sendMessage() {
  error.value = ''
  saveKernelConfig(config.value)
  const text = messageText.value.trim()
  const session = currentSession()
  if (!session) return
  if (!text && !selectedFile.value) {
    error.value = '请输入消息或选择附件'
    return
  }
  try {
    const fileWasSelected = selectedFile.value
    if (fileWasSelected) material.value = await uploadMaterial(config.value, session, fileWasSelected)
    const submittedText = text || '请查看我上传的资料。'
    liveUserText.value = submittedText
    liveAssistantText.value = ''
    liveStreaming.value = true
    lastTurn.value = await submitTurnStream(config.value, session, submittedText, newDesktopIdempotencyKey(), (event) => {
      if (event.type === 'assistant_delta') liveAssistantText.value += event.delta ?? ''
      if ((event.type === 'turn_completed' || event.type === 'turn_paused') && event.response) lastTurn.value = event.response
    })
    messageText.value = ''
    if (fileWasSelected) selectedFile.value = null
    timeline.value = await getTimeline(config.value, session)
    await loadSessionApproval(session)
    await loadSessions()
    liveUserText.value = ''
    liveAssistantText.value = ''
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    liveStreaming.value = false
  }
}

async function loadSessionApproval(session = currentSession()) {
  if (!session) return
  const projection = await getSession(config.value, session)
  pendingApprovals.value = (projection.approvals ?? []).filter((approval) => {
    const approvalSession = String(approval.session_id ?? session).trim()
    return approval.status === 'pending' && approvalSession === session
  })
}

async function answerApproval(approvalID: string, decision: ApprovalDecision) {
  approvalID = String(approvalID || '').trim()
  if (!approvalID) return
  error.value = ''
  saveKernelConfig(config.value)
  const session = currentSession()
  if (!session) return
  try {
    await decideApproval(config.value, approvalID, decision, decision === 'approved' ? 'desktop approved once' : 'desktop denied')
    pendingApprovals.value = pendingApprovals.value.filter((approval) => approval.approval_id !== approvalID)
    timeline.value = await getTimeline(config.value, session)
    await loadSessionApproval(session)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function loadDetail(detailRef = selectedDetailRef.value) {
  error.value = ''
  saveKernelConfig(config.value)
  selectedDetailRef.value = detailRef
  const session = currentSession()
  if (!session) return
  try {
    detail.value = await getTimelineDetail(config.value, session, detailRef)
    inspectorOpen.value = true
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

function selectMaterial(event: Event) {
  selectedFile.value = (event.target as HTMLInputElement).files?.[0] ?? null
}

async function pickMaterial() {
  const picked = await pickMaterialFile()
  if (picked) selectedFile.value = picked
}

async function enableDebug() {
  error.value = ''
  saveKernelConfig(config.value)
  const session = currentSession()
  if (!session) return
  try {
    debugExport.value = await enableSessionDebug(config.value, session)
    inspectorOpen.value = true
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function exportDebug() {
  error.value = ''
  saveKernelConfig(config.value)
  const session = currentSession()
  if (!session) return
  try {
    debugExport.value = await getSessionDebug(config.value, session)
    inspectorOpen.value = true
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

function downloadDebugExport() {
  if (!debugExport.value) return
  const url = URL.createObjectURL(new Blob([debugExportText(debugExport.value)], { type: 'application/json' }))
  const link = document.createElement('a')
  link.href = url
  link.download = `${sessionId.value.trim() || 'session'}-debug.json`
  link.click()
  URL.revokeObjectURL(url)
}

async function compactContext() {
  error.value = ''
  saveKernelConfig(config.value)
  const session = currentSession()
  if (!session) return
  try {
    compaction.value = await compactSessionContext(config.value, session)
    inspectorOpen.value = true
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

function newDesktopSessionId() {
  const randomUUID = globalThis.crypto?.randomUUID?.()
  if (randomUUID) return `desktop-${randomUUID}`
  return `desktop-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

function newDesktopIdempotencyKey() {
  const randomUUID = globalThis.crypto?.randomUUID?.()
  if (randomUUID) return `desktop-turn-${randomUUID}`
  return `desktop-turn-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

onMounted(() => {
  void checkReady()
})
</script>

<template>
  <main :class="['chat-shell', { 'chat-shell--inspector-open': inspectorOpen }]">
    <SessionRail
      :session-id="sessionId"
      :sessions="sessions"
      @new-session="newSession"
      @select-session="selectSession"
    />

    <section class="session-workspace">
      <KernelTopBar
        :session-id="sessionId"
        :readiness="readiness"
        :error="error"
        :inspector-open="inspectorOpen"
        @check-ready="checkReady"
        @toggle-inspector="inspectorOpen = !inspectorOpen"
      />

      <ConversationPane
        :session-id="sessionId"
        :message-text="messageText"
        :last-turn="lastTurn"
        :rows="displayedRows"
        :detail-entries="detailEntries"
        :selected-file-name="selectedFileName"
        :readiness="readiness"
        :approvals="pendingApprovals"
        @update:message-text="messageText = $event"
        @send-message="sendMessage"
        @decide-approval="answerApproval"
        @pick-material="pickMaterial"
        @load-detail="loadDetail"
        @select-material="selectMaterial"
      />
    </section>

    <InspectorDrawer
      v-if="inspectorOpen"
      :base-url="config.baseUrl"
      :runtime-token="config.runtimeToken"
      :readiness="readiness"
      :detail="detail"
      :selected-detail-ref="selectedDetailRef"
      :material-summary="materialSummary"
      :debug-summary-rows="debugSummaryRows"
      :compaction-summary-rows="compactionSummaryRows"
      :debug-export-ready="Boolean(debugExport)"
      @update:base-url="config.baseUrl = $event"
      @update:runtime-token="config.runtimeToken = $event"
      @update:selected-detail-ref="selectedDetailRef = $event"
      @check-ready="checkReady"
      @load-detail="loadDetail"
      @enable-debug="enableDebug"
      @export-debug="exportDebug"
      @download-debug="downloadDebugExport"
      @compact-context="compactContext"
      @close="inspectorOpen = false"
    />
  </main>
</template>
