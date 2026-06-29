import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { compactSessionContext, decideApproval, enableSessionDebug, getSessionDebug, getTimelineDetail, kernelConfig, kernelUrl, listApprovals, saveKernelConfig, submitTurn, uploadMaterial } from '../src/api/kernelApi.ts'
import { approvalSummary } from '../src/approvalView.ts'
import { compactionSummary } from '../src/compactionView.ts'
import { debugExportText, debugSummary } from '../src/debugExport.ts'
import { materialIntakeSummary } from '../src/materialIntake.ts'
import { timelineDetailEntries } from '../src/timelineDetail.ts'

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

saveKernelConfig({ baseUrl: 'http://127.0.0.1:8765/', runtimeToken: ' token ' }, storage)
assert.deepEqual(kernelConfig(storage), {
  baseUrl: 'http://127.0.0.1:8765',
  runtimeToken: 'token',
})

assert.equal(kernelUrl('http://127.0.0.1:8765/', '/ready'), 'http://127.0.0.1:8765/ready')
assert.equal(kernelUrl('', 'capabilities'), 'http://127.0.0.1:8765/capabilities')

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
  'admitted',
  'source:snapshot:1',
  'source_tree, source_read',
])

let approvalsUrl = ''
globalThis.fetch = async (input, init) => {
  approvalsUrl = String(input)
  requestedAuth = new Headers(init?.headers).get('Authorization') ?? ''
  return new Response(JSON.stringify({
    items: [{
      approval_id: 'approval/needs encoding',
      status: 'pending',
      effect: { tool: 'shell_exec', command_preview: 'echo ok' },
    }],
  }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

try {
  const approvals = await listApprovals({
    baseUrl: 'http://127.0.0.1:8765/',
    runtimeToken: 'secret',
  })

  assert.equal(approvalsUrl, 'http://127.0.0.1:8765/approvals?status=pending')
  assert.equal(requestedAuth, 'Bearer secret')
  assert.equal(approvals.items?.[0]?.approval_id, 'approval/needs encoding')
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
}), ['pending', 'shell_exec', 'echo ok'])

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
  assert.deepEqual(debugSummary(debug), ['ready', '2', 'user_text: 2, skill_index: 1', 'deepseek: 2'])
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
  assert.deepEqual(compactionSummary(compacted), ['admitted', ''])
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
