import { runtimeHeaders, runtimeTokenFromStorage, saveRuntimeToken } from '../src/api/kernelApi.ts'

const store = new Map<string, string>()
const storage = {
  getItem: (key: string) => store.get(key) ?? null,
  setItem: (key: string, value: string) => {
    store.set(key, value)
  },
}

saveRuntimeToken(' token-1 ', storage)
equal(runtimeTokenFromStorage(storage), 'token-1')

const jsonHeaders = runtimeHeaders('token-2', JSON.stringify({ ok: true }))
equal(jsonHeaders.get('Authorization'), 'Bearer token-2')
equal(jsonHeaders.get('Content-Type'), 'application/json')

const uploadHeaders = runtimeHeaders('token-3', new FormData())
equal(uploadHeaders.get('Authorization'), 'Bearer token-3')
equal(uploadHeaders.has('Content-Type'), false)

function equal(actual: unknown, expected: unknown) {
  if (actual !== expected) {
    throw new Error(`expected ${String(expected)}, got ${String(actual)}`)
  }
}
