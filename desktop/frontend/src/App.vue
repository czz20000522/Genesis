<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { computed, onMounted, ref } from 'vue'
import { bindSessionModel, bindSessionWorkspace, checkForUpdate, closeBehavior, compactSessionContext, createProjectWorkspace, createTaskWorkspace, decideApproval, desktopRuntimeConfig, enableSessionDebug, getAgentInvocationChildConversation, getReady, getSession, getSessionAgentInvocations, getSessionDebug, getSessionTaskGraphs, getTimeline, getTimelineDetail, importProviderTemplate, installUpdate, interruptSession, kernelConfig, listSessions, loadDesktopCatalog, localModelStatus, pickMaterialDirectory as pickMaterialDirectoryFromDesktop, pickMaterialFile, pickProjectDirectory, providerProfiles, rotateProviderCredential, saveDesktopCatalog, saveKernelConfig, saveUpdateToken, searchSessions, setCloseBehavior, setupDeepSeekFlash, startLocalModel, stopLocalModel, submitTurnStream, uploadMaterial, verifyProvider, type AgentInvocationChildConversation, type AgentInvocationProjection, type ApprovalProjection, type ApprovalDecision, type CloseBehavior, type ContextCompactionResponse, type DesktopUpdate, type KernelTimeline, type KernelTimelineDetail, type LocalModelStatus, type MaterialFileSelection, type MaterialIntakeProjection, type ProviderProfile, type SessionDebugExport, type SessionListItem, type TaskGraphProjection, type TurnResponse } from './api/kernelApi'
import AgentWorkspace from './components/AgentWorkspace.vue'
import InspectorDrawer from './components/InspectorDrawer.vue'
import KernelTopBar from './components/KernelTopBar.vue'
import ProviderPanel from './components/ProviderPanel.vue'
import SessionRail from './components/SessionRail.vue'
import { compactionSummary } from './compactionView'
import { debugExportText, debugSummary } from './debugExport'
import { materialIntakeSummary } from './materialIntake'
import { isLocalProfile, profileDisplayName } from './modelSelection'
import { operationErrorLabel, turnErrorLabel } from './display'
import { isBlankSessionDraft } from './sessionDraft'
import { latestKnownSessionID, loadProjectCatalog, loadSessionCatalog, recordProjectCatalogEntry, recordSessionCatalogEntry, replaceDesktopCatalog, type DesktopProjectCatalogEntry, type DesktopSessionCatalogEntry } from './sessionCatalog'
import { timelineRows, type TimelineRow } from './timelineView'

const config = ref(kernelConfig())
const readiness = ref('unchecked')
const error = ref('')
const ownedKernelEndpointConflict = ref(false)
const sessionId = ref('')
const sessions = ref<SessionListItem[]>([])
const sessionsLoaded = ref(false)
const sessionSearchQuery = ref('')
const sessionSearchResults = ref<SessionListItem[]>([])
const sessionCatalog = ref<DesktopSessionCatalogEntry[]>([])
const projectCatalog = ref<DesktopProjectCatalogEntry[]>([])
const messageText = ref('')
const lastTurn = ref<TurnResponse | null>(null)
const pendingApprovals = ref<ApprovalProjection[]>([])
const selectedDetailRef = ref('')
const timeline = ref<KernelTimeline | null>(null)
const detail = ref<KernelTimelineDetail | null>(null)
const selectedFile = ref<MaterialFileSelection | null>(null)
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
const selectedProviderProfile = ref('')
const sessionModelProfile = ref('')
const providerCredential = ref('')
const providerTemplate = ref('deepseek')
const providerBaseURL = ref('')
const providerModelID = ref('')
const updateToken = ref('')
const update = ref<DesktopUpdate | null>(null)
const desktopCloseBehavior = ref<CloseBehavior>('exit')

const timelineProjectionRows = computed(() => timelineRows(timeline.value?.items))
const retryText = computed(() => {
  for (let index = timelineProjectionRows.value.length - 1; index >= 0; index--) {
    const row = timelineProjectionRows.value[index]
    if (row.kind !== 'processing' || row.terminalOutcome !== 'failed' || !row.turnId) continue
    for (let prior = index - 1; prior >= 0; prior--) {
      const user = timelineProjectionRows.value[prior]
      if (user.turnId === row.turnId && user.kind === 'user' && user.text) return user.text
    }
  }
  return ''
})
const displayedRows = computed(() => {
  const rows: TimelineRow[] = [...timelineProjectionRows.value]
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
const selectedFileName = computed(() => selectedFile.value?.filename ?? '')
const selectedFileIsDirectory = computed(() => selectedFile.value?.kind === 'directory')
const materialSummary = computed(() => material.value ? materialIntakeSummary(material.value) : [])
const debugSummaryRows = computed(() => debugExport.value ? debugSummary(debugExport.value) : [])
const compactionSummaryRows = computed(() => compaction.value ? compactionSummary(compaction.value) : [])
const localModelRunning = computed(() => localModel.value.ownership === 'owned' && localModel.value.readiness === 'ready')
const localModelExternallyServing = computed(() => localModel.value.reason === 'local_model_endpoint_already_serving')
const hasSelectableProviderProfile = computed(() => providerProfilesState.value.some((profile) => profile.credential_present || isLocalProfile(profile)))
const localModelLabel = computed(() => {
  if (localModelStarting.value) return '正在加载本地模型…'
  if (localModelRunning.value) return `本地模型运行中${localModel.value.pid ? ` · PID ${localModel.value.pid}` : ''}`
  if (localModelExternallyServing.value) return '已有服务正在使用此模型地址；Genesis 未接管它'
  if (localModel.value.reason === 'local_model_disabled') return '本地模型未配置'
  return '本地模型已停止'
})
const selectedProviderIsLocal = computed(() => isLocalProfile(providerProfilesState.value.find((item) => item.profile_id === selectedProviderProfile.value)))
const activeCatalogEntry = computed(() => sessionCatalog.value.find((entry) => entry.sessionId === sessionId.value))
const activeSession = computed(() => sessions.value.find((item) => item.session_id === sessionId.value))
const workspaceKindLabel = computed(() => {
  const kind = activeCatalogEntry.value?.kind
  if (kind === 'project') return '项目会话'
  if (kind === 'task') return '任务'
  return '聊天'
})
const workspaceRoot = computed(() => {
  const entry = activeCatalogEntry.value
  if (!entry) return ''
  if (entry.root) return entry.root
  if (entry.kind !== 'project') return ''
  return projectCatalog.value.find((project) => project.projectId === entry.projectId)?.root ?? ''
})
const workspaceTitle = computed(() => {
  const title = String(activeSession.value?.title || '').trim()
  if (title) return title
  const entry = activeCatalogEntry.value
  if (entry?.name) return entry.name
  if (entry?.kind === 'project') return projectCatalog.value.find((project) => project.projectId === entry.projectId)?.name ?? '项目会话'
  if (entry?.kind === 'task') return '新任务'
  return '新聊天'
})
const workspaceModelLabel = computed(() => profileDisplayName(providerProfilesState.value.find((profile) => profile.profile_id === sessionModelProfile.value)))

function currentSession() {
  const session = sessionId.value.trim()
  if (!session) {
    error.value = '请先选择或新建会话'
    return ''
  }
  return session
}

async function createProjectSession(project: DesktopProjectCatalogEntry) {
  error.value = ''
  try {
    await bindAndActivateSession({ kind: 'project', projectId: project.projectId, root: project.root })
  } catch (err) {
    error.value = operationErrorLabel(err, '创建项目会话')
  }
}

async function createEmptyProject(name: string) {
  error.value = ''
  try {
    const workspace = await createProjectWorkspace(name)
    if (workspace.existing) {
      error.value = '该项目目录已存在。请用“使用现有文件夹”添加它，或换一个名称。'
      return
    }
    const project = { projectId: newDesktopProjectId(), name: name.trim(), root: workspace.root }
    recordProjectCatalogEntry(project)
    projectCatalog.value = loadProjectCatalog()
    await persistDesktopCatalog()
    await createProjectSession(project)
  } catch (err) {
    error.value = operationErrorLabel(err, '创建项目')
  }
}

async function useExistingProjectFolder() {
  error.value = ''
  try {
    const picked = await pickProjectDirectory()
    if (!picked) return
    const existing = projectCatalog.value.find((project) => sameWorkspaceRoot(project.root, picked.root))
    if (existing) {
      await createProjectSession(existing)
      return
    }
    const project = { projectId: newDesktopProjectId(), name: picked.name, root: picked.root }
    recordProjectCatalogEntry(project)
    projectCatalog.value = loadProjectCatalog()
    await persistDesktopCatalog()
    await createProjectSession(project)
  } catch (err) {
    error.value = operationErrorLabel(err, '添加项目文件夹')
  }
}

async function createTaskSession() {
  error.value = ''
  const next = newDesktopSessionId()
  try {
    const workspace = await createTaskWorkspace(next)
    await bindAndActivateSession({ kind: 'task', root: workspace.root }, next)
  } catch (err) {
    error.value = operationErrorLabel(err, '创建任务')
  }
}

async function createChatSession() {
  error.value = ''
  try {
    await bindAndActivateSession({ kind: 'chat' })
  } catch (err) {
    error.value = operationErrorLabel(err, '创建聊天')
  }
}

async function bindAndActivateSession(entry: Omit<DesktopSessionCatalogEntry, 'sessionId'>, nextSessionId = newDesktopSessionId()) {
  await bindSessionWorkspace(config.value, nextSessionId, entry.kind === 'chat' ? 'none' : entry.kind, entry.root ?? '')
  sessionId.value = nextSessionId
	 sessionModelProfile.value = ''
  recordSessionCatalogEntry({ ...entry, sessionId: nextSessionId })
  sessionCatalog.value = loadSessionCatalog()
  await persistDesktopCatalog()
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
    timelineRowCount: timelineProjectionRows.value.length,
    selectedFileName: selectedFileName.value,
    hasMaterial: Boolean(material.value),
    hasDebugExport: Boolean(debugExport.value),
    hasCompaction: Boolean(compaction.value),
    hasLastTurn: Boolean(lastTurn.value),
  })
}

async function checkReady(quiet = false): Promise<boolean> {
	if (ownedKernelEndpointConflict.value) {
		readiness.value = 'not_ready'
		if (!quiet) error.value = 'Genesis 未启动：已有服务正在使用本地地址。关闭该服务后重新打开 Genesis。'
		return false
	}
  if (!quiet) error.value = ''
  saveKernelConfig(config.value)
  try {
    const payload = await getReady(config.value)
    const providerReadiness = String(payload.readiness ?? payload.status ?? '').trim().toLowerCase()
    readiness.value = providerReadiness === 'ready' ? 'ready' : 'connected'
  } catch (err) {
    readiness.value = 'not_ready'
    if (!quiet) error.value = operationErrorLabel(err, '连接 Genesis 本地服务')
    return false
  }
  try {
    await loadSessions()
  } catch (err) {
    if (!quiet) error.value = operationErrorLabel(err, '加载会话列表')
  }
  return true
}

async function waitForKernelReady() {
  readiness.value = 'checking'
  for (let attempt = 0; attempt < 12; attempt += 1) {
    if (await checkReady(true)) return true
    if (attempt < 11) await new Promise((resolve) => setTimeout(resolve, 1000))
  }
  readiness.value = 'not_ready'
  error.value = 'Genesis 本地服务启动超时，请检查设置后重试。'
  return false
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
      error.value = operationErrorLabel(err, '停止本地模型')
		}
		return
	}
	localModelStarting.value = true
  try {
		localModel.value = await startLocalModel()
  } catch (err) {
    error.value = operationErrorLabel(err, '启动本地模型')
	} finally {
		localModelStarting.value = false
  }
}

async function saveDesktopUpdateToken() {
  error.value = ''
  try {
    if (!await saveUpdateToken(updateToken.value)) throw new Error('update credential was not saved')
    updateToken.value = ''
    ElMessage.success('GitHub 更新令牌已保存。')
  } catch (err) {
    error.value = operationErrorLabel(err, '保存更新设置')
  }
}

async function saveCloseBehavior(value: CloseBehavior) {
  try { desktopCloseBehavior.value = await setCloseBehavior(value) } catch (err) { error.value = operationErrorLabel(err, '保存关闭行为') }
}

async function refreshDesktopUpdate() {
  error.value = ''
  try {
    update.value = await checkForUpdate()
  } catch (err) {
    error.value = operationErrorLabel(err, '检查更新')
  }
}

async function installDesktopUpdate() {
  if (!update.value?.available) return
  error.value = ''
  try {
    await installUpdate(update.value)
  } catch (err) {
    error.value = operationErrorLabel(err, '安装更新')
  }
}

async function loadProviderProfiles() {
  const payload = await providerProfiles()
  providerProfilesState.value = payload.profiles ?? []
  if (!selectedProviderProfile.value || !providerProfilesState.value.some((profile) => profile.profile_id === selectedProviderProfile.value)) {
    selectedProviderProfile.value = String(providerProfilesState.value[0]?.profile_id ?? '')
  }
}

async function ensureInitialChatAfterProviderSetup() {
  if (sessionId.value || !(await checkReady(true)) || !sessionsLoaded.value || sessions.value.length > 0) return
  await createChatSession()
}

async function toggleProviderPanel() {
  providerOpen.value = !providerOpen.value
  if (!providerOpen.value) return
  await loadProviderPanel()
}

async function openProviderPanel() {
  providerOpen.value = true
  await loadProviderPanel()
}

async function loadProviderPanel() {
  providerNotice.value = ''
  try {
    await loadProviderProfiles()
  } catch (err) {
    providerNotice.value = operationErrorLabel(err, '读取模型配置')
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
    providerNotice.value = operationErrorLabel(err, '保存 API Key')
  } finally {
    providerCredential.value = ''
    providerBusy.value = false
  }
}

async function configureDeepSeekFlash() {
	providerBusy.value = true
	providerNotice.value = ''
	try {
		const setup = await setupDeepSeekFlash(providerCredential.value)
		if (!setup.credential_present || !setup.profile_id) throw new Error('DeepSeek 凭据未保存')
		await loadProviderProfiles()
		selectedProviderProfile.value = setup.profile_id
		await ensureInitialChatAfterProviderSetup()
		const verification = await verifyProvider('', setup.profile_id)
		providerNotice.value = verification.readiness === 'ready'
			? `验证成功：${verification.model || setup.profile_id}。现在可以在任一会话输入框旁选择它。`
			: '配置已保存，但模型暂未就绪。请检查 API Key 或网络后重试验证。'
	} catch (err) {
		providerNotice.value = operationErrorLabel(err, '配置 DeepSeek Flash')
	} finally {
		providerCredential.value = ''
		providerBusy.value = false
	}
}

async function importSelectedProvider() {
	providerBusy.value = true
	providerNotice.value = ''
	try {
		const result = await importProviderTemplate(providerTemplate.value, providerCredential.value, providerBaseURL.value, providerModelID.value)
		if (result.discovery_reason) {
			providerNotice.value = `配置已保存；暂时无法获取模型列表（${result.discovery_reason}），可稍后重试导入。`
			return
		}
		await loadProviderProfiles()
		selectedProviderProfile.value = String(result.profile_ids?.[0] || selectedProviderProfile.value)
		await ensureInitialChatAfterProviderSetup()
		providerNotice.value = '模型已导入。请在当前会话输入框旁选择它。'
	} catch (err) {
		providerNotice.value = operationErrorLabel(err, '导入模型')
	} finally {
		providerCredential.value = ''
		providerBusy.value = false
	}
}

async function verifySelectedProvider() {
  providerBusy.value = true
  providerNotice.value = ''
  try {
    const result = await verifyProvider('', selectedProviderProfile.value)
    providerNotice.value = result.readiness === 'ready'
      ? `验证成功：${result.model || selectedProviderProfile.value}`
      : '模型暂未就绪。请检查 API Key、网络或本地模型状态。'
  } catch (err) {
    providerNotice.value = operationErrorLabel(err, '验证模型')
  } finally {
    providerBusy.value = false
  }
}

async function loadSessions() {
  saveKernelConfig(config.value)
  const payload = await listSessions(config.value)
  sessions.value = (payload.items ?? []).filter((item) => String(item.session_id || '').trim())
  sessionsLoaded.value = true
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
    if (sessionSearchQuery.value.trim() === normalized) error.value = operationErrorLabel(err, '搜索会话')
  }
}

async function selectSession(nextSessionId: string) {
  const next = String(nextSessionId || '').trim()
  if (!next || next === sessionId.value) return
  sessionId.value = next
  resetSessionViewState()
  await loadTimeline()
	await rememberSessionActivation(next)
}

async function rememberSessionActivation(nextSessionID: string) {
  const entry = sessionCatalog.value.find((item) => item.sessionId === nextSessionID)
  if (!entry) return
  recordSessionCatalogEntry(entry)
  sessionCatalog.value = loadSessionCatalog()
  try {
    await persistDesktopCatalog()
  } catch {
    // Session selection remains usable when the local catalogue cannot persist.
  }
}

async function refreshSessionModel(session = sessionId.value) {
	const selectedSession = String(session || '').trim()
	if (!selectedSession) {
		sessionModelProfile.value = ''
		return
	}
	const projection = await getSession(config.value, selectedSession)
	sessionModelProfile.value = String(projection.model_profile_id || '').trim()
}

async function selectSessionModel(profileID: string) {
	const session = currentSession()
	if (!session || liveStreaming.value) return
	error.value = ''
	try {
		const projection = await bindSessionModel(config.value, session, profileID)
		sessionModelProfile.value = String(projection.model_profile_id || profileID).trim()
	} catch (err) {
		error.value = operationErrorLabel(err, '切换当前会话模型')
	}
}

async function loadTimeline() {
  error.value = ''
  saveKernelConfig(config.value)
  detail.value = null
  const session = currentSession()
  if (!session) return
  try {
		await refreshSessionModel(session)
    timeline.value = await getTimeline(config.value, session)
    await loadSessionApproval(session)
    await loadWorkerInvocations(session)
    await loadTaskGraphs(session)
    lastTurn.value = null
  } catch (err) {
    error.value = operationErrorLabel(err, '加载会话')
  }
}

async function sendMessage() {
  error.value = ''
  saveKernelConfig(config.value)
  const text = messageText.value.trim()
  const session = currentSession()
  if (!session) return
	if (!sessionModelProfile.value) {
		error.value = '请先在输入框旁为当前会话选择模型。'
    return
  }
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
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    if (stopRequested.value && message.includes('turn_interrupted')) {
      timeline.value = await getTimeline(config.value, session)
      await loadSessionApproval(session)
      await loadWorkerInvocations(session)
      await loadSessions()
    } else {
      error.value = turnErrorLabel(message)
      if (message.toLowerCase().includes('llama.cpp')) await openProviderPanel()
      try {
        timeline.value = await getTimeline(config.value, session)
      } catch {
        // A disconnected kernel cannot provide durable failure evidence yet.
      }
      liveUserText.value = ''
      liveAssistantText.value = ''
    }
		return
  } finally {
    liveStreaming.value = false
    stopRequested.value = false
  }
	try {
		timeline.value = await getTimeline(config.value, session)
		liveUserText.value = ''
		liveAssistantText.value = ''
	} catch {
		error.value = '回复已完成，但暂时无法刷新会话状态。'
		return
	}
	await Promise.allSettled([
		loadSessionApproval(session),
		loadWorkerInvocations(session),
		loadTaskGraphs(session),
		loadSessions(),
	])
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
      error.value = operationErrorLabel(message, '停止生成')
      return
    }
    await loadTimeline()
  }
}

async function retryFailedTurn() {
  if (liveStreaming.value) return
  if (!retryText.value) {
    await checkReady()
    if (readiness.value !== 'not_ready') await loadTimeline()
    return
  }
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
    error.value = operationErrorLabel(err, '加载工作代理')
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
    error.value = operationErrorLabel(err, '提交确认')
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
    error.value = operationErrorLabel(err, '加载详情')
  }
}

async function pickMaterialArchive() {
  const picked = await pickMaterialFile()
  if (picked) selectedFile.value = picked
}

async function pickMaterialFolder() {
  const picked = await pickMaterialDirectoryFromDesktop()
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
    error.value = operationErrorLabel(err, '记录诊断')
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
    error.value = operationErrorLabel(err, '导出诊断')
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
    error.value = operationErrorLabel(err, '整理上下文')
  }
}

function newDesktopSessionId() {
  const randomUUID = globalThis.crypto?.randomUUID?.()
  if (randomUUID) return `desktop-${randomUUID}`
  return `desktop-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

function newDesktopProjectId() {
  const randomUUID = globalThis.crypto?.randomUUID?.()
  if (randomUUID) return `project-${randomUUID}`
  return `project-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

function newDesktopIdempotencyKey() {
  const randomUUID = globalThis.crypto?.randomUUID?.()
  if (randomUUID) return `desktop-turn-${randomUUID}`
  return `desktop-turn-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

function sameWorkspaceRoot(left: string, right: string) {
  return String(left || '').trim().replace(/[\\/]+$/, '').toLowerCase() === String(right || '').trim().replace(/[\\/]+$/, '').toLowerCase()
}

onMounted(() => {
  void initializeDesktop()
})

async function initializeDesktop() {
  await restoreDesktopCatalog()
  await refreshLocalModelStatus()
  try { desktopCloseBehavior.value = await closeBehavior() } catch {}
	try {
		const runtime = await desktopRuntimeConfig()
		ownedKernelEndpointConflict.value = runtime.sidecar?.reason === 'kernel_already_serving'
	} catch {}
	if (ownedKernelEndpointConflict.value) {
		readiness.value = 'not_ready'
		error.value = 'Genesis 未启动：已有服务正在使用本地地址。关闭该服务后重新打开 Genesis。'
		return
	}
  try {
    await loadProviderProfiles()
  } catch {
    // Provider configuration is a desktop-only local surface; readiness still loads independently.
  }
  const connected = await waitForKernelReady()
  if (!connected) return
  const restored = await restoreLatestKnownSession()
  if (!restored && !sessionId.value && !hasSelectableProviderProfile.value) providerOpen.value = true
  if (!restored && !sessionId.value && hasSelectableProviderProfile.value && sessionsLoaded.value && sessions.value.length === 0) await createChatSession()
}

async function restoreLatestKnownSession() {
  const session = latestKnownSessionID(sessionCatalog.value, sessions.value)
  if (!session) return false
  await selectSession(session)
  return true
}

async function restoreDesktopCatalog() {
  const legacyProjects = loadProjectCatalog()
  const legacySessions = loadSessionCatalog()
  try {
    const catalog = await loadDesktopCatalog()
    if (catalog && ((catalog.projects?.length ?? 0) > 0 || (catalog.sessions?.length ?? 0) > 0)) {
      replaceDesktopCatalog(catalog.projects ?? [], catalog.sessions ?? [])
    } else if (legacyProjects.length || legacySessions.length) {
      await saveDesktopCatalog({ projects: legacyProjects, sessions: legacySessions })
    }
  } catch (err) {
    error.value = '项目列表暂时无法保存到 Genesis Home。'
  }
  projectCatalog.value = loadProjectCatalog()
  sessionCatalog.value = loadSessionCatalog()
}

async function persistDesktopCatalog() {
  await saveDesktopCatalog({ projects: projectCatalog.value, sessions: sessionCatalog.value })
}
</script>

<template>
  <main :class="['app-shell', { 'app-shell--inspector-open': inspectorOpen }]">
    <SessionRail
      :session-id="sessionId"
      :sessions="sessions"
      :projects="projectCatalog"
      :catalog="sessionCatalog"
      :search-query="sessionSearchQuery"
      :search-results="sessionSearchResults"
      @create-empty-project="createEmptyProject"
      @use-existing-project-folder="useExistingProjectFolder"
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
          :inspector-open="inspectorOpen"
          @check-ready="checkReady"
          @toggle-provider="toggleProviderPanel"
          @toggle-inspector="inspectorOpen = !inspectorOpen"
        />
        <ProviderPanel
          v-if="providerOpen"
          :profiles="providerProfilesState"
          :selected-profile="selectedProviderProfile"
          :credential="providerCredential"
          :busy="providerBusy"
          :notice="providerNotice"
          :selected-profile-is-local="selectedProviderIsLocal"
          :local-model-label="localModelLabel"
          :local-model-starting="localModelStarting"
          :local-model-running="localModelRunning"
		:local-model-externally-serving="localModelExternallyServing"
		:template-id="providerTemplate"
		:base-url="providerBaseURL"
		:model-id="providerModelID"
          @close="providerOpen = false"
          @update:selected-profile="selectedProviderProfile = $event"
          @update:credential="providerCredential = $event"
		@update:template-id="providerTemplate = $event"
		@update:base-url="providerBaseURL = $event"
		@update:model-id="providerModelID = $event"
		@import-provider="importSelectedProvider"
          @setup-deep-seek-flash="configureDeepSeekFlash"
          @rotate-credential="saveProviderCredential"
          @verify="verifySelectedProvider"
          @toggle-local-model="toggleLocalModel"
        />
      </div>

      <AgentWorkspace
        :title="workspaceTitle"
        :kind-label="workspaceKindLabel"
        :workspace-root="workspaceRoot"
        :model-label="workspaceModelLabel"
        :inspector-open="inspectorOpen"
        :message-text="messageText"
        :last-turn="lastTurn"
        :rows="displayedRows"
        :selected-file-name="selectedFileName"
        :selected-file-is-directory="selectedFileIsDirectory"
        :error="error"
        :approvals="pendingApprovals"
        :retry-text="retryText"
        :interrupt-available="liveStreaming"
        :interrupting="stopRequested"
        :profiles="providerProfilesState"
        :selected-model-profile="sessionModelProfile"
        :model-selection-disabled="liveStreaming"
        @update:message-text="messageText = $event"
        @select-model="selectSessionModel"
        @send-message="sendMessage"
        @decide-approval="answerApproval"
        @pick-material-archive="pickMaterialArchive"
        @pick-material-directory="pickMaterialFolder"
        @load-detail="loadDetail"
        @retry="retryFailedTurn"
        @interrupt="interruptCurrentTurn"
        @toggle-inspector="inspectorOpen = !inspectorOpen"
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
