const baseUrlKey = 'genesis.desktop.kernel_base_url'
const runtimeTokenKey = 'genesis.desktop.runtime_token'

export type KernelConfig = {
  baseUrl: string
  runtimeToken: string
}

export type KernelTimeline = {
  session_id?: string
  readiness?: string
  items?: UITimelineItem[]
}

export type KernelTimelineDetail = {
  detail_ref?: string
  readiness?: string
  item?: UITimelineItem
}

export type UITimelineItem = {
  item_id?: string
  turn_id?: string
  kind?: string
  phase?: string
  wait_reason?: string
  terminal_outcome?: string
  terminal_cause?: string
  text?: string
  tool?: string
  approval_id?: string
  job_id?: string
  command_preview?: string
  output_preview?: string
  visible_output?: string
  output_source?: string
  output_truncated?: boolean
  output_truncation?: string
  default_open?: boolean
  detail_ref?: string
  detail_available?: boolean
  duration_ms?: number
  tool_count?: number
  job_count?: number
  compaction_count?: number
  children?: UITimelineItem[]
  [key: string]: unknown
}

export type MaterialIntakeProjection = {
  admission_result?: string
  refusal_reason_class?: string
  source_snapshot_ref?: string
  available_operations?: string[]
  diagnostics?: Array<Record<string, unknown>>
}

export type TurnResponse = {
  session_id?: string
  turn_id?: string
  final?: {
    text?: string
    model?: string
  }
  pause?: Record<string, unknown>
  error?: {
    code?: string
    message?: string
  }
}

export type TurnStreamEvent = {
  type?: string
  delta?: string
  response?: TurnResponse
  error?: {
    code?: string
    message?: string
  }
}

export type ApprovalProjection = {
  approval_id?: string
  session_id?: string
  turn_id?: string
  status?: string
  effect?: {
    tool?: string
    command_preview?: string
    cwd?: string
  }
  blocked_reason?: string
}

export type ApprovalListResponse = {
  items?: ApprovalProjection[]
}

export type SessionListItem = {
  session_id?: string
  title?: string
  updated_at?: string
}

export type SessionListResponse = {
  items?: SessionListItem[]
}

export type SessionSearchResult = SessionListItem & {
  match_fields?: string[]
  snippet?: string
}

export type SessionSearchResponse = {
  query?: string
  items?: SessionSearchResult[]
}

export type SessionProjection = {
  session_id?: string
  workspace_mode?: SessionWorkspaceKind
	model_profile_id?: string
  approvals?: ApprovalProjection[]
}

export type AgentInvocationProjection = {
  invocation_id?: string
  session_id?: string
  parent_turn_id?: string
  parent_invocation_id?: string
  parent_role_id?: string
  agent_profile_ref?: string
  model_profile_id?: string
  context_scope?: string
  status?: string
  admitted_at?: string
}

export type AgentInvocationChildConversation = {
  invocation_id?: string
  run_id?: string
  session_id?: string
  role_id?: string
  status?: string
  model?: string
  final?: {
    text?: string
  }
  error?: {
    code?: string
    message?: string
  }
  usage?: Record<string, number>
  evidence_refs?: string[]
}

export type TaskGraphNodeProjection = {
  node_id?: string
  title?: string
  status?: string
  reason?: string
  evidence_refs?: string[]
}

export type TaskGraphProjection = {
  graph_id?: string
  nodes?: TaskGraphNodeProjection[]
  edges?: Array<{ from_node_id?: string, to_node_id?: string }>
}

export type SessionWorkspaceKind = 'project' | 'task' | 'none'

export type ProjectDirectorySelection = {
  root: string
  name: string
}

export type TaskWorkspaceSelection = {
  root: string
}

export type ProjectWorkspaceSelection = {
  root: string
  existing?: boolean
}

export type DesktopCatalogProjection = {
  projects?: Array<{ projectId: string, name: string, root: string }>
  sessions?: Array<{ sessionId: string, kind: 'project' | 'task' | 'chat', projectId?: string, root?: string, name?: string }>
}

export type ApprovalDecision = 'approved' | 'denied'

export type SessionDebugExport = Record<string, unknown>

export type ContextCompactionResponse = {
  admission_result?: string
  reason_class?: string
  refusal_reason_class?: string
}

export type MaterialFileSelection = {
  file_path: string
  filename: string
  kind?: 'archive' | 'directory'
}

export type LocalModelStatus = {
  ownership?: string
  readiness?: string
  reason?: string
  pid?: number
}

export type DesktopRuntimeConfig = {
  sidecar?: LocalModelStatus
}

export type ProviderProfile = {
  profile_id?: string
  model_id?: string
  gateway_route?: string
  protocol?: string
  provider_adapter_id?: string
  roles?: string[]
  credential_present?: boolean
}

export type ProviderProfiles = {
  profiles?: ProviderProfile[]
  role_bindings?: Record<string, string>
}

export type ProviderImport = {
	route_id?: string
	profile_ids?: string[]
	discovery_reason?: string
}

export type ProviderCredentialRotation = {
	profile_id?: string
	credential_present?: boolean
}

export type FirstRunDeepSeek = {
	profile_id?: string
	credential_present?: boolean
}

export type ProviderVerification = {
  readiness?: string
  readiness_reason?: string
  model_role?: string
  profile_id?: string
  model?: string
}

export type DesktopUpdate = {
  current_version?: string
  latest_version?: string
  release_url?: string
  release_notes?: string
  installer_url?: string
  checksum_url?: string
  available?: boolean
  reason?: string
}

export type CloseBehavior = 'exit' | 'minimize_to_tray'

export async function closeBehavior(): Promise<CloseBehavior> {
  const bridge = wailsAppBridge()
  if (!bridge?.CloseBehavior) throw new Error('关闭行为仅在 Genesis 桌面客户端中可用')
  return bridge.CloseBehavior() as Promise<CloseBehavior>
}

export async function setCloseBehavior(value: CloseBehavior): Promise<CloseBehavior> {
  const bridge = wailsAppBridge()
  if (!bridge?.SetCloseBehavior) throw new Error('关闭行为仅在 Genesis 桌面客户端中可用')
  return bridge.SetCloseBehavior(value) as Promise<CloseBehavior>
}

export async function providerProfiles(): Promise<ProviderProfiles> {
  const bridge = wailsAppBridge()
  if (!bridge?.ProviderProfiles) throw new Error('模型配置仅在 Genesis 桌面客户端中可用')
  return bridge.ProviderProfiles() as Promise<ProviderProfiles>
}

export async function importProviderTemplate(templateID: string, apiKey: string, baseURL = '', modelID = ''): Promise<ProviderImport> {
	const bridge = wailsAppBridge()
	if (!bridge?.ImportProviderTemplate) throw new Error('模型导入仅在 Genesis 桌面客户端中可用')
	return bridge.ImportProviderTemplate(String(templateID || '').trim(), String(apiKey || '').trim(), String(baseURL || '').trim(), String(modelID || '').trim()) as Promise<ProviderImport>
}

export async function setupDeepSeekFlash(apiKey: string): Promise<FirstRunDeepSeek> {
	const bridge = wailsAppBridge()
	if (!bridge?.SetupDeepSeekFlash) throw new Error('DeepSeek 首次配置仅在 Genesis 桌面客户端中可用')
	return bridge.SetupDeepSeekFlash(String(apiKey || '').trim()) as Promise<FirstRunDeepSeek>
}

export async function rotateProviderCredential(profileID: string, secret: string): Promise<ProviderCredentialRotation> {
  const bridge = wailsAppBridge()
  if (!bridge?.RotateProviderCredential) throw new Error('模型凭据仅在 Genesis 桌面客户端中可用')
  return bridge.RotateProviderCredential(String(profileID || '').trim(), String(secret || '')) as Promise<ProviderCredentialRotation>
}

export async function verifyProvider(modelRole: string, profileID: string): Promise<ProviderVerification> {
  const bridge = wailsAppBridge()
  if (!bridge?.VerifyProvider) throw new Error('模型验证仅在 Genesis 桌面客户端中可用')
  return bridge.VerifyProvider(String(modelRole || '').trim(), String(profileID || '').trim()) as Promise<ProviderVerification>
}

export async function localModelStatus(): Promise<LocalModelStatus> {
  const bridge = wailsAppBridge()
  if (!bridge?.LocalModelStatus) throw new Error('本地模型控制仅在 Genesis 桌面客户端中可用')
  return bridge.LocalModelStatus() as Promise<LocalModelStatus>
}

export async function desktopRuntimeConfig(): Promise<DesktopRuntimeConfig> {
  const bridge = wailsAppBridge()
  if (!bridge?.DesktopConfig) throw new Error('桌面运行状态仅在 Genesis 桌面客户端中可用')
  return bridge.DesktopConfig() as Promise<DesktopRuntimeConfig>
}

export async function startLocalModel(): Promise<LocalModelStatus> {
  const bridge = wailsAppBridge()
  if (!bridge?.StartLocalModel) throw new Error('本地模型控制仅在 Genesis 桌面客户端中可用')
  return bridge.StartLocalModel() as Promise<LocalModelStatus>
}

export async function stopLocalModel(): Promise<LocalModelStatus> {
  const bridge = wailsAppBridge()
  if (!bridge?.StopLocalModel) throw new Error('本地模型控制仅在 Genesis 桌面客户端中可用')
  return bridge.StopLocalModel() as Promise<LocalModelStatus>
}

export async function saveUpdateToken(token: string): Promise<boolean> {
  const bridge = wailsAppBridge()
  if (!bridge?.SaveUpdateToken) throw new Error('更新令牌仅在 Genesis 桌面客户端中可用')
  return bridge.SaveUpdateToken(String(token || '').trim()) as Promise<boolean>
}

export async function checkForUpdate(): Promise<DesktopUpdate> {
  const bridge = wailsAppBridge()
  if (!bridge?.CheckForUpdate) throw new Error('更新检查仅在 Genesis 桌面客户端中可用')
  return bridge.CheckForUpdate() as Promise<DesktopUpdate>
}

export async function installUpdate(update: DesktopUpdate): Promise<void> {
  const bridge = wailsAppBridge()
  if (!bridge?.InstallUpdate) throw new Error('更新安装仅在 Genesis 桌面客户端中可用')
  await bridge.InstallUpdate(update)
}

export function kernelConfig(storage: Pick<Storage, 'getItem'> | null = safeLocalStorage()): KernelConfig {
  return {
    baseUrl: String(storage?.getItem(baseUrlKey) ?? 'http://127.0.0.1:8765').trim(),
    runtimeToken: String(storage?.getItem(runtimeTokenKey) ?? '').trim(),
  }
}

export function saveKernelConfig(config: KernelConfig, storage: Pick<Storage, 'setItem'> | null = safeLocalStorage()) {
  storage?.setItem(baseUrlKey, normalizedBaseUrl(config.baseUrl))
  storage?.setItem(runtimeTokenKey, String(config.runtimeToken || '').trim())
}

export async function getReady(config = kernelConfig()) {
  const bridge = wailsAppBridge()
  if (bridge?.Ready) return bridge.Ready() as Promise<Record<string, unknown>>
  return requestKernel(config, '/ready')
}

export async function getCapabilities(config = kernelConfig()) {
  return requestKernel(config, '/capabilities')
}

export async function listSessions(config = kernelConfig()) {
  const bridge = wailsAppBridge()
  if (bridge?.ListSessions) return bridge.ListSessions() as Promise<SessionListResponse>
  return requestKernel<SessionListResponse>(config, '/sessions')
}

export async function searchSessions(config: KernelConfig, query: string, limit = 0) {
  const q = String(query || '').trim()
  if (!q) throw new Error('search query is required')
  const boundedLimit = Number.isFinite(limit) && limit > 0 ? Math.trunc(limit) : 0
  const bridge = wailsAppBridge()
  if (bridge?.SearchSessions) return bridge.SearchSessions(q, boundedLimit) as Promise<SessionSearchResponse>
  const params = new URLSearchParams({ q })
  if (boundedLimit > 0) params.set('limit', String(boundedLimit))
  return requestKernel<SessionSearchResponse>(config, `/sessions/search?${params.toString()}`)
}

export async function getTimeline(config: KernelConfig, sessionId: string) {
  const session = requiredSessionId(sessionId)
  const bridge = wailsAppBridge()
  if (bridge?.ReadTimeline) return bridge.ReadTimeline(session) as Promise<KernelTimeline>
  return requestKernel<KernelTimeline>(config, `/sessions/${encodeURIComponent(session)}/timeline`)
}

export async function getSession(config: KernelConfig, sessionId: string) {
  const session = requiredSessionId(sessionId)
  const bridge = wailsAppBridge()
  if (bridge?.ReadSession) return bridge.ReadSession(session) as Promise<SessionProjection>
  return requestKernel<SessionProjection>(config, `/sessions/${encodeURIComponent(session)}`)
}

export async function getSessionAgentInvocations(config: KernelConfig, sessionId: string) {
  const session = requiredSessionId(sessionId)
  return requestKernel<AgentInvocationProjection[]>(config, `/sessions/${encodeURIComponent(session)}/agent-invocations`)
}

export async function getSessionTaskGraphs(config: KernelConfig, sessionId: string) {
  const session = requiredSessionId(sessionId)
  return requestKernel<TaskGraphProjection[]>(config, `/sessions/${encodeURIComponent(session)}/task-graphs`)
}

export async function getAgentInvocationChildConversation(config: KernelConfig, invocationId: string) {
  const invocation = String(invocationId || '').trim()
  if (!invocation) throw new Error('invocation id is required')
  return requestKernel<AgentInvocationChildConversation>(config, `/agent-invocations/${encodeURIComponent(invocation)}/child-conversation`)
}

export async function bindSessionWorkspace(config: KernelConfig, sessionId: string, kind: SessionWorkspaceKind, root = '') {
  const session = requiredSessionId(sessionId)
  const bridge = wailsAppBridge()
  if (bridge?.BindSessionWorkspace) return bridge.BindSessionWorkspace(session, kind, String(root || '').trim()) as Promise<SessionProjection>
  return requestKernel<SessionProjection>(config, `/sessions/${encodeURIComponent(session)}/workspace`, {
    method: 'POST',
    body: JSON.stringify({ kind, root: String(root || '').trim() }),
  })
}

export async function bindSessionModel(config: KernelConfig, sessionId: string, profileId: string) {
  const session = requiredSessionId(sessionId)
  const profile = String(profileId || '').trim()
  if (!profile) throw new Error('请选择模型')
  const bridge = wailsAppBridge()
  if (bridge?.BindSessionModel) return bridge.BindSessionModel(session, profile) as Promise<SessionProjection>
  return requestKernel<SessionProjection>(config, `/sessions/${encodeURIComponent(session)}/model`, {
    method: 'POST',
    body: JSON.stringify({ profile_id: profile }),
  })
}

export async function pickProjectDirectory(): Promise<ProjectDirectorySelection | null> {
  const bridge = wailsAppBridge()
  if (!bridge?.PickProjectDirectory) throw new Error('项目目录选择仅在 Genesis 桌面客户端中可用')
  return bridge.PickProjectDirectory()
}

export async function createTaskWorkspace(sessionId: string): Promise<TaskWorkspaceSelection> {
  const session = requiredSessionId(sessionId)
  const bridge = wailsAppBridge()
  if (!bridge?.CreateTaskWorkspace) throw new Error('任务工作区创建仅在 Genesis 桌面客户端中可用')
  return bridge.CreateTaskWorkspace(session)
}

export async function createProjectWorkspace(name: string): Promise<ProjectWorkspaceSelection> {
  const normalized = String(name || '').trim()
  if (!normalized) throw new Error('项目名称不能为空')
  const bridge = wailsAppBridge()
  if (!bridge?.CreateProjectWorkspace) throw new Error('项目工作区创建仅在 Genesis 桌面客户端中可用')
  return bridge.CreateProjectWorkspace(normalized)
}

export async function loadDesktopCatalog(): Promise<DesktopCatalogProjection | null> {
  const bridge = wailsAppBridge()
  if (!bridge?.LoadDesktopCatalog) return null
  return bridge.LoadDesktopCatalog() as Promise<DesktopCatalogProjection>
}

export async function saveDesktopCatalog(catalog: DesktopCatalogProjection): Promise<void> {
  const bridge = wailsAppBridge()
  if (!bridge?.SaveDesktopCatalog) return
  await bridge.SaveDesktopCatalog(catalog)
}

export async function getTimelineDetail(config: KernelConfig, sessionId: string, detailRef: string) {
  const session = requiredSessionId(sessionId)
  const bridge = wailsAppBridge()
  if (bridge?.ReadTimelineDetail) return bridge.ReadTimelineDetail(session, detailRef) as Promise<KernelTimelineDetail>
  return requestKernel<KernelTimelineDetail>(
    config,
    `/sessions/${encodeURIComponent(session)}/timeline/details/${encodeURIComponent(detailRef)}`,
  )
}

export async function pickMaterialFile(): Promise<MaterialFileSelection | null> {
  const bridge = wailsAppBridge()
  if (!bridge?.PickMaterialFile) return null
  return bridge.PickMaterialFile()
}

export async function pickMaterialDirectory(): Promise<MaterialFileSelection | null> {
  const bridge = wailsAppBridge()
  if (!bridge?.PickMaterialDirectory) return null
  return bridge.PickMaterialDirectory()
}

export async function uploadMaterial(config: KernelConfig, sessionId: string, file: File | MaterialFileSelection) {
  const session = requiredSessionId(sessionId)
  const bridge = wailsAppBridge()
  if (bridge?.UploadMaterial && isMaterialFileSelection(file)) {
    return bridge.UploadMaterial({
      session_id: session,
      purpose: 'source_analysis',
      file_path: file.file_path,
    }) as Promise<MaterialIntakeProjection>
  }
  if (!(file instanceof File)) throw new Error('desktop material upload requires a selected file path')
  const form = new FormData()
  form.set('session_id', session)
  form.set('purpose', 'source_analysis')
  form.set('file', file)
  return requestKernel<MaterialIntakeProjection>(config, '/materials/upload', {
    method: 'POST',
    body: form,
  })
}

export async function submitTurn(config: KernelConfig, sessionId: string, text: string, idempotencyKey: string) {
  const session = requiredSessionId(sessionId)
  const bridge = wailsAppBridge()
  if (bridge?.SubmitTurn) return bridge.SubmitTurn(session, text, idempotencyKey) as Promise<TurnResponse>
  return requestKernel<TurnResponse>(config, '/turn', {
    method: 'POST',
    body: JSON.stringify({
      session_id: session,
      idempotency_key: idempotencyKey,
      input_items: [{ type: 'text', text }],
    }),
  })
}

export async function interruptSession(config: KernelConfig, sessionId: string, reason = 'user requested stop') {
  const session = requiredSessionId(sessionId)
  return requestKernel<Record<string, unknown>>(config, `/sessions/${encodeURIComponent(session)}/interrupt`, {
    method: 'POST',
    body: JSON.stringify({ reason: String(reason || 'user requested stop').trim() || 'user requested stop' }),
  })
}

export async function submitTurnStream(
  config: KernelConfig,
  sessionId: string,
  text: string,
  idempotencyKey: string,
  onEvent: (event: TurnStreamEvent) => void,
) {
  const session = requiredSessionId(sessionId)
  const bridge = wailsAppBridge()
  if (bridge?.SubmitTurnStream) {
    let finalResponse: TurnResponse | null = null
    let streamError: Error | null = null
    const unsubscribe = await subscribeTurnStreamEvents(idempotencyKey, (event) => {
      onEvent(event)
      if (event.type === 'turn_failed' && event.error) {
        streamError = new Error([event.error.code, event.error.message].filter(Boolean).join(': ') || 'turn failed')
      }
      if (isTerminalTurnStreamEvent(event) && event.response) finalResponse = event.response
    })
    try {
      const payload = await bridge.SubmitTurnStream(session, text, idempotencyKey) as { response?: TurnResponse }
      if (streamError) throw streamError
      finalResponse = finalResponse ?? payload.response ?? null
      if (!finalResponse) throw new Error('stream ended before terminal turn event')
      return finalResponse
    } finally {
      unsubscribe()
    }
  }
  const response = await fetch(kernelUrl(config.baseUrl, '/turn/stream'), {
    method: 'POST',
    headers: kernelHeaders(config.runtimeToken, JSON.stringify({})),
    body: JSON.stringify({
      session_id: session,
      idempotency_key: idempotencyKey,
      input_items: [{ type: 'text', text }],
    }),
  })
  if (!response.ok) {
    throw new Error(await responseMessage(response))
  }
  if (!response.body) {
    throw new Error('streaming response body is unavailable')
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let finalResponse: TurnResponse | null = null
  for (;;) {
    const { done, value } = await reader.read()
    if (value) {
      buffer += decoder.decode(value, { stream: !done })
      const lines = buffer.split(/\r?\n/)
      buffer = lines.pop() ?? ''
      for (const line of lines) {
        const event = parseTurnStreamEvent(line)
        if (!event) continue
        onEvent(event)
        if (event.type === 'turn_failed' && event.error) {
          throw new Error([event.error.code, event.error.message].filter(Boolean).join(': ') || 'turn failed')
        }
        if (isTerminalTurnStreamEvent(event) && event.response) finalResponse = event.response
      }
    }
    if (done) break
  }
  const tail = parseTurnStreamEvent(buffer)
  if (tail) {
    onEvent(tail)
    if (tail.type === 'turn_failed' && tail.error) {
      throw new Error([tail.error.code, tail.error.message].filter(Boolean).join(': ') || 'turn failed')
    }
    if (isTerminalTurnStreamEvent(tail) && tail.response) finalResponse = tail.response
  }
  if (!finalResponse) throw new Error('stream ended before terminal turn event')
  return finalResponse
}

function isTerminalTurnStreamEvent(event: TurnStreamEvent): boolean {
  return event.type === 'turn_completed' || event.type === 'turn_paused'
}

export async function listApprovals(config: KernelConfig, status = 'pending') {
  return requestKernel<ApprovalListResponse>(config, `/approvals?status=${encodeURIComponent(status)}`)
}

export async function decideApproval(config: KernelConfig, approvalId: string, decision: ApprovalDecision, reason: string) {
  const bridge = wailsAppBridge()
  if (bridge?.DecideApproval) return bridge.DecideApproval(approvalId, decision, String(reason || decision).trim()) as Promise<ApprovalProjection>
  return requestKernel<ApprovalProjection>(config, `/approvals/${encodeURIComponent(approvalId)}/decision`, {
    method: 'POST',
    body: JSON.stringify({
      decision,
      decision_authority: 'desktop:operator',
      decision_reason: String(reason || decision).trim(),
      decision_evidence_ref: 'approval:desktop-operator',
    }),
  })
}

export async function enableSessionDebug(config: KernelConfig, sessionId: string) {
  const session = requiredSessionId(sessionId)
  const bridge = wailsAppBridge()
  if (bridge?.EnableSessionDebug) return bridge.EnableSessionDebug(session) as Promise<SessionDebugExport>
  return requestKernel<SessionDebugExport>(config, `/sessions/${encodeURIComponent(session)}/debug/enable`, {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

export async function getSessionDebug(config: KernelConfig, sessionId: string) {
  const session = requiredSessionId(sessionId)
  const bridge = wailsAppBridge()
  if (bridge?.ExportSessionDebug) return bridge.ExportSessionDebug(session) as Promise<SessionDebugExport>
  return requestKernel<SessionDebugExport>(config, `/sessions/${encodeURIComponent(session)}/debug`)
}

export async function compactSessionContext(config: KernelConfig, sessionId: string) {
  const session = requiredSessionId(sessionId)
  const bridge = wailsAppBridge()
  if (bridge?.CompactSessionContext) return bridge.CompactSessionContext(session) as Promise<ContextCompactionResponse>
  return requestKernel<ContextCompactionResponse>(config, `/sessions/${encodeURIComponent(session)}/context/compact`, {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

export async function requestKernel<T = Record<string, unknown>>(config: KernelConfig, path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(kernelUrl(config.baseUrl, path), {
    ...init,
    headers: mergeHeaders(kernelHeaders(config.runtimeToken, init.body), init.headers),
  })
  if (!response.ok) {
    throw new Error(await responseMessage(response))
  }
  return (await response.json()) as T
}

type MaterialBridgeRequest = {
  session_id: string
  purpose: string
  file_path: string
}

type WailsAppBridge = {
	DesktopConfig?: () => Promise<unknown>
  Ready?: () => Promise<unknown>
  ListSessions?: () => Promise<unknown>
  SearchSessions?: (query: string, limit: number) => Promise<unknown>
  SubmitTurn?: (sessionId: string, text: string, idempotencyKey: string) => Promise<unknown>
  SubmitTurnStream?: (sessionId: string, text: string, idempotencyKey: string) => Promise<unknown>
  ReadTimeline?: (sessionId: string) => Promise<unknown>
  ReadTimelineDetail?: (sessionId: string, detailRef: string) => Promise<unknown>
  ReadSession?: (sessionId: string) => Promise<unknown>
  BindSessionWorkspace?: (sessionId: string, kind: SessionWorkspaceKind, root: string) => Promise<unknown>
	BindSessionModel?: (sessionId: string, profileId: string) => Promise<unknown>
  PickProjectDirectory?: () => Promise<ProjectDirectorySelection | null>
  CreateTaskWorkspace?: (sessionId: string) => Promise<TaskWorkspaceSelection>
  CreateProjectWorkspace?: (name: string) => Promise<ProjectWorkspaceSelection>
  LoadDesktopCatalog?: () => Promise<DesktopCatalogProjection>
  SaveDesktopCatalog?: (catalog: DesktopCatalogProjection) => Promise<unknown>
  DecideApproval?: (approvalId: string, decision: ApprovalDecision, reason: string) => Promise<unknown>
  PickMaterialFile?: () => Promise<MaterialFileSelection | null>
  PickMaterialDirectory?: () => Promise<MaterialFileSelection | null>
  UploadMaterial?: (request: MaterialBridgeRequest) => Promise<unknown>
  EnableSessionDebug?: (sessionId: string) => Promise<unknown>
  ExportSessionDebug?: (sessionId: string) => Promise<unknown>
  CompactSessionContext?: (sessionId: string) => Promise<unknown>
  LocalModelStatus?: () => Promise<unknown>
  StartLocalModel?: () => Promise<unknown>
  StopLocalModel?: () => Promise<unknown>
  ProviderProfiles?: () => Promise<unknown>
	ImportProviderTemplate?: (templateID: string, apiKey: string, baseURL: string, modelID: string) => Promise<unknown>
	SetupDeepSeekFlash?: (apiKey: string) => Promise<unknown>
	RotateProviderCredential?: (profileID: string, secret: string) => Promise<unknown>
  VerifyProvider?: (modelRole: string, profileID: string) => Promise<unknown>
  SaveUpdateToken?: (token: string) => Promise<boolean>
  CheckForUpdate?: () => Promise<unknown>
  InstallUpdate?: (update: DesktopUpdate) => Promise<unknown>
  CloseBehavior?: () => Promise<unknown>
  SetCloseBehavior?: (value: CloseBehavior) => Promise<unknown>
}

type WailsRuntimeBridge = {
  EventsOnMultiple?: (eventName: string, callback: (...payload: unknown[]) => void, maxCallbacks: number) => () => void
}

declare global {
  var go: { main?: { App?: WailsAppBridge } } | undefined
  interface Window {
    runtime?: WailsRuntimeBridge
  }
}

function wailsAppBridge(): WailsAppBridge | undefined {
  return globalThis.go?.main?.App
}

function wailsRuntimeBridge(): WailsRuntimeBridge | undefined {
  return typeof window === 'undefined' ? undefined : window.runtime
}

function isMaterialFileSelection(value: File | MaterialFileSelection): value is MaterialFileSelection {
  return !(value instanceof File) && typeof value.file_path === 'string'
}

export function parseTurnStreamEvent(line: string): TurnStreamEvent | null {
  const trimmed = String(line || '').trim()
  if (!trimmed) return null
  return JSON.parse(trimmed) as TurnStreamEvent
}

export function turnStreamEventName(idempotencyKey: string) {
  const key = String(idempotencyKey || '').trim() || 'anonymous'
  return `genesis:turn-stream:${key}`
}

async function subscribeTurnStreamEvents(idempotencyKey: string, onEvent: (event: TurnStreamEvent) => void) {
  const runtime = wailsRuntimeBridge()
  if (!runtime?.EventsOnMultiple) throw new Error('Wails runtime event bridge is unavailable')
  return runtime.EventsOnMultiple(turnStreamEventName(idempotencyKey), (payload: unknown) => {
    if (!payload || typeof payload !== 'object') return
    onEvent(payload as TurnStreamEvent)
  }, -1)
}

export function kernelUrl(baseUrl: string, path: string) {
  const base = normalizedBaseUrl(baseUrl)
  return `${base}/${path.replace(/^\/+/, '')}`
}

export function requiredSessionId(sessionId: string) {
  const session = String(sessionId || '').trim()
  if (!session) throw new Error('session id is required')
  return session
}

function normalizedBaseUrl(baseUrl: string) {
  return String(baseUrl || 'http://127.0.0.1:8765').trim().replace(/\/+$/, '')
}

function kernelHeaders(token: string, body?: BodyInit | null) {
  const headers = new Headers()
  const trimmed = String(token || '').trim()
  if (trimmed) headers.set('Authorization', `Bearer ${trimmed}`)
  if (typeof body === 'string') headers.set('Content-Type', 'application/json')
  return headers
}

function mergeHeaders(base: Headers, extra: HeadersInit | undefined) {
  const merged = new Headers(base)
  if (!extra) return merged
  new Headers(extra).forEach((value, key) => merged.set(key, value))
  return merged
}

async function responseMessage(response: Response) {
  try {
    const payload = await response.json() as { error?: { code?: string; message?: string } }
    const code = String(payload.error?.code ?? '').trim()
    const message = String(payload.error?.message ?? '').trim()
    return [code, message].filter(Boolean).join(': ') || `HTTP ${response.status}`
  } catch {
    return `HTTP ${response.status}`
  }
}

function safeLocalStorage() {
  try {
    return typeof window === 'undefined' ? null : window.localStorage
  } catch {
    return null
  }
}
