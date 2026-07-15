# Design: Desktop Agent Workspace

- **Requirement:** `docs/requirements/desktop-agent-workspace.md`.
- **Status:** implemented; packaged native visual acceptance remains pending.
- **Visual reference:** the user-provided Codex Desktop screenshot is a design
  system reference, not a screenshot to copy.

## Reference scan

Codex's TUI puts editable prompt state in
`codex-rs/tui/src/bottom_pane/chat_composer.rs` and owns its footer modes in
the adjacent `bottom_pane` module. `chatwidget.rs` separately renders committed
history cells and an in-flight active cell. This supports a dedicated Genesis
composer and a timeline that distinguishes durable rows from transient stream
state without duplicating either.

Reasonix's `desktop/app.go` owns a map of `WorkspaceTab` controllers and routes
operations to the active workspace while tagging controller events for the
frontend. Its `ProjectTree.tsx` projects nested project/topic navigation, and
`WorkspacePanel.tsx` is a demand-opened file/detail panel. Genesis aligns with
the separation of active-workspace navigation and contextual inspection, but
does not adopt its tab-controller runtime: Genesis continues to obtain session
and turn truth only through `genesisd` projections.

## Component shape

```text
App.vue
├─ KernelTopBar                 # application/service controls only
├─ SessionRail                  # Project / Task / Chat navigation
└─ AgentWorkspace
   ├─ WorkspaceHeader           # session identity and truthful metadata
   ├─ WorkspaceTimeline         # durable + in-flight projection rows
   │  ├─ WorkspaceEmptyState
   │  ├─ ActivityGroup
   │  ├─ OutputBlock
   │  └─ ActionFailure
   ├─ TaskComposer              # draft, attachment and session-model controls
   └─ InspectorDrawer           # demand-opened third column
```

`App.vue` remains the orchestration boundary for kernel API calls and session
state. It supplies typed projection data and explicit actions to child
components. `AgentWorkspace` composes the header, timeline, and composer;
`WorkspaceTimeline` adapts existing `TimelineRow` kinds into visual activity
groups; `TaskComposer` holds no kernel truth and emits intent only.

## Projection rules

| Existing evidence | Workspace rendering |
| --- | --- |
| User timeline row | Compact task brief, right-aligned only when helpful |
| Reasoning row | Collapsible **Thinking** activity item |
| Processing/tool row | **Working** item with real terminal result |
| Approval projection | **Needs your decision** activity item and actions |
| Assistant row | Primary result/output block |
| Failed terminal row | Inline failed activity item with the existing retry action |
| Task graph/worker projection | Inspector content when present |

There is no branch field in the Genesis desktop projection. The header may show
project name, session kind, workspace root (where bound), selected model, and
kernel connectivity, but it must not display a branch until a read-only,
owned projection exists. Likewise, **Planning** and **Reviewing** appear only
when later kernel evidence can distinguish them; this slice uses no fabricated
phase labels.

## Layout and visual system

- **Shell:** `TopBar` spans the application content. The rail is 248px wide,
  uses a soft off-white surface and no heavy separator. The central canvas is
  white. Inspector width is 340–440px when opened and otherwise removed from
  layout.
- **Workspace:** center content has an 880px reading width and a lower pinned
  composer. The empty state uses one quiet title and compact suggestion rows,
  not marketing cards.
- **Composer:** a 12–16px elevated white surface with a generous task input;
  its footer places attachment, the session model selector, stop/send, and
  actual session metadata in one compact control row.
- **Timeline:** rows use spacing and tiny semantic icons, not high-contrast
  chat bubbles. Assistant output can become a bordered result block; reasoning
  and process details default to collapsed. Failures are local inline callouts.
- **Palette:** canvas `#ffffff`, app background `#fafafa`, primary text
  `#111111`, secondary text `#6b7280`, borders `rgba(17,24,39,.10)`, accent
  `#007a62`. Red and amber are reserved for real error/attention states.
- **Motion:** 120–140ms opacity/transform only, `scale(.96)` press feedback,
  disabled under reduced motion. Hover exists only for pointer devices.

## Failure and recovery

The workspace keeps its current timeline after a background refresh or
auxiliary projection failure. A connection failure is represented by a compact
header state, while a send failure remains attached to the failed turn and
composer retry. The Inspector may expose diagnostic detail only on demand.
The ordinary user path never exposes ports, raw provider errors, or daemon
commands.

## Rejected alternatives

- **CSS-only facelift:** rejected because current `ConversationPane` presents
  a message transcript as the primary information architecture.
- **A permanently open operations dashboard:** rejected because it makes an
  occasional detail view dominate every task.
- **A frontend workflow state machine:** rejected because the desktop may not
  mint or infer kernel execution truth.
