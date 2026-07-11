import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { applyProviderRole, bindSessionWorkspace, compactSessionContext, decideApproval, enableSessionDebug, getSession, getSessionDebug, getTimeline, getTimelineDetail, kernelConfig, kernelUrl, listSessions, parseTurnStreamEvent, providerProfiles, rotateProviderCredential, saveKernelConfig, searchSessions, submitTurn, submitTurnStream, turnStreamEventName, uploadMaterial, verifyProvider } from '../src/api/kernelApi.ts'
import { approvalSummary } from '../src/approvalView.ts'
import { compactionSummary } from '../src/compactionView.ts'
import { debugExportText, debugSummary } from '../src/debugExport.ts'
import { connectionErrorLabel, readinessLabel, sessionLabel, sessionStatus } from '../src/display.ts'
import { materialIntakeSummary } from '../src/materialIntake.ts'
import { isBlankSessionDraft } from '../src/sessionDraft.ts'
import { timelineDetailEntries } from '../src/timelineDetail.ts'
import { timelineRows } from '../src/timelineView.ts'
import { loadSessionCatalog, recordSessionCatalogEntry } from '../src/sessionCatalog.ts'

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
const conversationSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'ConversationPane.vue'), 'utf8')
const providerPanelSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'ProviderPanel.vue'), 'utf8')
assert.equal(appSource.includes('listApprovals'), false, 'App.vue must not load global pending approvals into the current conversation')
assert.equal(appSource.includes('localSessions'), false, 'App.vue must not keep frontend-local sessions as history truth')
assert.equal(conversationSource.includes('approvals: ApprovalProjection[]'), true, 'ConversationPane must render a current-session approval queue')
assert.equal(conversationSource.includes('<details v-if="row.kind === \'reasoning\'"'), true, 'ConversationPane must keep reasoning in its own collapsed disclosure')
assert.equal(apiSource.includes('KernelRequest'), false, 'desktop production bridge must not expose a generic HTTP proxy')
assert.equal(apiSource.includes('content_base64'), false, 'desktop upload bridge must not pass whole files as base64')
assert.equal(providerPanelSource.includes('type="password"'), true, 'provider key input must not render as plain text')
assert.equal(providerPanelSource.includes('localStorage'), false, 'provider key input must not persist in browser storage')
assert.equal(appSource.includes('const localModelStarting = ref(false)'), true, 'App.vue must track explicit local-model startup')
assert.equal(appSource.includes('localModelStarting.value = true'), true, 'App.vue must mark local-model startup before awaiting the bridge')

globalThis.go = {
  main: {
    App: {
      ProviderProfiles: async () => ({
        profiles: [{ profile_id: 'cloud-glm', model_id: 'glm-5-2', protocol: 'openai-chat-completions', credential_present: true }],
        role_bindings: { coordinator: 'cloud-glm' },
      }),
      RotateProviderCredential: async (profileID: string, secret: string) => ({ profile_id: profileID, credential_present: secret.length > 0 }),
      ApplyProviderRole: async (modelRole: string, profileID: string) => ({ status: 'owned_kernel_restarted', binding: { model_role: modelRole, profile_id: profileID } }),
      VerifyProvider: async (modelRole: string, profileID: string) => ({ readiness: 'ready', model_role: modelRole, profile_id: profileID, model: 'glm-5-2' }),
    },
  },
}
const configuredProviders = await providerProfiles()
assert.equal(configuredProviders.profiles?.[0]?.model_id, 'glm-5-2')
assert.deepEqual(await rotateProviderCredential('cloud-glm', 'one-shot-key'), { profile_id: 'cloud-glm', credential_present: true })
assert.equal((await applyProviderRole('coordinator', 'cloud-glm')).status, 'owned_kernel_restarted')
assert.equal((await verifyProvider('coordinator', 'cloud-glm')).model, 'glm-5-2')
globalThis.go = undefined

saveKernelConfig({ baseUrl: 'http://127.0.0.1:8765/', runtimeToken: ' token ' }, storage)
assert.deepEqual(kernelConfig(storage), {
  baseUrl: 'http://127.0.0.1:8765',
  runtimeToken: 'token',
})

recordSessionCatalogEntry({ sessionId: 'project-session', kind: 'project', root: 'D:\\repo-a', name: 'repo-a' }, storage)
recordSessionCatalogEntry({ sessionId: 'task-session', kind: 'task', root: 'C:\\Users\\Tomczz\\Documents\\Genesis\\task-session' }, storage)
recordSessionCatalogEntry({ sessionId: 'chat-session', kind: 'chat' }, storage)
assert.deepEqual(loadSessionCatalog(storage), [
  { sessionId: 'project-session', kind: 'project', root: 'D:\\repo-a', name: 'repo-a' },
  { sessionId: 'task-session', kind: 'task', root: 'C:\\Users\\Tomczz\\Documents\\Genesis\\task-session', name: '' },
  { sessionId: 'chat-session', kind: 'chat', root: '', name: '' },
])

assert.equal(kernelUrl('http://127.0.0.1:8765/', '/ready'), 'http://127.0.0.1:8765/ready')
assert.equal(kernelUrl('', 'capabilities'), 'http://127.0.0.1:8765/capabilities')
assert.equal(readinessLabel('ready'), '已连接')
assert.equal(readinessLabel('serving-ready'), '已连接')
assert.equal(readinessLabel('not_ready'), '连接失败')
assert.equal(readinessLabel('unchecked'), '未连接')
assert.equal(sessionLabel('desktop-full-id'), '当前会话')
assert.equal(sessionLabel(''), '未选择会话')
assert.equal(sessionStatus('a', 'a'), '正在使用')
assert.equal(sessionStatus('a', 'b'), '未打开')
assert.equal(connectionErrorLabel('Failed to fetch'), '连接失败，请检查本地服务')
assert.equal(connectionErrorLabel(''), '')
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
