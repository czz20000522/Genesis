# Genesis Desktop Design Contract

## Source of truth
- Status: Active
- Last refreshed: 2026-06-29
- Primary product surface: Wails desktop app
- Evidence reviewed: `desktop/frontend/src/App.vue`, `desktop/frontend/src/api/kernelApi.ts`, GitHub issues #25 and #26

## Brand
- Personality: local-first, technical, calm, operator-grade
- Trust signals: visible connection state, explicit current conversation, inspectable diagnostic evidence
- Avoid: marketing layout, decorative visuals, hidden side effects

## Product goals
- Goals: make the first screen read as a local assistant chat while keeping confirmations, attachments, diagnostics, context cleanup, and details reachable
- Non-goals: WebUI polish, multi-session database, provider context construction in the frontend
- Success signals: user can send a message and inspect processing details without leaving the primary workbench

## Personas and jobs
- Primary personas: local operator, developer/operator validating Genesis
- User jobs: connect to the local service, send messages, confirm risky actions, inspect details, export diagnostic evidence
- Key contexts of use: desktop local runtime, single active conversation

## Information architecture
- Primary navigation: no router; one workbench screen
- Core surfaces: compact connection bar, conversation rail, central transcript/composer, settings and diagnostics drawer
- Content hierarchy: chat transcript and composer first, conversation navigation second, diagnostics in settings

## Design principles
- Principle 1: desktop is a shell over local-service HTTP primitives, not a truth owner
- Principle 2: confirmations and diagnostics are action/detail surfaces, not chat messages
- Tradeoffs: chat-first utility layout over decorative polish until the kernel/product flow stabilizes

## Visual language
- Color: restrained neutral surface with green primary action, green processing state, amber interruption state, and red denial action
- Typography: system UI, monospace only for refs/commands/output
- Spacing/layout rhythm: compact panels with 8px radius
- Shape/radius/elevation: bordered panels, no nested decorative cards
- Motion: none
- Imagery/iconography: none in the walking skeleton

## Components
- Existing components to reuse: `kernelApi.ts` as the HTTP choke point, view helpers for confirmation/material/diagnostic/context/detail projection
- New/changed components: `KernelTopBar`, `SessionRail`, `ConversationPane`, `InspectorDrawer`
- Variants and states: connected/disconnected, empty conversation, pending confirmations, diagnostic export available, context cleanup result
- Token/component ownership: frontend-local CSS only; no design-system dependency

## Accessibility
- Target standard: usable keyboard/tab flow for walking skeleton
- Keyboard/focus behavior: native form controls and buttons
- Contrast/readability: neutral backgrounds and clear error color
- Screen-reader semantics: labels remain explicit
- Reduced motion and sensory considerations: no animation

## Responsive behavior
- Supported breakpoints/devices: desktop-first, single-column fallback below 980px
- Layout adaptations: rail, session workspace, and inspector collapse to one column
- Touch/hover differences: native controls only

## Interaction states
- Loading: live response is transient feedback only; settled display returns to the conversation projection
- Empty: first viewport still shows a chat shell with prompt chips and composer
- Error: shared topbar error line
- Success: local-service projections appear in conversation or diagnostics
- Disabled: diagnostic download disabled until export exists
- Offline/slow network, if applicable: readiness/error surface reports request failure

## Content voice
- Tone: concise operator labels
- Terminology: local service, conversation, processing, confirmation, settings and diagnostics
- Microcopy rules: keep kernel/session/timeline/approval/inspector/tool wording out of the primary user surface; expose internal terms only in API, tests, and diagnostics when necessary

## Implementation constraints
- Framework/styling system: Vue/Vite/Wails, plain CSS
- Design-token constraints: no new dependency or global state library
- Performance constraints: no raw event rendering in desktop
- Compatibility constraints: all local-service calls stay behind `desktop/frontend/src/api/kernelApi.ts`
- Conversation constraints: new conversations use local opaque ids; desktop adds no DB and no new service route
- Local service constraints: when `GENESIS_KERNEL_BASE_URL` is unset, desktop owns a `genesisd` sidecar lifecycle; when it is set, desktop treats the kernel as external and must not start or stop it.
- Test/screenshot expectations: static guard prevents component-level `fetch`

## Open questions
- [ ] Final visual language after the workbench flow survives live use / owner: product / impact: polish timing
