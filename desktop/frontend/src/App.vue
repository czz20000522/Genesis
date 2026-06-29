<script setup lang="ts">
import { computed, ref } from 'vue'
import { compactSessionContext, decideApproval, enableSessionDebug, getReady, getSessionDebug, getTimeline, getTimelineDetail, kernelConfig, listApprovals, saveKernelConfig, submitTurn, uploadMaterial, type ApprovalProjection, type ContextCompactionResponse, type KernelTimeline, type KernelTimelineDetail, type MaterialIntakeProjection, type SessionDebugExport, type TurnResponse } from './api/kernelApi'
import ConversationPane from './components/ConversationPane.vue'
import InspectorDrawer from './components/InspectorDrawer.vue'
import KernelTopBar from './components/KernelTopBar.vue'
import SessionRail from './components/SessionRail.vue'
import { compactionSummary } from './compactionView'
import { debugExportText, debugSummary } from './debugExport'
import { materialIntakeSummary } from './materialIntake'
import { timelineDetailEntries } from './timelineDetail'
import { timelineRows } from './timelineView'

const config = ref(kernelConfig())
const readiness = ref('unchecked')
const error = ref('')
const sessionId = ref(newDesktopSessionId())
const localSessions = ref([sessionId.value])
const messageText = ref('')
const lastTurn = ref<TurnResponse | null>(null)
const selectedDetailRef = ref('')
const timeline = ref<KernelTimeline | null>(null)
const detail = ref<KernelTimelineDetail | null>(null)
const selectedFile = ref<File | null>(null)
const material = ref<MaterialIntakeProjection | null>(null)
const approvals = ref<ApprovalProjection[]>([])
const approvalReason = ref('')
const debugExport = ref<SessionDebugExport | null>(null)
const compaction = ref<ContextCompactionResponse | null>(null)
const inspectorOpen = ref(false)

const conversationRows = computed(() => timelineRows(timeline.value?.items))
const detailEntries = computed(() => timelineDetailEntries(timeline.value?.items))
const selectedFileName = computed(() => selectedFile.value?.name ?? '')
const materialSummary = computed(() => material.value ? materialIntakeSummary(material.value) : [])
const debugSummaryRows = computed(() => debugExport.value ? debugSummary(debugExport.value) : [])
const compactionSummaryRows = computed(() => compaction.value ? compactionSummary(compaction.value) : [])

function currentSession() {
  const session = sessionId.value.trim()
  if (!session) {
    error.value = '请先选择或新建会话'
    return ''
  }
  if (!localSessions.value.includes(session)) localSessions.value = [session, ...localSessions.value]
  return session
}

function newSession() {
  const next = newDesktopSessionId()
  sessionId.value = next
  localSessions.value = [next, ...localSessions.value]
  timeline.value = null
  detail.value = null
  lastTurn.value = null
  selectedDetailRef.value = ''
  approvals.value = []
  messageText.value = ''
  error.value = ''
}

function selectSession(value: string) {
  const session = value.trim()
  if (!session) return
  sessionId.value = session
  if (!localSessions.value.includes(session)) localSessions.value = [session, ...localSessions.value]
  timeline.value = null
  detail.value = null
  lastTurn.value = null
  selectedDetailRef.value = ''
  error.value = ''
}

async function checkReady() {
  error.value = ''
  saveKernelConfig(config.value)
  try {
    const payload = await getReady(config.value)
    readiness.value = String(payload.readiness ?? payload.status ?? 'unknown')
  } catch (err) {
    readiness.value = 'not_ready'
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function loadTimeline() {
  error.value = ''
  saveKernelConfig(config.value)
  detail.value = null
  const session = currentSession()
  if (!session) return
  try {
    timeline.value = await getTimeline(config.value, session)
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
    lastTurn.value = await submitTurn(config.value, session, text || '请查看我上传的资料。', newDesktopIdempotencyKey())
    messageText.value = ''
    if (fileWasSelected) selectedFile.value = null
    timeline.value = await getTimeline(config.value, session)
    await loadApprovals()
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

async function loadApprovals() {
  error.value = ''
  saveKernelConfig(config.value)
  try {
    approvals.value = (await listApprovals(config.value)).items ?? []
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function submitApprovalDecision(approvalId: string, decision: 'approved' | 'denied') {
  error.value = ''
  saveKernelConfig(config.value)
  try {
    await decideApproval(config.value, approvalId, decision, approvalReason.value || decision)
    approvalReason.value = ''
    await loadApprovals()
    const session = sessionId.value.trim()
    if (session) timeline.value = await getTimeline(config.value, session)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
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
</script>

<template>
  <main :class="['chat-shell', { 'chat-shell--inspector-open': inspectorOpen }]">
    <SessionRail
      :session-id="sessionId"
      :sessions="localSessions"
      @new-session="newSession"
      @select-session="selectSession"
      @load-timeline="loadTimeline"
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
        :rows="conversationRows"
        :detail-entries="detailEntries"
        :approvals="approvals"
        :approval-reason="approvalReason"
        :selected-file-name="selectedFileName"
        :readiness="readiness"
        @update:message-text="messageText = $event"
        @update:approval-reason="approvalReason = $event"
        @send-message="sendMessage"
        @load-detail="loadDetail"
        @select-material="selectMaterial"
        @decide-approval="submitApprovalDecision"
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
