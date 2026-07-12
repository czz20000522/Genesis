<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { applyProviderRole, bindSessionWorkspace, checkForUpdate, closeBehavior, compactSessionContext, createTaskWorkspace, decideApproval, enableSessionDebug, getAgentInvocationChildConversation, getReady, getSession, getSessionAgentInvocations, getSessionDebug, getSessionTaskGraphs, getTimeline, getTimelineDetail, installUpdate, interruptSession, kernelConfig, listSessions, localModelStatus, pickMaterialFile, pickProjectDirectory, providerProfiles, rotateProviderCredential, saveKernelConfig, saveUpdateToken, searchSessions, setCloseBehavior, startLocalModel, stopLocalModel, submitTurnStream, uploadMaterial, verifyProvider, type AgentInvocationChildConversation, type AgentInvocationProjection, type ApprovalProjection, type ApprovalDecision, type CloseBehavior, type ContextCompactionResponse, type DesktopUpdate, type KernelTimeline, type KernelTimelineDetail, type LocalModelStatus, type MaterialFileSelection, type MaterialIntakeProjection, type ProviderProfile, type SessionDebugExport, type SessionListItem, type TaskGraphProjection, type TurnResponse } from './api/kernelApi'
import ConversationPane from './components/ConversationPane.vue'
import InspectorDrawer from './components/InspectorDrawer.vue'
import KernelTopBar from './components/KernelTopBar.vue'
import ProviderPanel from './components/ProviderPanel.vue'
import SessionRail from './components/SessionRail.vue'
import { compactionSummary } from './compactionView'
import { debugExportText, debugSummary } from './debugExport'
import { materialIntakeSummary } from './materialIntake'
import { isBlankSessionDraft } from './sessionDraft'
import { loadSessionCatalog, recordSessionCatalogEntry, type DesktopSessionCatalogEntry } from './sessionCatalog'
import { timelineDetailEntries } from './timelineDetail'
import { timelineRows, type TimelineRow } from './timelineView'

const config = ref(kernelConfig())
const readiness = ref('unchecked')
const error = ref('')
const sessionId = ref('')
const sessions = ref<SessionListItem[]>([])
const sessionSearchQuery = ref('')
const sessionSearchResults = ref<SessionListItem[]>([])
const sessionCatalog = ref<DesktopSessionCatalogEntry[]>(loadSessionCatalog())
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
const workerInvocations = ref<AgentInvocationProjection[]>([])
const workerConversation = ref<AgentInvocationChildConversation | null>(null)
const taskGraphs = ref<TaskGraphProjection[]>([])
const inspectorOpen = ref(false)
const liveUserText = ref('')
const liveAssistantText = ref('')
const liveStreaming = ref(false)
const stopRequested = ref(false)
const localModel = ref<LocalModelStatus>({})
const localModelStarting = ref(false)
const providerOpen = ref(false)
const providerBusy = ref(false)
const providerNotice = ref('')
const providerProfilesState = ref<ProviderProfile[]>([])
const providerRoleBindings = ref<Record<string, string>>({})
const selectedProviderRole = ref('coordinator')
const selectedProviderProfile = ref('')
const providerCredential = ref('')
const updateToken = ref('')
const update = ref<DesktopUpdate | null>(null)
const desktopCloseBehavior = ref<CloseBehavior>('exit')

const conversationRows = computed(() => timelineRows(timeline.value?.items))
const retryText = computed(() => {
  for (let index = conversationRows.value.length - 1; index >= 0; index--) {
    const row = conversationRows.value[index]
    if (row.kind !== 'processing' || row.terminalOutcome !== 'failed' || !row.turnId) continue
    for (let prior = index - 1; prior >= 0; prior--) {
      const user = conversationRows.value[prior]
      if (user.turnId === row.turnId && user.kind === 'user' && user.text) return user.text
    }
  }
  return ''
})
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
      turnId: '',
      terminalOutcome: '',
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
      turnId: '',
      terminalOutcome: '',
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
const localModelRunning = computed(() => localModel.value.ownership === 'owned' && localModel.value.readiness === 'ready')
const localModelLabel = computed(() => {
  if (localModelStarting.value) return '正在加载本地模型…'
  if (localModelRunning.value) return `本地模型运行中${localModel.value.pid ? ` · PID ${localModel.value.pid}` : ''}`
  if (localModel.value.reason === 'local_model_disabled') return '本地模型未配置'
  return '本地模型已停止'
})
const providerSummary = computed(() => {
  const profile = providerProfilesState.value.find((item) => item.profile_id === selectedProviderProfile.value)
  if (!profile) return '模型未配置'
  return `${profile.model_id || profile.profile_id || '模型'} · ${selectedProviderRole.value || 'coordinator'}`
})

function currentSession() {
  const session = sessionId.value.trim()
  if (!session) {
    error.value = '请先选择或新建会话'
    return ''
  }
  return session
}

async function createProjectSession(existingRoot = '') {
  error.value = ''
  try {
    const project = existingRoot
      ? { root: existingRoot, name: existingRoot.split(/[\\/]/).filter(Boolean).at(-1) || '项目' }
      : await pickProjectDirectory()
    if (!project) return
    await bindAndActivateSession('project', project.root, project.name)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function createTaskSession() {
  error.value = ''
  const next = newDesktopSessionId()
  try {
    const workspace = await createTaskWorkspace(next)
    await bindAndActivateSession('task', workspace.root, '', next)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function createChatSession() {
  error.value = ''
  try {
    await bindAndActivateSession('chat', '')
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function bindAndActivateSession(kind: DesktopSessionCatalogEntry['kind'], root: string, name = '', nextSessionId = newDesktopSessionId()) {
  await bindSessionWorkspace(config.value, nextSessionId, kind === 'chat' ? 'none' : kind, root)
  sessionId.value = nextSessionId
  recordSessionCatalogEntry({ sessionId: nextSessionId, kind, root, name })
  sessionCatalog.value = loadSessionCatalog()
  resetSessionViewState()
  await loadSessions()
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
  workerInvocations.value = []
  workerConversation.value = null
  taskGraphs.value = []
  messageText.value = ''
  liveUserText.value = ''
  liveAssistantText.value = ''
  liveStreaming.value = false
  stopRequested.value = false
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

async function refreshLocalModelStatus() {
  try {
    localModel.value = await localModelStatus()
  } catch {
    localModel.value = {}
  }
}

async function toggleLocalModel() {
	if (localModelStarting.value) return
  error.value = ''
	if (localModelRunning.value) {
		try {
			localModel.value = await stopLocalModel()
		} catch (err) {
			error.value = err instanceof Error ? err.message : String(err)
		}
		return
	}
	localModelStarting.value = true
  try {
		localModel.value = await startLocalModel()
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
	} finally {
		localModelStarting.value = false
  }
}

async function saveDesktopUpdateToken() {
  error.value = ''
  try {
    await saveUpdateToken(updateToken.value)
    updateToken.value = ''
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function saveCloseBehavior(value: CloseBehavior) {
  try { desktopCloseBehavior.value = await setCloseBehavior(value) } catch (err) { error.value = err instanceof Error ? err.message : String(err) }
}

async function refreshDesktopUpdate() {
  error.value = ''
  try {
    update.value = await checkForUpdate()
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function installDesktopUpdate() {
  if (!update.value?.available) return
  error.value = ''
  try {
    await installUpdate(update.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function loadProviderProfiles() {
  const payload = await providerProfiles()
  providerProfilesState.value = payload.profiles ?? []
  providerRoleBindings.value = payload.role_bindings ?? {}
  const bound = providerRoleBindings.value[selectedProviderRole.value]
  if (bound && providerProfilesState.value.some((profile) => profile.profile_id === bound)) {
    selectedProviderProfile.value = bound
  } else if (!selectedProviderProfile.value || !providerProfilesState.value.some((profile) => profile.profile_id === selectedProviderProfile.value)) {
    selectedProviderProfile.value = String(providerProfilesState.value[0]?.profile_id ?? '')
  }
}

async function toggleProviderPanel() {
  providerOpen.value = !providerOpen.value
  if (!providerOpen.value) return
  providerNotice.value = ''
  try {
    await loadProviderProfiles()
  } catch (err) {
    providerNotice.value = err instanceof Error ? err.message : String(err)
  }
}

async function saveProviderCredential() {
  providerBusy.value = true
  providerNotice.value = ''
  try {
    const result = await rotateProviderCredential(selectedProviderProfile.value, providerCredential.value)
    providerNotice.value = result.credential_present ? '凭据已保存到本地受保护存储。' : '凭据未保存。'
    await loadProviderProfiles()
  } catch (err) {
    providerNotice.value = err instanceof Error ? err.message : String(err)
  } finally {
    providerCredential.value = ''
    providerBusy.value = false
  }
}

async function verifySelectedProvider() {
  providerBusy.value = true
  providerNotice.value = ''
  try {
    const result = await verifyProvider(selectedProviderRole.value, selectedProviderProfile.value)
    providerNotice.value = result.readiness === 'ready'
      ? `验证成功：${result.model || selectedProviderProfile.value}`
      : `验证未就绪：${result.readiness_reason || 'provider_not_ready'}`
  } catch (err) {
    providerNotice.value = err instanceof Error ? err.message : String(err)
  } finally {
    providerBusy.value = false
  }
}

async function applySelectedProvider() {
  providerBusy.value = true
  providerNotice.value = ''
  try {
    const result = await applyProviderRole(selectedProviderRole.value, selectedProviderProfile.value)
    if (result.status === 'owned_kernel_restarted') {
      providerNotice.value = '已应用并重启本地 Genesis 服务。'
      await checkReady()
    } else if (result.status === 'external_kernel_restart_required') {
      providerNotice.value = '配置已保存；外部 Genesis 服务需要由其所有者重启。'
    } else {
      providerNotice.value = `配置已保存，但重启未就绪：${result.sidecar?.reason ?? result.status ?? 'unknown'}`
    }
    await loadProviderProfiles()
  } catch (err) {
    providerNotice.value = err instanceof Error ? err.message : String(err)
  } finally {
    providerBusy.value = false
  }
}

async function loadSessions() {
  saveKernelConfig(config.value)
  const payload = await listSessions(config.value)
  sessions.value = (payload.items ?? []).filter((item) => String(item.session_id || '').trim())
}

async function updateSessionSearch(query: string) {
  sessionSearchQuery.value = query
  const normalized = query.trim()
  if (!normalized) {
    sessionSearchResults.value = []
    return
  }
  try {
    const payload = await searchSessions(config.value, normalized, 30)
    if (sessionSearchQuery.value.trim() === normalized) sessionSearchResults.value = payload.items ?? []
  } catch (err) {
    if (sessionSearchQuery.value.trim() === normalized) error.value = err instanceof Error ? err.message : String(err)
  }
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
    await loadWorkerInvocations(session)
    await loadTaskGraphs(session)
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
    await loadWorkerInvocations(session)
    await loadTaskGraphs(session)
    await loadSessions()
    liveUserText.value = ''
    liveAssistantText.value = ''
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    if (stopRequested.value && message.includes('turn_interrupted')) {
      timeline.value = await getTimeline(config.value, session)
      await loadSessionApproval(session)
      await loadWorkerInvocations(session)
      await loadSessions()
    } else {
      error.value = message
    }
  } finally {
    liveStreaming.value = false
    stopRequested.value = false
  }
}

async function interruptCurrentTurn() {
  if (!liveStreaming.value || stopRequested.value) return
  const session = currentSession()
  if (!session) return
  error.value = ''
  stopRequested.value = true
  try {
    await interruptSession(config.value, session)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    if (!message.includes('no active turn')) {
      error.value = message
      return
    }
    await loadTimeline()
  }
}

async function retryFailedTurn() {
  if (!retryText.value || liveStreaming.value) return
  messageText.value = retryText.value
  await sendMessage()
}

async function loadSessionApproval(session = currentSession()) {
  if (!session) return
  const projection = await getSession(config.value, session)
  pendingApprovals.value = (projection.approvals ?? []).filter((approval) => {
    const approvalSession = String(approval.session_id ?? session).trim()
    return approval.status === 'pending' && approvalSession === session
  })
}

async function loadWorkerInvocations(session = currentSession()) {
  if (!session) return
  workerInvocations.value = await getSessionAgentInvocations(config.value, session)
}

async function loadTaskGraphs(session = currentSession()) {
  if (!session) return
  taskGraphs.value = await getSessionTaskGraphs(config.value, session)
}

async function loadWorkerConversation(invocationID: string) {
  const normalized = String(invocationID || '').trim()
  if (!normalized) return
  error.value = ''
  saveKernelConfig(config.value)
  try {
    workerConversation.value = await getAgentInvocationChildConversation(config.value, normalized)
    inspectorOpen.value = true
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
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
  void initializeDesktop()
})

async function initializeDesktop() {
  await refreshLocalModelStatus()
  try { desktopCloseBehavior.value = await closeBehavior() } catch {}
  try {
    await loadProviderProfiles()
  } catch {
    // Provider configuration is a desktop-only local surface; readiness still loads independently.
  }
  await checkReady()
  if (!sessionId.value) await createChatSession()
}
</script>

<template>
  <main :class="['chat-shell', { 'chat-shell--inspector-open': inspectorOpen }]">
    <SessionRail
      :session-id="sessionId"
      :sessions="sessions"
      :catalog="sessionCatalog"
      :search-query="sessionSearchQuery"
      :search-results="sessionSearchResults"
      @new-project="createProjectSession"
      @new-project-session="createProjectSession"
      @new-task="createTaskSession"
      @new-chat="createChatSession"
      @select-session="selectSession"
      @update:search-query="updateSessionSearch"
    />

    <section class="session-workspace">
      <div class="topbar-stack">
        <KernelTopBar
          :session-id="sessionId"
          :readiness="readiness"
          :error="error"
          :inspector-open="inspectorOpen"
          :local-model="localModelLabel"
          :local-model-running="localModelRunning"
		  :local-model-starting="localModelStarting"
          :provider-summary="providerSummary"
          @check-ready="checkReady"
          @toggle-local-model="toggleLocalModel"
          @toggle-provider="toggleProviderPanel"
          @toggle-inspector="inspectorOpen = !inspectorOpen"
        />
        <ProviderPanel
          v-if="providerOpen"
          :profiles="providerProfilesState"
          :role-bindings="providerRoleBindings"
          :selected-role="selectedProviderRole"
          :selected-profile="selectedProviderProfile"
          :credential="providerCredential"
          :busy="providerBusy"
          :notice="providerNotice"
          @close="providerOpen = false"
          @update:selected-role="selectedProviderRole = $event"
          @update:selected-profile="selectedProviderProfile = $event"
          @update:credential="providerCredential = $event"
          @rotate-credential="saveProviderCredential"
          @verify="verifySelectedProvider"
          @apply="applySelectedProvider"
        />
      </div>

      <ConversationPane
        :session-id="sessionId"
        :message-text="messageText"
        :last-turn="lastTurn"
        :rows="displayedRows"
        :detail-entries="detailEntries"
        :selected-file-name="selectedFileName"
        :readiness="readiness"
        :approvals="pendingApprovals"
        :retry-text="retryText"
        :interrupt-available="liveStreaming"
        :interrupting="stopRequested"
        @update:message-text="messageText = $event"
        @send-message="sendMessage"
        @decide-approval="answerApproval"
        @pick-material="pickMaterial"
        @load-detail="loadDetail"
        @select-material="selectMaterial"
        @retry="retryFailedTurn"
        @interrupt="interruptCurrentTurn"
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
	  :worker-invocations="workerInvocations"
	  :worker-conversation="workerConversation"
	  :task-graphs="taskGraphs"
      :debug-export-ready="Boolean(debugExport)"
	  :update-token="updateToken"
	  :update="update"
	  :close-behavior="desktopCloseBehavior"
      @update:base-url="config.baseUrl = $event"
      @update:runtime-token="config.runtimeToken = $event"
      @update:selected-detail-ref="selectedDetailRef = $event"
      @check-ready="checkReady"
      @load-detail="loadDetail"
      @enable-debug="enableDebug"
      @export-debug="exportDebug"
      @download-debug="downloadDebugExport"
      @compact-context="compactContext"
	  @refresh-workers="loadWorkerInvocations()"
	  @refresh-task-graphs="loadTaskGraphs()"
	  @select-worker="loadWorkerConversation"
	  @update:update-token="updateToken = $event"
	  @save-update-token="saveDesktopUpdateToken"
	  @check-update="refreshDesktopUpdate"
	  @install-update="installDesktopUpdate"
	  @update:close-behavior="saveCloseBehavior"
      @close="inspectorOpen = false"
    />
  </main>
</template>
