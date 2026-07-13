import assert from 'node:assert/strict'
import { workspaceActivity } from '../src/workspaceActivity.ts'

const rows = workspaceActivity([
  { id: 'u', kind: 'user', text: '阅读仓库', meta: '', detailRef: '', detailAvailable: false, turnId: 't', terminalOutcome: '' },
  { id: 'r', kind: 'reasoning', text: '先列出目录', meta: '已思考', detailRef: '', detailAvailable: false, turnId: 't', terminalOutcome: '' },
  { id: 'p', kind: 'processing', text: '执行中', meta: '2 项操作', detailRef: '', detailAvailable: false, turnId: 't', terminalOutcome: 'succeeded' },
  { id: 'a', kind: 'assistant', text: '仓库包含 kernel 与 desktop。', meta: '', detailRef: '', detailAvailable: false, turnId: 't', terminalOutcome: '' },
])

assert.deepEqual(rows.map((row) => row.presentation), ['brief', 'thinking', 'work', 'output'])
assert.equal(rows[2]?.label, '已完成')
assert.equal(rows.some((row) => row.label === 'Planning' || row.label === 'Reviewing'), false)
