# Design

## Source of truth
- Status: Active
- Last refreshed: 2026-06-29
- Primary product surfaces: Wails desktop app
- Evidence reviewed: `desktop/frontend/src/App.vue`, `desktop/frontend/src/api/kernelApi.ts`, GitHub issue #25

## Brand
- Personality: local-first, technical, calm, operator-grade
- Trust signals: visible readiness, explicit session controls, inspectable kernel evidence
- Avoid: marketing layout, decorative visuals, hidden side effects

## Product goals
- Goals: make one session workable from connection through conversation, approval, materials, debug, compaction, and details
- Non-goals: WebUI polish, multi-session database, provider context construction in the frontend
- Success signals: user can send a turn and inspect kernel projections without leaving the primary workbench

## Personas and jobs
- Primary personas: local operator, developer/operator validating Genesis
- User jobs: connect to kernel, submit session turns, approve effects, inspect details, export debug evidence
- Key contexts of use: desktop local runtime, single active session

## Information architecture
- Primary navigation: no router; one workbench screen
- Core routes/screens: top kernel bar, session rail, conversation pane, action dock, inspector drawer
- Content hierarchy: conversation first, controls second, diagnostics in inspector

## Design principles
- Principle 1: desktop is a shell over kernel HTTP primitives, not a truth owner
- Principle 2: approvals and diagnostics are action/detail surfaces, not chat messages
- Tradeoffs: dense utility layout over visual polish until the kernel/product flow stabilizes

## Visual language
- Color: restrained neutral surface with blue primary action and red denial action
- Typography: system UI, monospace only for refs/commands/output
- Spacing/layout rhythm: compact panels with 8px radius
- Shape/radius/elevation: bordered panels, no nested decorative cards
- Motion: none
- Imagery/iconography: none in the walking skeleton

## Components
- Existing components to reuse: `kernelApi.ts` as the HTTP choke point, view helpers for approvals/material/debug/compaction/timeline detail
- New/changed components: `KernelTopBar`, `SessionRail`, `ConversationPane`, `ActionDock`, `InspectorDrawer`
- Variants and states: ready/not ready, empty timeline, pending approvals, debug export available, compaction result
- Token/component ownership: frontend-local CSS only; no design-system dependency

## Accessibility
- Target standard: usable keyboard/tab flow for walking skeleton
- Keyboard/focus behavior: native form controls and buttons
- Contrast/readability: neutral backgrounds and clear error color
- Screen-reader semantics: labels remain explicit
- Reduced motion and sensory considerations: no animation

## Responsive behavior
- Supported breakpoints/devices: desktop-first, single-column fallback below 980px
- Layout adaptations: topbar and three-column workbench collapse to one column
- Touch/hover differences: native controls only

## Interaction states
- Loading: button-triggered actions currently rely on kernel response/error state
- Empty: empty timeline/detail/approval panels stay secondary
- Error: shared topbar error line
- Success: kernel projections appear in conversation or inspector
- Disabled: debug download disabled until export exists
- Offline/slow network, if applicable: readiness/error surface reports request failure

## Content voice
- Tone: concise operator labels
- Terminology: kernel, session, timeline, approval, inspector
- Microcopy rules: expose kernel status/results, do not narrate internal implementation

## Implementation constraints
- Framework/styling system: Vue/Vite/Wails, plain CSS
- Design-token constraints: no new dependency or global state library
- Performance constraints: no raw event rendering in desktop
- Compatibility constraints: all kernel calls stay behind `desktop/frontend/src/api/kernelApi.ts`
- Test/screenshot expectations: static guard prevents component-level `fetch`

## Open questions
- [ ] Final visual language after the workbench flow survives live use / owner: product / impact: polish timing
