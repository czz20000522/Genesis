<script setup lang="ts">
import { computed, ref } from 'vue'
import { compactSessionContext, decideApproval, enableSessionDebug, getReady, getSessionDebug, getTimeline, getTimelineDetail, kernelConfig, listApprovals, saveKernelConfig, submitTurn, uploadMaterial, type ApprovalProjection, type ContextCompactionResponse, type KernelTimeline, type KernelTimelineDetail, type MaterialIntakeProjection, type SessionDebugExport, type TurnResponse } from './api/kernelApi'
import ActionDock from './components/ActionDock.vue'
import ConversationPane from './components/ConversationPane.vue'
import InspectorDrawer from './components/InspectorDrawer.vue'
import KernelTopBar from './components/KernelTopBar.vue'
import SessionRail from './components/SessionRail.vue'
import { compactionSummary } from './compactionView'
import { debugExportText, debugSummary } from './debugExport'
import { materialIntakeSummary } from './materialIntake'
import { timelineDetailEntries } from './timelineDetail'

const config = ref(kernelConfig())
const readiness = ref('unchecked')
const error = ref('')
const sessionId = ref('')
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
const detailEntries = computed(() => timelineDetailEntries(timeline.value?.items))
const materialSummary = computed(() => material.value ? materialIntakeSummary(material.value) : [])
const debugSummaryRows = computed(() => debugExport.value ? debugSummary(debugExport.value) : [])
const compactionSummaryRows = computed(() => compaction.value ? compactionSummary(compaction.value) : [])

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
  try {
    timeline.value = await getTimeline(config.value, sessionId.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function sendMessage() {
  error.value = ''
  saveKernelConfig(config.value)
  const text = messageText.value.trim()
  const session = sessionId.value.trim()
  if (!session) {
    error.value = 'session id is required'
    return
  }
  if (!text) {
    error.value = 'message is required'
    return
  }
  try {
    lastTurn.value = await submitTurn(config.value, session, text, newDesktopIdempotencyKey())
    messageText.value = ''
    timeline.value = await getTimeline(config.value, session)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function loadDetail(detailRef = selectedDetailRef.value) {
  error.value = ''
  saveKernelConfig(config.value)
  selectedDetailRef.value = detailRef
  try {
    detail.value = await getTimelineDetail(config.value, sessionId.value, detailRef)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

function selectMaterial(event: Event) {
  selectedFile.value = (event.target as HTMLInputElement).files?.[0] ?? null
}

async function uploadSelectedMaterial() {
  error.value = ''
  saveKernelConfig(config.value)
  if (!selectedFile.value) {
    error.value = 'select a material file first'
    return
  }
  try {
    material.value = await uploadMaterial(config.value, sessionId.value, selectedFile.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
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
    if (sessionId.value.trim()) {
      timeline.value = await getTimeline(config.value, sessionId.value)
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function enableDebug() {
  error.value = ''
  saveKernelConfig(config.value)
  if (!sessionId.value.trim()) {
    error.value = 'session id is required'
    return
  }
  try {
    debugExport.value = await enableSessionDebug(config.value, sessionId.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function exportDebug() {
  error.value = ''
  saveKernelConfig(config.value)
  if (!sessionId.value.trim()) {
    error.value = 'session id is required'
    return
  }
  try {
    debugExport.value = await getSessionDebug(config.value, sessionId.value)
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
  if (!sessionId.value.trim()) {
    error.value = 'session id is required'
    return
  }
  try {
    compaction.value = await compactSessionContext(config.value, sessionId.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

function newDesktopIdempotencyKey() {
  const randomUUID = globalThis.crypto?.randomUUID?.()
  if (randomUUID) return `desktop-turn-${randomUUID}`
  return `desktop-turn-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}
</script>

<template>
  <main class="workbench">
    <KernelTopBar
      :base-url="config.baseUrl"
      :runtime-token="config.runtimeToken"
      :readiness="readiness"
      :error="error"
      @update:base-url="config.baseUrl = $event"
      @update:runtime-token="config.runtimeToken = $event"
      @check-ready="checkReady"
    />

    <section class="workbench-grid">
      <SessionRail
        :session-id="sessionId"
        :debug-export-ready="Boolean(debugExport)"
        @update:session-id="sessionId = $event"
        @load-timeline="loadTimeline"
        @select-material="selectMaterial"
        @upload-material="uploadSelectedMaterial"
        @enable-debug="enableDebug"
        @export-debug="exportDebug"
        @download-debug="downloadDebugExport"
        @compact-context="compactContext"
      />

      <ConversationPane
        :message-text="messageText"
        :last-turn="lastTurn"
        :timeline="timeline"
        :detail-entries="detailEntries"
        @update:message-text="messageText = $event"
        @send-message="sendMessage"
        @load-detail="loadDetail"
      />

      <ActionDock
        :approvals="approvals"
        :approval-reason="approvalReason"
        @update:approval-reason="approvalReason = $event"
        @load-approvals="loadApprovals"
        @decide-approval="submitApprovalDecision"
      />

      <InspectorDrawer
        :detail="detail"
        :selected-detail-ref="selectedDetailRef"
        :material-summary="materialSummary"
        :debug-summary-rows="debugSummaryRows"
        :compaction-summary-rows="compactionSummaryRows"
        @update:selected-detail-ref="selectedDetailRef = $event"
        @load-detail="loadDetail"
      />
    </section>
  </main>
</template>
