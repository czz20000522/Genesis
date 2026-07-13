# Desktop Agent Workspace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (\`- [ ]\`) syntax for tracking.

**Goal:** Replace the chat-first desktop surface with a truthful, calm Agent Workspace while preserving session, model, and kernel projection behavior.

**Architecture:** \`App.vue\` stays the only frontend orchestration layer. It supplies existing projections and actions to a new workspace composition; a pure adapter maps \`TimelineRow\` values into activity presentation without producing runtime facts.

**Tech Stack:** Vue 3 Composition API, TypeScript, Wails, Element Plus, plain CSS, Node assertions, \`vue-tsc\`, Vite.

---

## File map

| File | Responsibility |
| --- | --- |
| \`desktop/frontend/src/workspaceActivity.ts\` | Pure conversion from existing timeline rows to workspace activity rows. |
| \`desktop/frontend/src/components/AgentWorkspace.vue\` | Composes header, timeline, empty state, and composer. |
| \`desktop/frontend/src/components/WorkspaceHeader.vue\` | Session identity and truthful metadata. |
| \`desktop/frontend/src/components/WorkspaceTimeline.vue\` | Durable activity, output, decision, and failure presentation. |
| \`desktop/frontend/src/components/TaskComposer.vue\` | Session-scoped task draft and emitted user intent. |
| \`desktop/frontend/src/components/SessionRail.vue\` | Compact Project / Task / Chat hierarchy. |
| \`desktop/frontend/src/components/KernelTopBar.vue\` | Quiet global controls only. |
| \`desktop/frontend/src/App.vue\` | Existing kernel calls and state wiring. |
| \`desktop/frontend/src/styles.css\` | Workspace tokens and layout; removal of chat-first rules. |
| \`desktop/frontend/tests/workspaceActivity.test.ts\` | Activity mapping regression tests. |
| \`desktop/frontend/tests/kernelApi.test.ts\` | Component-boundary and no-direct-fetch regression tests. |

### Task 1: Add a truthful activity adapter

**Files:**
- Create: \`desktop/frontend/src/workspaceActivity.ts\`
- Create: \`desktop/frontend/tests/workspaceActivity.test.ts\`
- Modify: \`desktop/frontend/package.json\`

- [ ] **Step 1: Write the failing mapping test**

\`\`\`ts
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
\`\`\`

- [ ] **Step 2: Verify the absent module fails**

Run: \`node --experimental-strip-types ./tests/workspaceActivity.test.ts\` from \`desktop/frontend\`.

Expected: module-not-found failure for \`workspaceActivity.ts\`.

- [ ] **Step 3: Implement the adapter**

\`\`\`ts
import type { TimelineRow } from './timelineView'

export type WorkspaceActivityRow = TimelineRow & {
  presentation: 'brief' | 'thinking' | 'work' | 'decision' | 'output'
  label: string
}

export function workspaceActivity(rows: TimelineRow[]): WorkspaceActivityRow[] {
  return rows.map((row) => ({ ...row, presentation: presentationFor(row), label: labelFor(row) }))
}
\`\`\`

Map only \`user\`, \`reasoning\`, \`processing\`, \`action\`, and \`assistant\` to
\`brief\`, \`thinking\`, \`work\`, \`decision\`, and \`output\`. Return \`Thinking\`,
\`Needs your decision\`, \`已完成\` only for a succeeded processing row, \`正在处理\`,
\`Result\`, or \`Task\`; never return fabricated planning/review labels.

- [ ] **Step 4: Register and run the test**

Add \`node --experimental-strip-types ./tests/workspaceActivity.test.ts\` to the
existing \`test\` script before \`kernelApi.test.ts\`.

Run: \`npm test\` from \`desktop/frontend\`.

Expected: all assertion suites pass.

- [ ] **Step 5: Commit the adapter**

Run: \`git add desktop/frontend/src/workspaceActivity.ts desktop/frontend/tests/workspaceActivity.test.ts desktop/frontend/package.json\`.

Run: \`git commit -m "Define Desktop activity projection" -m "Use existing timeline facts to render workspace activity." -m "Constraint: The adapter must not create phase facts" -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: npm test"\`.

### Task 2: Introduce focused workspace components

**Files:**
- Create: \`desktop/frontend/src/components/WorkspaceHeader.vue\`
- Create: \`desktop/frontend/src/components/WorkspaceTimeline.vue\`
- Create: \`desktop/frontend/src/components/TaskComposer.vue\`
- Create: \`desktop/frontend/src/components/AgentWorkspace.vue\`
- Modify: \`desktop/frontend/tests/kernelApi.test.ts\`

- [ ] **Step 1: Add failing component-boundary assertions**

\`\`\`ts
const workspaceSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'AgentWorkspace.vue'), 'utf8')
const timelineSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'WorkspaceTimeline.vue'), 'utf8')
const composerSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'TaskComposer.vue'), 'utf8')
assert.equal(workspaceSource.includes('<WorkspaceHeader'), true)
assert.equal(workspaceSource.includes('<WorkspaceTimeline'), true)
assert.equal(workspaceSource.includes('<TaskComposer'), true)
assert.equal(timelineSource.includes('workspaceActivity'), true)
assert.equal(composerSource.includes('selectedModelProfile'), true)
assert.equal(/\bfetch\s*\(/.test(workspaceSource + timelineSource + composerSource), false)
\`\`\`

- [ ] **Step 2: Confirm the assertions fail**

Run: \`npm test\` from \`desktop/frontend\`.

Expected: failure opening the absent workspace component.

- [ ] **Step 3: Implement component contracts**

\`AgentWorkspace.vue\` receives existing rows, approvals, error, draft, profiles,
selected model, and session metadata. It renders a header, either timeline or a
quiet empty state, and a permanent composer. It emits existing
send/retry/interrupt/attachment/approval/model/detail/inspector actions.

\`WorkspaceHeader.vue\` displays only real title, session kind, bound root,
selected model, and connection state. \`WorkspaceTimeline.vue\` imports
\`workspaceActivity\`, renders reasoning inside \`<details>\`, activity as quiet
rows, output with \`AssistantMessage\`, errors near the affected activity, and
existing approval actions. \`TaskComposer.vue\` moves current Element Plus
textarea/select/button controls and emits intent only.

- [ ] **Step 4: Verify the component boundary**

Run: \`npm test; npm run build\` from \`desktop/frontend\`.

Expected: Node tests, \`vue-tsc --noEmit\`, and Vite pass.

- [ ] **Step 5: Commit the component boundary**

Run: \`git add desktop/frontend/src/components/AgentWorkspace.vue desktop/frontend/src/components/WorkspaceHeader.vue desktop/frontend/src/components/WorkspaceTimeline.vue desktop/frontend/src/components/TaskComposer.vue desktop/frontend/tests/kernelApi.test.ts\`.

Run: \`git commit -m "Expose Desktop task workspace" -m "Separate task composition from timeline projection." -m "Constraint: Child components make no kernel calls" -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: npm test; npm run build"\`.

### Task 3: Integrate projections and retire the chat-first pane

**Files:**
- Modify: \`desktop/frontend/src/App.vue\`
- Delete: \`desktop/frontend/src/components/ConversationPane.vue\`
- Modify: \`desktop/frontend/tests/kernelApi.test.ts\`

- [ ] **Step 1: Add failing integration guardrails**

\`\`\`ts
assert.equal(appSource.includes("import AgentWorkspace from './components/AgentWorkspace.vue'"), true)
assert.equal(appSource.includes("import ConversationPane from './components/ConversationPane.vue'"), false)
assert.equal(appSource.includes('<AgentWorkspace'), true)
assert.equal(readdirSync(join(import.meta.dirname, '..', 'src', 'components')).includes('ConversationPane.vue'), false)
\`\`\`

- [ ] **Step 2: Confirm the old-pane assertion fails**

Run: \`npm test\` from \`desktop/frontend\`.

Expected: \`ConversationPane\` assertion fails.

- [ ] **Step 3: Rewire without moving orchestration**

Pass existing \`displayedRows\`, \`retryText\`, \`selectedFileName\`,
\`selectedFileIsDirectory\`, \`providerProfilesState\`, \`sessionModelProfile\`, and
existing action functions to \`AgentWorkspace\`. Retain all HTTP calls, stream
reconciliation, session model binding, approval decisions, and local-model
lifecycle in \`App.vue\`. Delete \`ConversationPane.vue\` only after its import and
template use are gone.

- [ ] **Step 4: Run regression checks**

Run: \`npm test; npm run build\` from \`desktop/frontend\`.

Expected: all tests pass and Vite produces a bundle without the legacy pane.

- [ ] **Step 5: Commit integration**

Run: \`git add desktop/frontend/src/App.vue desktop/frontend/src/components/ConversationPane.vue desktop/frontend/tests/kernelApi.test.ts\`.

Run: \`git commit -m "Center Desktop on agent work" -m "Replace the transcript-first composition with the workspace." -m "Constraint: App owns all kernel API calls" -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: npm test; npm run build"\`.

### Task 4: Rebuild navigation and global controls

**Files:**
- Modify: \`desktop/frontend/src/components/SessionRail.vue\`
- Modify: \`desktop/frontend/src/components/KernelTopBar.vue\`
- Modify: \`desktop/frontend/tests/kernelApi.test.ts\`

- [ ] **Step 1: Add navigation assertions**

\`\`\`ts
const railSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'SessionRail.vue'), 'utf8')
const topbarSource = readFileSync(join(import.meta.dirname, '..', 'src', 'components', 'KernelTopBar.vue'), 'utf8')
assert.equal(railSource.includes('项目'), true)
assert.equal(railSource.includes('任务'), true)
assert.equal(railSource.includes('聊天'), true)
assert.equal(topbarSource.includes('readinessLabel(readiness)'), true)
assert.equal(topbarSource.includes('error:'), false)
\`\`\`

- [ ] **Step 2: Confirm the header check fails**

Run: \`npm test\` from \`desktop/frontend\`.

Expected: failure because the old top bar accepts a duplicate error prop.

- [ ] **Step 3: Simplify the rail and top bar**

Keep existing catalog grouping and emitted commands. Replace textual glyphs
\`▱\`/\`+\` with installed Element Plus icons. Remove the top-bar error prop;
retain only compact accessible connection, Model, and inspector actions. Active
rail state is a pale background with dark text, never an opaque accent card.

- [ ] **Step 4: Verify navigation changes**

Run: \`npm test; npm run build\` from \`desktop/frontend\`.

Expected: all tests and compilation pass.

- [ ] **Step 5: Commit navigation**

Run: \`git add desktop/frontend/src/components/SessionRail.vue desktop/frontend/src/components/KernelTopBar.vue desktop/frontend/tests/kernelApi.test.ts\`.

Run: \`git commit -m "Constrain Desktop workspace navigation" -m "Make project hierarchy quiet and legible." -m "Constraint: Project, Task, and Chat semantics stay intact" -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: npm test; npm run build"\`.

### Task 5: Replace chat/SaaS CSS with the workspace system

**Files:**
- Modify: \`desktop/frontend/src/styles.css\`
- Modify: \`desktop/frontend/tests/kernelApi.test.ts\`

- [ ] **Step 1: Add failing visual-system assertions**

\`\`\`ts
assert.equal(stylesSource.includes('--app: #fafafa;'), true)
assert.equal(stylesSource.includes('--ink: #111111;'), true)
assert.equal(stylesSource.includes('--muted: #6b7280;'), true)
assert.equal(stylesSource.includes('--primary: #007a62;'), true)
assert.equal(stylesSource.includes('.agent-workspace'), true)
assert.equal(stylesSource.includes('.chat-bubble'), false)
assert.equal(stylesSource.includes('.rail .session-link-active'), true)
\`\`\`

- [ ] **Step 2: Confirm the visual contract fails**

Run: \`npm test\` from \`desktop/frontend\`.

Expected: token and workspace-selector assertions fail.

- [ ] **Step 3: Rewrite the shared styles**

Use required exact colors, 12–16px elevated surfaces, 248px rail, 880px reading
width, compact top bar, and a demand-opened 340–440px inspector. Delete
\`chat-*\`, \`button:not(.el-button)\`, strong colored-card, and obsolete transcript
selectors instead of stacking overrides. Keep focus styling, 120–140ms motion,
pointer-only hover, and reduced-motion behavior.

- [ ] **Step 4: Build and prove legacy selectors are gone**

Run: \`npm test; npm run build; rg -n "chat-bubble|empty-chat|composer-wrap|conversation" src\` from \`desktop/frontend\`.

Expected: tests/build pass and the search finds no live legacy selector.

- [ ] **Step 5: Commit the visual system**

Run: \`git add desktop/frontend/src/styles.css desktop/frontend/tests/kernelApi.test.ts\`.

Run: \`git commit -m "Refine Desktop workspace visual system" -m "Replace legacy chat styling with the approved workbench system." -m "Constraint: Use one restrained accent only" -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: npm test; npm run build"\`.

### Task 6: Validate package behavior and record evidence

**Files:**
- Modify if evidence changes: \`docs/operations/application-issues.md\`
- Modify if evidence changes: \`docs/implementation-plans/*desktop*.md\`

- [ ] **Step 1: Run full repository verification**

Run: \`git diff --check\` from repository root.

Run: \`go test ./... -count=1\` from repository root.

Run: \`go build ./...\` from repository root.

Run: \`npm test; npm run build\` from \`desktop/frontend\`.

Expected: every command exits zero.

- [ ] **Step 2: Build the installer without installing it**

Run: \`powershell -ExecutionPolicy Bypass -File scripts/build_desktop_release.ps1\` from repository root.

Expected: an NSIS installer contains the desktop binary and its private kernel runtime.

- [ ] **Step 3: Perform the visual user-path inspection**

Launch the packaged UI; create/select a Chat, Task, and Project; create a second
Project session; select DeepSeek Flash for each; send a small task; open activity
details; exercise retry; open/close inspector; restart and reopen each session.
Record any human-only verification as \`待人工测试\`; do not claim GUI proof from
source inspection.

- [ ] **Step 4: Carry out the closing drift check**

Compare the result to \`docs/requirements/desktop-agent-workspace.md\`,
\`docs/design/desktop-agent-workspace.md\`, the approved design spec, and this
plan. Remove temporary CSS layers, fake phases, raw service errors, and
chat-first components. Update the live issue ledger only for an unresolved
user-visible gap.

## Plan self-review

- Tasks 1–3 cover truthful timeline/workspace composition and preserve kernel ownership.
- Task 4 covers Project/Task/Chat navigation and global state disclosure.
- Task 5 covers the visual system and deletes legacy styling rather than layering it.
- Task 6 supplies package, repository, and honest visual-proof closure.

