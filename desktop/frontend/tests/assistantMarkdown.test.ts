import assert from 'node:assert/strict'
import { assistantMarkdown } from '../src/assistantMarkdown.ts'

assert.equal(assistantMarkdown('**加粗**').includes('<strong>加粗</strong>'), true)
assert.equal(assistantMarkdown('`shell_exec`').includes('<code>shell_exec</code>'), true)
assert.equal(assistantMarkdown('# 标题').includes('<h1>标题</h1>'), true)
assert.equal(assistantMarkdown('- 条目').includes('<li>条目</li>'), true)

const escaped = assistantMarkdown('<script>alert(1)</script>')
assert.equal(escaped.includes('<script>'), false)
assert.equal(escaped.includes('&lt;script&gt;alert(1)&lt;/script&gt;'), true)

const unsafeLink = assistantMarkdown('[点我](javascript:alert(1))')
assert.equal(unsafeLink.includes('href="javascript:'), false)
assert.equal(unsafeLink.includes('javascript:alert(1)'), true)

const safeLink = assistantMarkdown('[官网](https://example.com/path?q=1)')
assert.equal(safeLink.includes('href="https://example.com/path?q=1"'), true)

const mermaid = assistantMarkdown('```mermaid\nflowchart TD\nA[用户] --> B[结果]\n```')
assert.equal(mermaid.includes('class="mermaid-card"'), true)
assert.equal(mermaid.includes('data-mermaid-code='), true)
assert.equal(mermaid.includes('复制代码'), true)
assert.equal(mermaid.includes('flowchart TD'), true)
