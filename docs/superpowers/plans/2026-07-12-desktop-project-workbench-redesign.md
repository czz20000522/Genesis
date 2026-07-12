# Desktop Project Workbench Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use inline execution task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the directory-shaped session rail and diagnostic-heavy chat UI with durable project containers and a conversation-first desktop workbench.

**Architecture:** Keep project metadata in the existing desktop-local catalog and make its project record explicit. The Go Wails bridge creates the default project workspace; the frontend binds sessions through the existing kernel workspace endpoint. Rendering remains projection-only: the frontend receives timeline rows and presents normal, processing, reasoning, and failed-send states without creating kernel facts.

**Tech Stack:** Go/Wails, Vue 3, TypeScript, plain CSS, existing kernel HTTP bridge.

---

### Task 1: Make project metadata explicit and durable

**Files:**
- Modify: `desktop/frontend/src/sessionCatalog.ts`
- Modify: `desktop/frontend/tests/kernelApi.test.ts`
- Modify: `desktop/app.go`
- Modify: `desktop/app_test.go`

- [ ] **Step 1: Write catalog tests for a project and its sessions**

```ts
recordProjectCatalogEntry({ projectId: 'project-a', name: 'A', root: 'C:\\Users\\Tomczz\\Documents\\Genesis\\A' }, storage)
recordSessionCatalogEntry({ sessionId: 'session-a', kind: 'project', projectId: 'project-a' }, storage)
assert.equal(loadProjectCatalog(storage)[0]?.projectId, 'project-a')
assert.equal(loadSessionCatalog(storage)[0]?.projectId, 'project-a')
```

- [ ] **Step 2: Add separate `DesktopProjectCatalogEntry` storage**

```ts
export type DesktopProjectCatalogEntry = { projectId: string; name: string; root: string }
export type DesktopSessionCatalogEntry = { sessionId: string; kind: DesktopSessionKind; projectId?: string }
```

Use a second local-storage key for projects. Keep task roots and chat entries in
the session catalog; require `projectId` only for project sessions.

- [ ] **Step 3: Add a tested Wails workspace creator**

```go
type ProjectWorkspaceSelection struct { Root string `json:"root"` }
func (a *App) CreateProjectWorkspace(name string) (*ProjectWorkspaceSelection, error)
```

It creates exactly one directory under `Documents/Genesis/<sanitized-name>` and
rejects empty names, path separators, `.` and `..`.

- [ ] **Step 4: Run focused verification**

Run: `go test ./... -count=1` from `desktop` and `npm test` from
`desktop/frontend`.

### Task 2: Replace directory picking with project creation actions

**Files:**
- Modify: `desktop/frontend/src/api/kernelApi.ts`
- Modify: `desktop/frontend/src/App.vue`
- Modify: `desktop/frontend/src/components/SessionRail.vue`
- Modify: `desktop/frontend/tests/kernelApi.test.ts`

- [ ] **Step 1: Add the typed Wails bridge**

```ts
export type ProjectWorkspaceSelection = { root: string }
export async function createProjectWorkspace(name: string): Promise<ProjectWorkspaceSelection>
```

The bridge must use `App.CreateProjectWorkspace` when Wails is available and
must not expose a generic filesystem bridge.

- [ ] **Step 2: Change project session activation to use a project record**

```ts
async function createEmptyProject(name: string) {
  const workspace = await createProjectWorkspace(name)
  const project = { projectId: newDesktopProjectId(), name, root: workspace.root }
  recordProjectCatalogEntry(project)
  await bindAndActivateSession({ kind: 'project', projectId: project.projectId, root: project.root })
}
```

`Use existing folder` obtains a root from the existing directory picker, records
the same project shape, and then creates its first session. New project
sessions reuse that project's root.

- [ ] **Step 3: Replace the rail actions**

```vue
<div class="rail-section-heading"><span>项目</span><button aria-label="新建项目">+</button></div>
<menu v-if="projectMenuOpen"><button>新建空白项目</button><button>使用现有文件夹</button></menu>
```

Remove `打开项目`; render projects first and sessions nested under the matching
project. Keep independent Tasks and Chats as distinct persistent sections.

- [ ] **Step 4: Run the frontend tests and typecheck**

Run: `npm test; npm run build` from `desktop/frontend`.

### Task 3: Rebuild the rail and compact top bar

**Files:**
- Modify: `desktop/frontend/src/components/SessionRail.vue`
- Modify: `desktop/frontend/src/components/KernelTopBar.vue`
- Modify: `desktop/frontend/src/styles.css`

- [ ] **Step 1: Remove permanent search and diagnostic clutter**

Replace the always-visible `label.session-search` with a compact toggle and a
single input shown only while searching. Remove the giant search surface and
all composer-level connection/session labels.

- [ ] **Step 2: Render status as one compact indicator**

```vue
<button class="connection-indicator" :aria-label="connectionLabel" @click="$emit('toggleInspector')">
  <span :class="`connection-dot connection-dot--${readiness}`" />
</button>
```

The top bar shows identity and provider summary; errors move to the relevant
message or settings surface.

- [ ] **Step 3: Apply the locked visual system**

Use warm-neutral canvas/rail lightness steps, 6/10/14px radius tiers, one
deep-teal accent, 40px targets, and only opacity/transform transitions. Delete
the old rail/composer status styles instead of layering overrides over them.

- [ ] **Step 4: Run the frontend build**

Run: `npm run build` from `desktop/frontend`.

### Task 4: Make message, processing, reasoning, and failure states intentional

**Files:**
- Modify: `desktop/frontend/src/App.vue`
- Modify: `desktop/frontend/src/components/ConversationPane.vue`
- Modify: `desktop/frontend/src/styles.css`
- Modify: `desktop/frontend/tests/kernelApi.test.ts`

- [ ] **Step 1: Add a failed-send view state**

```ts
const failedSend = ref<{ text: string; message: string } | null>(null)
```

Clear the composer only after a request is accepted for streaming. On failure,
remove the optimistic live row, keep one failed-send card with retry, and never
also render the same text in the composer and timeline.

- [ ] **Step 2: Render a compact inline retry card**

```vue
<article v-if="failedSend" class="send-failure" role="alert">
  <strong>未能发送</strong><p>{{ failedSend.message }}</p><button @click="$emit('retry')">重试</button>
</article>
```

Place it at the transcript/composer boundary, not in the global top bar.

- [ ] **Step 3: Rebalance transcript hierarchy**

User turns use compact right-side bubbles. Assistant Markdown is left-side
content without a generic card. Processing and persisted reasoning stay as
collapsed disclosure rows associated with the assistant response.

- [ ] **Step 4: Run frontend tests and build**

Run: `npm test; npm run build` from `desktop/frontend`.

### Task 5: Verify the complete packaged desktop

**Files:**
- Modify: `desktop/DESIGN.md`
- Test: `desktop/app_test.go`

- [ ] **Step 1: Refresh the active design contract**

Replace the walking-skeleton visual language with the project-workbench
contract and list project metadata as desktop user-space state.

- [ ] **Step 2: Run full checks**

```powershell
git diff --check
Set-Location desktop; go test ./... -count=1; go build ./...
Set-Location frontend; npm test; npm run build
Set-Location ..\..; $env:Path = 'C:\Program Files (x86)\NSIS;' + $env:Path; .\scripts\build_desktop_release.ps1
```

- [ ] **Step 3: Perform manual acceptance**

Create an empty project, create two sessions under it, restart Genesis, and
confirm both sessions share the project root. Bind one existing folder, send a
message, observe a failed-send retry path, and verify a normal streamed reply
with collapsible reasoning.
