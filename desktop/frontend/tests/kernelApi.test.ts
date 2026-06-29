import assert from 'node:assert/strict'
import { getTimelineDetail, kernelConfig, kernelUrl, saveKernelConfig } from '../src/api/kernelApi.ts'
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
