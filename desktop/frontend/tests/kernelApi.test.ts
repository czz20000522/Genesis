import assert from 'node:assert/strict'
import { kernelConfig, kernelUrl, saveKernelConfig } from '../src/api/kernelApi.ts'

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
