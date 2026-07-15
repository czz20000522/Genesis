import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { bindSessionModel, bindSessionWorkspace, compactSessionContext, createProjectWorkspace, decideApproval, desktopRuntimeConfig, enableSessionDebug, getAgentInvocationChildConversation, getSession, getSessionAgentInvocations, getSessionDebug, getTimeline, getTimelineDetail, interruptSession, kernelConfig, kernelUrl, listSessions, parseTurnStreamEvent, pickMaterialDirectory, providerProfiles, rotateProviderCredential, saveKernelConfig, searchSessions, setupDeepSeekFlash, submitTurn, submitTurnStream, turnStreamEventName, uploadMaterial, verifyProvider } from '../src/api/kernelApi.ts'
import { approvalSummary } from '../src/approvalView.ts'
import { compactionSummary } from '../src/compactionView.ts'
import { debugExportText, debugSummary } from '../src/debugExport.ts'
import { connectionErrorLabel, operationErrorLabel, readinessLabel, sessionLabel, sessionStatus, turnErrorLabel } from '../src/display.ts'
import { materialIntakeSummary } from '../src/materialIntake.ts'
import { isBlankSessionDraft } from '../src/sessionDraft.ts'
import { timelineDetailEntries } from '../src/timelineDetail.ts'
import { timelineRows } from '../src/timelineView.ts'
import { latestKnownSessionID, loadProjectCatalog, loadSessionCatalog, recordProjectCatalogEntry, recordSessionCatalogEntry, replaceDesktopCatalog } from '../src/sessionCatalog.ts'

const values = new Map<string, string>()
const storage = {
  getItem(key: string) {
    return values.get(key) ?? null
  },
  setItem(key: string, value: string) {
    values.set(key, value)
  },
}

for (const file of vueFiles(join(import.meta.dirname, '..', 'src'))) {
  const source = readFileSync(file, 'utf8')
  assert.equal(/\bfetch\s*\(/.test(source), false, `${file} must use src/api/kernelApi.ts instead of fetch`)
}

const appSource = readFileSync(join(import.meta.dirname, '..', 'src', 'App.vue'), 'utf8')
const apiSource = readFileSync(join(import.meta.dirname, '..', 'src', 'api', 'kernelApi.ts'), 'utf8')
const workspaceSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'AgentWorkspace.vue'), 'utf8')
const workspaceHeaderSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'WorkspaceHeader.vue'), 'utf8')
const workspaceTimelineSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'WorkspaceTimeline.vue'), 'utf8')
const taskComposerSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'TaskComposer.vue'), 'utf8')
const railSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'SessionRail.vue'), 'utf8')
const topbarSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'KernelTopBar.vue'), 'utf8')
const inspectorSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'InspectorDrawer.vue'), 'utf8')
const providerPanelSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'ProviderPanel.vue'), 'utf8')
const stylesSource = readFileSync(join(import.meta.dirname, '..', 'src', 'styles.css'), 'utf8')
const providerControlSource = readFileSync(join(import.meta.dirname, '..', '..', 'provider_control.go'), 'utf8')
assert.equal(workspaceSource.includes('<WorkspaceHeader'), true, 'AgentWorkspace must compose a truthful workspace header')
assert.equal(workspaceSource.includes('<WorkspaceTimeline'), true, 'AgentWorkspace must compose activity separately from its shell')
assert.equal(workspaceSource.includes('<TaskComposer'), true, 'AgentWorkspace must keep task input as a dedicated surface')
assert.equal(workspaceHeaderSource.includes('readinessLabel'), false, 'workspace header must not duplicate the top-bar connection status')
assert.equal(workspaceTimelineSource.includes('workspaceActivity'), true, 'WorkspaceTimeline must render the shared activity projection')
assert.equal(taskComposerSource.includes('selectedModelProfile'), true, 'TaskComposer must expose the current session model')
assert.equal(/\bfetch\s*\(/.test(workspaceSource + workspaceTimelineSource + taskComposerSource), false, 'workspace components must not bypass the kernel API choke point')
assert.equal(appSource.includes("import AgentWorkspace from './components/AgentWorkspace.vue'"), true, 'App.vue must mount the Agent Workspace')
assert.equal(appSource.includes("import ConversationPane from './components/ConversationPane.vue'"), false, 'App.vue must not retain the legacy conversation monolith')
assert.equal(appSource.includes('<AgentWorkspace'), true, 'App.vue must render the Agent Workspace')
assert.equal(readdirSync(join(import.meta.dirname, '..', 'src', 'components')).includes('ConversationPane.vue'), false, 'legacy ConversationPane must be retired after the workspace is wired')
assert.equal(railSource.includes('项目'), true, 'rail must retain project navigation')
assert.equal(railSource.includes('任务'), true, 'rail must retain task navigation')
assert.equal(railSource.includes('聊天'), true, 'rail must retain durable chat navigation')
assert.equal(railSource.includes("openSettings: []"), true, 'rail must own the restrained settings entry')
assert.equal(railSource.includes('<Folder />'), true, 'rail must use the installed icon set for project identity')
assert.equal(topbarSource.includes('readinessLabel(readiness)'), true, 'top bar must retain accessible connection disclosure')
assert.equal(topbarSource.includes('error:'), false, 'top bar must not duplicate a workspace failure state')
assert.equal(topbarSource.includes("收起设置"), false, 'top bar must not duplicate the rail settings entry')
assert.equal(appSource.includes('@open-settings="inspectorOpen = true"'), true, 'rail settings must open the existing contextual inspector')
assert.equal(/\.workspace-empty-state h2\s*\{[^}]*font-size:\s*24px;/.test(stylesSource), true, 'workspace empty state must keep a desktop-scale title instead of a marketing display heading')
assert.equal(appSource.includes('listApprovals'), false, 'App.vue must not load global pending approvals into the current conversation')
assert.equal(appSource.includes('localSessions'), false, 'App.vue must not keep frontend-local sessions as history truth')
assert.equal(workspaceTimelineSource.includes('approvals: ApprovalProjection[]'), true, 'WorkspaceTimeline must render a current-session approval queue')
assert.equal(workspaceTimelineSource.includes('<details class="activity-thinking">'), true, 'WorkspaceTimeline must keep reasoning in its own collapsed disclosure')
assert.equal(taskComposerSource.includes('turnErrorLabel'), true, 'TaskComposer must not expose raw provider errors in turn status')
assert.equal(taskComposerSource.includes('turn_id'), false, 'TaskComposer must not expose kernel turn identifiers as status')
assert.equal(taskComposerSource.includes('选择模型'), true, 'TaskComposer must keep the current session model readable beside the composer')
assert.equal(taskComposerSource.includes("retryText ? '重试这次任务' : '重新连接'"), true, 'TaskComposer must offer a useful recovery action when the desktop loses the kernel connection')
assert.equal(taskComposerSource.includes('<p>{{ error }}</p>'), true, 'TaskComposer must keep the recovery error visible beside its retry action')
assert.equal(workspaceTimelineSource.includes("terminalOutcome === 'succeeded'"), true, 'WorkspaceTimeline must recognize the kernel succeeded outcome when folding completed processing')
assert.equal(appSource.includes("message.toLowerCase().includes('llama.cpp') || message.toLowerCase().includes('connection refused')"), false, 'App.vue must not treat a kernel connection failure as a local-model failure')
assert.equal(appSource.includes("readiness.value = providerReadiness === 'ready' ? 'ready' : 'connected'"), true, 'App.vue must distinguish a reachable Genesis service from an unready model provider')
assert.equal(appSource.includes('async function waitForKernelReady()'), true, 'App.vue must wait through the owned kernel cold start before showing a connection failure')
assert.equal(appSource.includes('desktopRuntimeConfig'), true, 'App.vue must surface a refused unowned kernel endpoint before trying to use it')
assert.equal(appSource.includes('const connected = await waitForKernelReady()'), true, 'App.vue must defer initial session creation until the kernel is reachable')
assert.equal(appSource.includes('async function ensureInitialChatAfterProviderSetup()'), true, 'App.vue must create an empty session after first provider setup without binding a model')
assert.equal(appSource.includes('await ensureInitialChatAfterProviderSetup()'), true, 'provider setup must make the first usable chat available')
assert.equal(appSource.includes('const hasSelectableProviderProfile = computed(() => providerProfilesState.value.some((profile) => profile.credential_present || isLocalProfile(profile)))'), true, 'first-run routing must distinguish a usable model from an uncredentialed profile')
assert.equal(appSource.includes('if (!restored && !sessionId.value && !hasSelectableProviderProfile.value) providerOpen.value = true'), true, 'an unusable Genesis Home must open provider import instead of creating an unbound chat')
assert.equal(appSource.includes('await restoreLatestKnownSession()'), true, 'startup must restore the latest known session before considering a new chat')
assert.equal(appSource.includes('hasSelectableProviderProfile.value && sessionsLoaded.value && sessions.value.length === 0'), true, 'only a successful empty session list with a selectable model may create the first configured chat')
assert.equal(appSource.includes("operationErrorLabel(err, '加载会话列表')"), true, 'App.vue must preserve a confirmed connection when only session listing fails')
assert.equal(appSource.includes("sessionId.value = next\n  resetSessionViewState()\n  await loadTimeline()\n\tawait rememberSessionActivation(next)"), true, 'session selection must use the shared recoverable timeline load instead of a second unhandled model read')
assert.equal(appSource.includes("ElMessage.success('GitHub 更新令牌已保存。')"), true, 'saving an update token must confirm success before the user checks for updates')
assert.equal(inspectorSource.includes('仅用于检查和下载此私有发行；保存后写入本机受保护存储，无需重复输入。'), true, 'update settings must explain why the private-release token is needed and where it is stored')
assert.equal(inspectorSource.includes('readinessLabel(readiness)'), true, 'InspectorDrawer must not show transport state identifiers directly')
assert.equal(inspectorSource.includes('<el-input'), true, 'InspectorDrawer must use the shared input component')
assert.equal(inspectorSource.includes('<el-select'), true, 'InspectorDrawer must use the shared select component')
assert.equal(inspectorSource.includes('<el-button'), true, 'InspectorDrawer must use the shared button component')
assert.equal(apiSource.includes('KernelRequest'), false, 'desktop production bridge must not expose a generic HTTP proxy')
assert.equal(apiSource.includes('applyProviderRole'), false, 'ordinary session model selection must not expose a global coordinator switch')
assert.equal(providerControlSource.includes('ApplyProviderRole'), false, 'desktop must not retain a sidecar-restarting global coordinator mutation')
assert.equal(apiSource.includes('content_base64'), false, 'desktop upload bridge must not pass whole files as base64')
assert.equal(providerPanelSource.includes('type="password"'), true, 'provider key input must not render as plain text')
assert.equal(providerPanelSource.includes('OpenCode Go'), true, 'empty Genesis Home must offer the curated provider templates')
assert.equal(providerPanelSource.includes("$emit('importProvider')"), true, 'provider import action must stay inside the Provider panel')
assert.equal(providerPanelSource.includes('notice && !profiles.length'), true, 'provider import notice must not render twice after a profile is available')
assert.equal(providerPanelSource.includes('当前会话的模型请在输入框旁选择。'), true, 'provider management must not be mistaken for session-scoped model binding')
assert.equal(appSource.includes('importProviderTemplate'), true, 'App.vue must invoke the typed provider-import bridge')
assert.equal(appSource.includes('sessionModelProfile'), true, 'App.vue must keep the selected model on the current session projection')
assert.equal(appSource.includes('回复已完成，但暂时无法刷新会话状态。'), true, 'App.vue must not describe a completed reply as a failed turn when refresh loses the service')
assert.equal(appSource.includes("if (readiness.value !== 'not_ready') await loadTimeline()"), true, 'App.vue must reconcile a completed reply after reconnecting the kernel service')
assert.equal(appSource.includes("error.value = turnErrorLabel(message)\n      if (message.toLowerCase().includes('llama.cpp')) await openProviderPanel()\n      try {\n        timeline.value = await getTimeline(config.value, session)"), true, 'App.vue must refresh durable failed-turn evidence before clearing its optimistic message')
assert.equal(appSource.includes("timeline.value = await getTimeline(config.value, session)\n\t\tliveUserText.value = ''\n\t\tliveAssistantText.value = ''"), true, 'App.vue must clear optimistic messages as soon as the durable timeline is available')
assert.equal(taskComposerSource.includes('selectModel'), true, 'TaskComposer must expose a session-scoped model selector')
assert.equal(providerPanelSource.includes('localStorage'), false, 'provider key input must not persist in browser storage')
assert.equal(appSource.includes('const localModelStarting = ref(false)'), true, 'App.vue must track explicit local-model startup')
assert.equal(appSource.includes('localModelStarting.value = true'), true, 'App.vue must mark local-model startup before awaiting the bridge')
assert.equal(appSource.includes("local_model_endpoint_already_serving"), true, 'App.vue must distinguish an external serving endpoint from a stopped local model')
assert.equal(providerPanelSource.includes('localModelExternallyServing'), true, 'ProviderPanel must prevent a second local-model launch against an external endpoint')
assert.equal(stylesSource.includes('--app: #fafafa;'), true, 'workspace must use the approved soft application background')
assert.equal(stylesSource.includes('--ink: #111111;'), true, 'workspace must use the approved primary text color')
assert.equal(stylesSource.includes('--muted: #6b7280;'), true, 'workspace must use the approved metadata color')
assert.equal(stylesSource.includes('--primary: #007a62;'), true, 'workspace must use one restrained accent color')
assert.equal(stylesSource.includes('.agent-workspace'), true, 'workspace needs a dedicated central canvas')
assert.equal(stylesSource.includes('.chat-bubble'), false, 'legacy chat bubble styling must be deleted')
assert.equal(stylesSource.includes('button:not(.el-button)'), false, 'generic native button styling must not turn navigation rows into action cards')
assert.equal(stylesSource.includes('.session-link-active,\n.session-link-active:hover'), true, 'only the selected session may use the pale teal navigation state')

globalThis.go = {
  main: {
    App: {
      ProviderProfiles: async () => ({
        profiles: [{ profile_id: 'cloud-glm', model_id: 'glm-5-2', protocol: 'openai-chat-completions', credential_present: true }],
        role_bindings: { coordinator: 'cloud-glm' },
      }),
      SetupDeepSeekFlash: async (apiKey: string) => ({ profile_id: 'deepseek-flash', credential_present: apiKey.length > 0 }),
      RotateProviderCredential: async (profileID: string, secret: string) => ({ profile_id: profileID, credential_present: secret.length > 0 }),
      VerifyProvider: async (modelRole: string, profileID: string) => ({ readiness: 'ready', model_role: modelRole, profile_id: profileID, model: 'glm-5-2' }),
		DesktopConfig: async () => ({ sidecar: { ownership: 'unowned', readiness: 'not_ready', reason: 'kernel_already_serving' } }),
    },
  },
}
const configuredProviders = await providerProfiles()
assert.equal(configuredProviders.profiles?.[0]?.model_id, 'glm-5-2')
assert.deepEqual(await setupDeepSeekFlash('one-shot-key'), { profile_id: 'deepseek-flash', credential_present: true })
assert.deepEqual(await rotateProviderCredential('cloud-glm', 'one-shot-key'), { profile_id: 'cloud-glm', credential_present: true })
assert.equal((await verifyProvider('coordinator', 'cloud-glm')).model, 'glm-5-2')
assert.equal((await desktopRuntimeConfig()).sidecar?.reason, 'kernel_already_serving')
globalThis.go = undefined

globalThis.go = {
  main: {
    App: {
      CreateProjectWorkspace: async (name: string) => {
        assert.equal(name, 'alpha')
        return { root: 'C:\\Users\\Tomczz\\Documents\\Genesis\\alpha' }
      },
    },
  },
}
assert.deepEqual(await createProjectWorkspace(' alpha '), { root: 'C:\\Users\\Tomczz\\Documents\\Genesis\\alpha' })
globalThis.go = undefined

globalThis.go = {
  main: {
    App: {
      PickMaterialDirectory: async () => ({ file_path: 'D:\\repo', filename: 'repo', kind: 'directory' }),
    },
  },
}
assert.deepEqual(await pickMaterialDirectory(), { file_path: 'D:\\repo', filename: 'repo', kind: 'directory' })
globalThis.go = undefined

saveKernelConfig({ baseUrl: 'http://127.0.0.1:8765/', runtimeToken: ' token ' }, storage)
assert.deepEqual(kernelConfig(storage), {
  baseUrl: 'http://127.0.0.1:8765',
  runtimeToken: 'token',
})

recordProjectCatalogEntry({ projectId: 'project-a', name: 'repo-a', root: 'D:\\repo-a' }, storage)
recordSessionCatalogEntry({ sessionId: 'project-session', kind: 'project', projectId: 'project-a' }, storage)
recordSessionCatalogEntry({ sessionId: 'task-session', kind: 'task', root: 'C:\\Users\\Tomczz\\Documents\\Genesis\\task-session' }, storage)
recordSessionCatalogEntry({ sessionId: 'chat-session', kind: 'chat' }, storage)
assert.deepEqual(loadProjectCatalog(storage), [
  { projectId: 'project-a', name: 'repo-a', root: 'D:\\repo-a' },
])
assert.deepEqual(loadSessionCatalog(storage), [
  { sessionId: 'project-session', kind: 'project', projectId: 'project-a', root: '', name: '' },
  { sessionId: 'task-session', kind: 'task', projectId: '', root: 'C:\\Users\\Tomczz\\Documents\\Genesis\\task-session', name: '' },
  { sessionId: 'chat-session', kind: 'chat', projectId: '', root: '', name: '' },
])
assert.equal(latestKnownSessionID(loadSessionCatalog(storage), [{ session_id: 'project-session' }, { session_id: 'chat-session' }]), 'chat-session')
assert.equal(latestKnownSessionID(loadSessionCatalog(storage), [{ session_id: 'project-session' }]), 'project-session')
assert.equal(latestKnownSessionID(loadSessionCatalog(storage), []), '')
recordSessionCatalogEntry({ sessionId: 'project-session', kind: 'project', projectId: 'project-a' }, storage)
assert.equal(latestKnownSessionID(loadSessionCatalog(storage), [{ session_id: 'project-session' }, { session_id: 'chat-session' }]), 'project-session')
replaceDesktopCatalog([{ projectId: 'project-b', name: 'repo-b', root: 'D:\\repo-b' }], [{ sessionId: 'chat-b', kind: 'chat' }], storage)
assert.deepEqual(loadProjectCatalog(storage), [{ projectId: 'project-b', name: 'repo-b', root: 'D:\\repo-b' }])
assert.deepEqual(loadSessionCatalog(storage), [{ sessionId: 'chat-b', kind: 'chat', projectId: '', root: '', name: '' }])

assert.equal(kernelUrl('http://127.0.0.1:8765/', '/ready'), 'http://127.0.0.1:8765/ready')
assert.equal(kernelUrl('', 'capabilities'), 'http://127.0.0.1:8765/capabilities')
assert.equal(readinessLabel('ready'), '已连接')
assert.equal(readinessLabel('serving-ready'), '已连接')
assert.equal(readinessLabel('connected'), 'Genesis 已连接，等待模型配置')
assert.equal(readinessLabel('not_ready'), '连接失败')
assert.equal(readinessLabel('unchecked'), '未连接')
assert.equal(sessionLabel('desktop-full-id'), '当前会话')
assert.equal(sessionLabel(''), '未选择会话')
assert.equal(sessionStatus('a', 'a'), '正在使用')
assert.equal(sessionStatus('a', 'b'), '未打开')
assert.equal(connectionErrorLabel('Failed to fetch'), '连接失败，请检查本地服务')
assert.equal(connectionErrorLabel(''), '')
assert.equal(turnErrorLabel('provider_profile_missing'), '请先选择一个模型，然后再发送消息。')
assert.equal(turnErrorLabel('llama.cpp server unavailable: [WinError 10061]'), '本地模型尚未启动。请在“模型”中启动它，或改用云端模型。')
assert.equal(turnErrorLabel('provider error: unauthorized'), '模型凭据不可用。请在“模型”中检查 API Key。')
assert.equal(turnErrorLabel('provider command failed: exit status 1'), '模型服务暂时无法完成此请求。请稍后重试或切换模型。')
assert.equal(operationErrorLabel('llama.cpp server unavailable: [WinError 10061]', '启动本地模型'), '本地模型尚未启动。请在“模型”中启动它，或改用云端模型。')
assert.equal(operationErrorLabel('fetch failed: connection refused', '连接 Genesis 本地服务'), '无法连接 Genesis 本地服务，请稍后重试。')
assert.equal(operationErrorLabel('checksum mismatch', '安装更新'), '更新文件校验失败，请重新检查更新后再试。')
assert.equal(operationErrorLabel('update credential is required', '检查更新'), '此安装来自私有发行。请先保存 GitHub 只读令牌，再检查更新。')
assert.equal(operationErrorLabel('opaque internal exception', '保存设置'), '无法保存设置，请稍后重试。')
assert.equal(isBlankSessionDraft({}), true)
assert.equal(isBlankSessionDraft({ messageText: 'hello' }), false)
assert.equal(isBlankSessionDraft({ selectedFileName: 'package.zip' }), false)
assert.equal(isBlankSessionDraft({ timelineRowCount: 1 }), false)
assert.equal(isBlankSessionDraft({ hasMaterial: true }), false)

let sessionsUrl = ''
let sessionsAuth = ''
const originalFetchForSessions = globalThis.fetch
globalThis.fetch = async (input, init) => {
  sessionsUrl = String(input)
  sessionsAuth = new Headers(init?.headers).get('Authorization') ?? ''
  return new Response(JSON.stringify({
    items: [{ session_id: 'session-2', title: '第二个会话', updated_at: '2026-07-05T01:00:00Z' }],
  }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const sessions = await listSessions({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  })
  assert.equal(sessionsUrl, 'http://127.0.0.1:8765/sessions')
  assert.equal(sessionsAuth, 'Bearer secret')
  assert.equal(sessions.items?.[0]?.session_id, 'session-2')
} finally {
  globalThis.fetch = originalFetchForSessions
}

let workspaceBindingUrl = ''
let workspaceBindingMethod = ''
let workspaceBindingBody: Record<string, unknown> = {}
const originalFetchForWorkspaceBinding = globalThis.fetch
globalThis.fetch = async (input, init) => {
  workspaceBindingUrl = String(input)
  workspaceBindingMethod = String(init?.method ?? '')
  workspaceBindingBody = JSON.parse(String(init?.body ?? '{}')) as Record<string, unknown>
  return new Response(JSON.stringify({ session_id: 'project-session', workspace_mode: 'project' }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

let modelBindingUrl = ''
let modelBindingMethod = ''
let modelBindingBody: Record<string, unknown> = {}
const originalFetchForModelBinding = globalThis.fetch
globalThis.fetch = async (input, init) => {
  modelBindingUrl = String(input)
  modelBindingMethod = String(init?.method ?? '')
  modelBindingBody = JSON.parse(String(init?.body ?? '{}')) as Record<string, unknown>
  return new Response(JSON.stringify({ session_id: 'project-session', model_profile_id: 'deepseek-flash' }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const bound = await bindSessionModel({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'project/session', 'deepseek-flash')
  assert.equal(modelBindingUrl, 'http://127.0.0.1:8765/sessions/project%2Fsession/model')
  assert.equal(modelBindingMethod, 'POST')
  assert.deepEqual(modelBindingBody, { profile_id: 'deepseek-flash' })
  assert.equal(bound.model_profile_id, 'deepseek-flash')
} finally {
  globalThis.fetch = originalFetchForModelBinding
}

try {
  const bound = await bindSessionWorkspace({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'project/session', 'project', 'D:\\workspace')
  assert.equal(workspaceBindingUrl, 'http://127.0.0.1:8765/sessions/project%2Fsession/workspace')
  assert.equal(workspaceBindingMethod, 'POST')
  assert.deepEqual(workspaceBindingBody, { kind: 'project', root: 'D:\\workspace' })
  assert.equal(bound.workspace_mode, 'project')
} finally {
  globalThis.fetch = originalFetchForWorkspaceBinding
}

let interruptURL = ''
let interruptMethod = ''
let interruptAuth = ''
let interruptBody: Record<string, unknown> = {}
const originalFetchForInterrupt = globalThis.fetch
globalThis.fetch = async (input, init) => {
  interruptURL = String(input)
  interruptMethod = String(init?.method ?? '')
  interruptAuth = new Headers(init?.headers).get('Authorization') ?? ''
  interruptBody = JSON.parse(String(init?.body ?? '{}')) as Record<string, unknown>
  return new Response(JSON.stringify({ session_id: 'project/session', terminal_outcome: 'interrupted' }), {
    status: 202,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  await interruptSession({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'project/session', 'user requested stop')
  assert.equal(interruptURL, 'http://127.0.0.1:8765/sessions/project%2Fsession/interrupt')
  assert.equal(interruptMethod, 'POST')
  assert.equal(interruptAuth, 'Bearer secret')
  assert.deepEqual(interruptBody, { reason: 'user requested stop' })
} finally {
  globalThis.fetch = originalFetchForInterrupt
}

let searchUrl = ''
let searchAuth = ''
const originalFetchForSearch = globalThis.fetch
globalThis.fetch = async (input, init) => {
  searchUrl = String(input)
  searchAuth = new Headers(init?.headers).get('Authorization') ?? ''
  return new Response(JSON.stringify({
    query: 'basalt notes',
    items: [{ session_id: 'session-search', title: 'Basalt notes', match_fields: ['title'], snippet: 'Basalt notes' }],
  }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const search = await searchSessions({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, ' basalt notes ', 5)
  assert.equal(searchUrl, 'http://127.0.0.1:8765/sessions/search?q=basalt+notes&limit=5')
  assert.equal(searchAuth, 'Bearer secret')
  assert.equal(search.query, 'basalt notes')
  assert.equal(search.items?.[0]?.session_id, 'session-search')
} finally {
  globalThis.fetch = originalFetchForSearch
}

let emptySearchRequests = 0
const originalFetchForEmptySearch = globalThis.fetch
globalThis.fetch = async () => {
  emptySearchRequests += 1
  return new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } })
}

try {
  await assert.rejects(
    () => searchSessions({ baseUrl: 'http://127.0.0.1:8765', runtimeToken: '' }, ' '),
    /search query is required/,
  )
  assert.equal(emptySearchRequests, 0)
} finally {
  globalThis.fetch = originalFetchForEmptySearch
}

let emptySessionRequests = 0
const originalFetchForEmptySession = globalThis.fetch
globalThis.fetch = async () => {
  emptySessionRequests += 1
  return new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } })
}

try {
  await assert.rejects(
    () => getTimeline({ baseUrl: 'http://127.0.0.1:8765', runtimeToken: '' }, ' '),
    /session id is required/,
  )
  assert.equal(emptySessionRequests, 0)
} finally {
  globalThis.fetch = originalFetchForEmptySession
}

let requestedUrl = ''
let requestedAuth = ''
const originalFetch = globalThis.fetch
globalThis.fetch = async (input, init) => {
  requestedUrl = String(input)
  requestedAuth = new Headers(init?.headers).get('Authorization') ?? ''
  return new Response(JSON.stringify({
    detail_ref: 'tool/ref',
    item: { kind: 'operation_detail', visible_output: 'done' },
  }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const detail = await getTimelineDetail({
    baseUrl: 'http://127.0.0.1:8765',
    runtimeToken: 'secret',
  }, 'session 1', 'tool/ref')

  assert.equal(requestedUrl, 'http://127.0.0.1:8765/sessions/session%201/timeline/details/tool%2Fref')
  assert.equal(requestedAuth, 'Bearer secret')
  assert.deepEqual(detail.item, { kind: 'operation_detail', visible_output: 'done' })
} finally {
  globalThis.fetch = originalFetch
}

let workerProjectionURL = ''
let workerProjectionAuth = ''
const originalFetchForWorkerProjection = globalThis.fetch
globalThis.fetch = async (input, init) => {
  workerProjectionURL = String(input)
  workerProjectionAuth = new Headers(init?.headers).get('Authorization') ?? ''
  return new Response(JSON.stringify([{ invocation_id: 'worker-1', agent_profile_ref: 'agent_profile:reviewer', status: 'admitted' }]), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const workers = await getSessionAgentInvocations({ baseUrl: 'http://127.0.0.1:8765/', runtimeToken: 'secret' }, 'session/a')
  assert.equal(workerProjectionURL, 'http://127.0.0.1:8765/sessions/session%2Fa/agent-invocations')
  assert.equal(workerProjectionAuth, 'Bearer secret')
  assert.equal(workers[0]?.invocation_id, 'worker-1')
} finally {
  globalThis.fetch = originalFetchForWorkerProjection
}

let childConversationURL = ''
const originalFetchForChildConversation = globalThis.fetch
globalThis.fetch = async (input) => {
  childConversationURL = String(input)
  return new Response(JSON.stringify({ invocation_id: 'worker-1', role_id: 'reviewer', status: 'completed', final: { text: 'review accepted' } }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const child = await getAgentInvocationChildConversation({ baseUrl: 'http://127.0.0.1:8765', runtimeToken: '' }, 'worker/1')
  assert.equal(childConversationURL, 'http://127.0.0.1:8765/agent-invocations/worker%2F1/child-conversation')
  assert.equal(child.final?.text, 'review accepted')
} finally {
  globalThis.fetch = originalFetchForChildConversation
}

let streamUrl = ''
let streamBody: Record<string, unknown> = {}
globalThis.fetch = async (input, init) => {
  streamUrl = String(input)
  requestedAuth = new Headers(init?.headers).get('Authorization') ?? ''
  streamBody = JSON.parse(String(init?.body ?? '{}')) as Record<string, unknown>
  const encoder = new TextEncoder()
  return new Response(new ReadableStream({
    start(controller) {
      controller.enqueue(encoder.encode('{"type":"assistant_delta","delta":"你"}\n'))
      controller.enqueue(encoder.encode('{"type":"assistant_delta","delta":"好"}\n'))
      controller.enqueue(encoder.encode('{"type":"turn_completed","response":{"session_id":"desktop-session","turn_id":"turn-stream","final":{"text":"你好","model":"m"}}}\n'))
      controller.close()
    },
  }), {
    status: 200,
    headers: { 'Content-Type': 'application/x-ndjson' },
  })
}

try {
  const events: string[] = []
  const turn = await submitTurnStream({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'desktop-session', 'hello streaming', 'desktop-idem-stream', (event) => {
    if (event.type === 'assistant_delta') events.push(event.delta ?? '')
  })

  assert.equal(streamUrl, 'http://127.0.0.1:8765/turn/stream')
  assert.equal(requestedAuth, 'Bearer secret')
  assert.deepEqual(streamBody, {
    session_id: 'desktop-session',
    idempotency_key: 'desktop-idem-stream',
    input_items: [{ type: 'text', text: 'hello streaming' }],
  })
  assert.deepEqual(events, ['你', '好'])
  assert.equal(turn.final?.text, '你好')
} finally {
  globalThis.fetch = originalFetch
}

globalThis.fetch = async () => {
  const encoder = new TextEncoder()
  return new Response(new ReadableStream({
    start(controller) {
      controller.enqueue(encoder.encode('{"type":"turn_paused","response":{"session_id":"desktop-session","turn_id":"turn-paused","pause":{"wait_reason":"budget_pause"}}}\n'))
      controller.close()
    },
  }), {
    status: 200,
    headers: { 'Content-Type': 'application/x-ndjson' },
  })
}

try {
  const turn = await submitTurnStream({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'desktop-session', 'pause stream', 'desktop-idem-paused-stream', () => {})

  assert.equal(turn.turn_id, 'turn-paused')
  assert.equal(turn.pause?.wait_reason, 'budget_pause')
} finally {
  globalThis.fetch = originalFetch
}

assert.equal(parseTurnStreamEvent(''), null)
assert.deepEqual(parseTurnStreamEvent('{"type":"assistant_delta","delta":"x"}'), {
  type: 'assistant_delta',
  delta: 'x',
})

function vueFiles(root: string): string[] {
  return readdirSync(root, { withFileTypes: true }).flatMap((entry) => {
    const path = join(root, entry.name)
    if (entry.isDirectory()) return vueFiles(path)
    return entry.isFile() && entry.name.endsWith('.vue') ? [path] : []
  })
}

assert.deepEqual(timelineDetailEntries([
  {
    item_id: 'turn-1',
    kind: 'turn',
    children: [
      { item_id: 'group-1', kind: 'processing_group', detail_ref: 'group-1', detail_available: true },
      { item_id: 'message-1', kind: 'assistant_message' },
      {
        item_id: 'tool-1',
        kind: 'operation_detail',
        detail_available: true,
        children: [{ item_id: 'nested-1', kind: 'operation_detail', detail_ref: 'nested-ref', detail_available: true }],
      },
    ],
  },
]), [
  { detailRef: 'group-1', label: 'processing_group: group-1' },
  { detailRef: 'tool-1', label: 'operation_detail: tool-1' },
  { detailRef: 'nested-ref', label: 'operation_detail: nested-ref' },
])

assert.deepEqual(timelineRows([
  {
    item_id: 'turn-1',
    kind: 'turn',
    children: [
      { item_id: 'user-1', kind: 'user_message', text: 'hello Genesis' },
      {
        item_id: 'processing-1',
        kind: 'processing_group',
        text: '已处理 3s',
        tool_count: 1,
        detail_ref: 'processing-detail',
        detail_available: true,
        children: [{ item_id: 'operation-1', kind: 'operation_detail', output_preview: 'raw tool output' }],
      },
      { item_id: 'reasoning-1', kind: 'assistant_reasoning', text: 'check the available evidence' },
      { item_id: 'assistant-1', kind: 'assistant_message', text: 'done' },
      { item_id: 'action-1', kind: 'user_action_request', text: '需要用户批准', tool: 'shell_exec' },
    ],
  },
]).map((row) => [row.kind, row.text, row.meta]), [
  ['user', 'hello Genesis', ''],
  ['processing', '已处理 3s', '1 项操作'],
  ['reasoning', 'check the available evidence', '已思考'],
  ['assistant', 'done', ''],
  ['action', '需要用户批准', '需要确认'],
])

assert.deepEqual(timelineRows([
  {
    item_id: 'turn-a',
    kind: 'turn',
    children: [
      { item_id: 'action-a', kind: 'user_action_request', text: '需要用户批准', approval_id: 'approval-a', tool: 'shell_exec' },
    ],
  },
  {
    item_id: 'turn-b',
    kind: 'turn',
    children: [
      { item_id: 'assistant-b', kind: 'assistant_message', text: 'session B is clean' },
    ],
  },
]).filter((row) => row.kind === 'action').length, 1, 'timeline actions are projection rows, not a global approval queue')

const failedRows = timelineRows([{
  item_id: 'turn-failed',
  turn_id: 'turn-failed',
  kind: 'turn',
  children: [
    { item_id: 'user-failed', turn_id: 'turn-failed', kind: 'user_message', text: 'retry this request' },
    { item_id: 'processing-failed', turn_id: 'turn-failed', kind: 'processing_group', text: 'provider unavailable', terminal_outcome: 'failed' },
  ],
}])
assert.equal(failedRows[0]?.turnId, 'turn-failed')
assert.equal(failedRows[1]?.terminalOutcome, 'failed')

const safeActionMetadata = JSON.stringify({
  approval_id: 'approval-safe',
  job_id: 'job-safe',
  detail_ref: 'approval-safe',
})
for (const forbidden of ['pid', 'signal', 'process_handle', 'C:\\\\Users\\\\Tomczz', 'evt_raw']) {
  assert.equal(safeActionMetadata.includes(forbidden), false, `action metadata must not leak ${forbidden}`)
}

let directFetchCalls = 0
const originalGo = globalThis.go
globalThis.go = {
  main: {
    App: {
      SubmitTurn: async (sessionId: string, text: string, idempotencyKey: string) => {
        assert.equal(sessionId, 'bridge-session')
        assert.equal(text, 'hello')
        assert.equal(idempotencyKey, 'idem-bridge')
        return { ok: true }
      },
    },
  },
}
globalThis.fetch = async () => {
  directFetchCalls += 1
  return new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } })
}
try {
  const payload = await submitTurn({ baseUrl: 'http://127.0.0.1:8765', runtimeToken: 'secret' }, 'bridge-session', 'hello', 'idem-bridge')
  assert.deepEqual(payload, { ok: true })
  assert.equal(directFetchCalls, 0, 'typed Wails bridge must be the production request choke point when present')
} finally {
  globalThis.go = originalGo
  globalThis.fetch = originalFetch
}

globalThis.go = {
  main: {
    App: {
      SearchSessions: async (query: string, limit: number) => {
        assert.equal(query, 'bridge search')
        assert.equal(limit, 7)
        return { query, items: [{ session_id: 'bridge-session', match_fields: ['title'] }] }
      },
    },
  },
}
globalThis.fetch = async () => {
  directFetchCalls += 1
  return new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } })
}
try {
  directFetchCalls = 0
  const payload = await searchSessions({ baseUrl: 'http://127.0.0.1:8765', runtimeToken: 'secret' }, ' bridge search ', 7)
  assert.equal(payload.items?.[0]?.session_id, 'bridge-session')
  assert.equal(directFetchCalls, 0, 'session search must use typed Wails bridge when present')
} finally {
  globalThis.go = originalGo
  globalThis.fetch = originalFetch
}

const listeners = new Map<string, (...payload: unknown[]) => void>()
const originalWindow = (globalThis as Record<string, unknown>).window
;(globalThis as Record<string, unknown>).window = {
  runtime: {
    EventsOnMultiple(eventName: string, callback: (...payload: unknown[]) => void) {
      listeners.set(eventName, callback)
      return () => listeners.delete(eventName)
    },
  },
}
globalThis.go = {
  main: {
    App: {
      SubmitTurnStream: async (sessionId: string, text: string, idempotencyKey: string) => {
        assert.equal(sessionId, 'bridge-stream-session')
        assert.equal(text, 'hello bridge stream')
        assert.equal(idempotencyKey, 'idem-bridge-stream')
        const listener = listeners.get(turnStreamEventName(idempotencyKey))
        assert.ok(listener, 'Wails stream listener must be registered before bridge call')
        listener({ type: 'assistant_delta', delta: '桥' })
        listener({ type: 'assistant_delta', delta: '接' })
        listener({
          type: 'turn_completed',
          response: {
            session_id: sessionId,
            turn_id: 'turn-bridge-stream',
            final: { text: '桥接', model: 'm' },
          },
        })
        return {
          response: {
            session_id: sessionId,
            turn_id: 'turn-bridge-stream',
            final: { text: '桥接', model: 'm' },
          },
        }
      },
    },
  },
}
globalThis.fetch = async () => {
  directFetchCalls += 1
  return new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } })
}
try {
  directFetchCalls = 0
  const events: string[] = []
  const turn = await submitTurnStream({
    baseUrl: 'http://127.0.0.1:8765',
    runtimeToken: 'secret',
  }, 'bridge-stream-session', 'hello bridge stream', 'idem-bridge-stream', (event) => {
    if (event.type === 'assistant_delta') events.push(event.delta ?? '')
  })
  assert.deepEqual(events, ['桥', '接'])
  assert.equal(turn.final?.text, '桥接')
  assert.equal(directFetchCalls, 0, 'Wails streaming must use typed bridge plus runtime events when present')
  assert.equal(listeners.size, 0, 'stream listener must be unsubscribed after completion')
} finally {
  globalThis.go = originalGo
  ;(globalThis as Record<string, unknown>).window = originalWindow
  globalThis.fetch = originalFetch
}

globalThis.go = {
  main: {
    App: {
      UploadMaterial: async (request: Record<string, unknown>) => {
        assert.deepEqual(request, { session_id: 'session-upload', purpose: 'source_analysis', file_path: 'D:\\tmp\\package.zip' })
        assert.equal(Object.hasOwn(request, 'content_base64'), false)
        return { source_snapshot_ref: 'source:snapshot:bridge' }
      },
    },
  },
}
globalThis.fetch = async () => {
  directFetchCalls += 1
  return new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } })
}
try {
  directFetchCalls = 0
  const projection = await uploadMaterial({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'session-upload', { file_path: 'D:\\tmp\\package.zip', filename: 'package.zip' })
  assert.equal(projection.source_snapshot_ref, 'source:snapshot:bridge')
  assert.equal(directFetchCalls, 0, 'Wails material upload must use typed file-path bridge')
} finally {
  globalThis.go = originalGo
  globalThis.fetch = originalFetch
}

let uploadedUrl = ''
let uploadedSession = ''
let uploadedPurpose = ''
let uploadedFilename = ''
globalThis.fetch = async (input, init) => {
  uploadedUrl = String(input)
  requestedAuth = new Headers(init?.headers).get('Authorization') ?? ''
  const form = init?.body as FormData
  uploadedSession = String(form.get('session_id') ?? '')
  uploadedPurpose = String(form.get('purpose') ?? '')
  uploadedFilename = (form.get('file') as File).name
  return new Response(JSON.stringify({
    admission_result: 'admitted',
    source_snapshot_ref: 'source:snapshot:1',
    available_operations: ['source_tree', 'source_read'],
  }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const projection = await uploadMaterial({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'session-upload', new File(['zip'], 'package.zip'))

  assert.equal(uploadedUrl, 'http://127.0.0.1:8765/materials/upload')
  assert.equal(requestedAuth, 'Bearer secret')
  assert.equal(uploadedSession, 'session-upload')
  assert.equal(uploadedPurpose, 'source_analysis')
  assert.equal(uploadedFilename, 'package.zip')
  assert.equal(projection.source_snapshot_ref, 'source:snapshot:1')
} finally {
  globalThis.fetch = originalFetch
}

assert.deepEqual(materialIntakeSummary({
  admission_result: 'admitted',
  source_snapshot_ref: 'source:snapshot:1',
  available_operations: ['source_tree', 'source_read'],
}), [
  '已添加',
  'source:snapshot:1',
  '查看目录, 读取文件',
])

let approvalsUrl = ''
globalThis.fetch = async (input, init) => {
  approvalsUrl = String(input)
  requestedAuth = new Headers(init?.headers).get('Authorization') ?? ''
  return new Response(JSON.stringify({
    session_id: 'session/approval',
    approvals: [{
      approval_id: 'approval/needs encoding',
      session_id: 'session/approval',
      status: 'pending',
      effect: { tool: 'shell_exec', command_preview: 'echo ok' },
    }],
  }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const session = await getSession({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'session/approval')

  assert.equal(approvalsUrl, 'http://127.0.0.1:8765/sessions/session%2Fapproval')
  assert.equal(requestedAuth, 'Bearer secret')
  assert.equal(session.approvals?.[0]?.approval_id, 'approval/needs encoding')
  assert.equal(session.approvals?.[0]?.session_id, 'session/approval')
} finally {
  globalThis.fetch = originalFetch
}

let decisionUrl = ''
let decisionMethod = ''
let decisionContentType = ''
let decisionBody: Record<string, unknown> = {}
globalThis.fetch = async (input, init) => {
  decisionUrl = String(input)
  decisionMethod = String(init?.method ?? '')
  requestedAuth = new Headers(init?.headers).get('Authorization') ?? ''
  decisionContentType = new Headers(init?.headers).get('Content-Type') ?? ''
  decisionBody = JSON.parse(String(init?.body ?? '{}')) as Record<string, unknown>
  return new Response(JSON.stringify({
    approval_id: 'approval/needs encoding',
    status: 'approved',
  }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const approval = await decideApproval({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'approval/needs encoding', 'approved', 'looks correct')

  assert.equal(decisionUrl, 'http://127.0.0.1:8765/approvals/approval%2Fneeds%20encoding/decision')
  assert.equal(decisionMethod, 'POST')
  assert.equal(requestedAuth, 'Bearer secret')
  assert.equal(decisionContentType, 'application/json')
  assert.deepEqual(decisionBody, {
    decision: 'approved',
    decision_authority: 'desktop:operator',
    decision_reason: 'looks correct',
    decision_evidence_ref: 'approval:desktop-operator',
  })
  assert.equal(approval.status, 'approved')
} finally {
  globalThis.fetch = originalFetch
}

assert.deepEqual(approvalSummary({
  approval_id: 'approval-1',
  status: 'pending',
  effect: {
    tool: 'shell_exec',
    command_preview: 'echo ok',
  },
}), ['等待确认', '运行命令', 'echo ok'])

let debugEnableUrl = ''
let debugEnableMethod = ''
let debugEnableContentType = ''
let debugEnableBody = ''
globalThis.fetch = async (input, init) => {
  debugEnableUrl = String(input)
  debugEnableMethod = String(init?.method ?? '')
  requestedAuth = new Headers(init?.headers).get('Authorization') ?? ''
  debugEnableContentType = new Headers(init?.headers).get('Content-Type') ?? ''
  debugEnableBody = String(init?.body ?? '')
  return new Response(JSON.stringify({ readiness: 'ready' }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const enabled = await enableSessionDebug({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'session/debug')

  assert.equal(debugEnableUrl, 'http://127.0.0.1:8765/sessions/session%2Fdebug/debug/enable')
  assert.equal(debugEnableMethod, 'POST')
  assert.equal(requestedAuth, 'Bearer secret')
  assert.equal(debugEnableContentType, 'application/json')
  assert.equal(debugEnableBody, '{}')
  assert.equal(enabled.readiness, 'ready')
} finally {
  globalThis.fetch = originalFetch
}

let debugExportUrl = ''
globalThis.fetch = async (input, init) => {
  debugExportUrl = String(input)
  requestedAuth = new Headers(init?.headers).get('Authorization') ?? ''
  return new Response(JSON.stringify({
    readiness: 'ready',
    steps: [{ model: 'm1' }, { model: 'm2' }],
    input_kind_counts: { user_text: 2, skill_index: 1 },
    model_counts: { deepseek: 2 },
  }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const debug = await getSessionDebug({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'session/debug')

  assert.equal(debugExportUrl, 'http://127.0.0.1:8765/sessions/session%2Fdebug/debug')
  assert.equal(requestedAuth, 'Bearer secret')
  assert.deepEqual(debugSummary(debug), ['已连接', '2', 'user_text: 2, skill_index: 1', 'deepseek: 2'])
  assert.equal(debugExportText(debug), '{\n  "readiness": "ready",\n  "steps": [\n    {\n      "model": "m1"\n    },\n    {\n      "model": "m2"\n    }\n  ],\n  "input_kind_counts": {\n    "user_text": 2,\n    "skill_index": 1\n  },\n  "model_counts": {\n    "deepseek": 2\n  }\n}')
} finally {
  globalThis.fetch = originalFetch
}

let compactUrl = ''
let compactMethod = ''
let compactContentType = ''
let compactBody = ''
globalThis.fetch = async (input, init) => {
  compactUrl = String(input)
  compactMethod = String(init?.method ?? '')
  requestedAuth = new Headers(init?.headers).get('Authorization') ?? ''
  compactContentType = new Headers(init?.headers).get('Content-Type') ?? ''
  compactBody = String(init?.body ?? '')
  return new Response(JSON.stringify({
    admission_result: 'admitted',
  }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const compacted = await compactSessionContext({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'session/compact')

  assert.equal(compactUrl, 'http://127.0.0.1:8765/sessions/session%2Fcompact/context/compact')
  assert.equal(compactMethod, 'POST')
  assert.equal(requestedAuth, 'Bearer secret')
  assert.equal(compactContentType, 'application/json')
  assert.equal(compactBody, '{}')
  assert.deepEqual(compactionSummary(compacted), ['已开始', ''])
} finally {
  globalThis.fetch = originalFetch
}

let compactAttempts = 0
globalThis.fetch = async () => {
  compactAttempts += 1
  return new Response(JSON.stringify({
    error: { code: 'active_turn_running', message: 'manual compaction requires an idle session' },
  }), {
    status: 409,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  await assert.rejects(
    () => compactSessionContext({ baseUrl: 'http://127.0.0.1:8765', runtimeToken: 'secret' }, 'busy-session'),
    /active_turn_running: manual compaction requires an idle session/,
  )
  assert.equal(compactAttempts, 1)
} finally {
  globalThis.fetch = originalFetch
}

let turnUrl = ''
let turnMethod = ''
let turnContentType = ''
let turnBody: Record<string, unknown> = {}
globalThis.fetch = async (input, init) => {
  turnUrl = String(input)
  turnMethod = String(init?.method ?? '')
  requestedAuth = new Headers(init?.headers).get('Authorization') ?? ''
  turnContentType = new Headers(init?.headers).get('Content-Type') ?? ''
  turnBody = JSON.parse(String(init?.body ?? '{}')) as Record<string, unknown>
  return new Response(JSON.stringify({
    session_id: 'desktop-session',
    turn_id: 'turn-1',
    final: { text: 'hello from kernel', model: 'test-model' },
  }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const turn = await submitTurn({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  }, 'desktop-session', 'hello', 'desktop-idem-1')

  assert.equal(turnUrl, 'http://127.0.0.1:8765/turn')
  assert.equal(turnMethod, 'POST')
  assert.equal(requestedAuth, 'Bearer secret')
  assert.equal(turnContentType, 'application/json')
  assert.deepEqual(turnBody, {
    session_id: 'desktop-session',
    idempotency_key: 'desktop-idem-1',
    input_items: [{ type: 'text', text: 'hello' }],
  })
  assert.equal(turn.final?.text, 'hello from kernel')
} finally {
  globalThis.fetch = originalFetch
}
