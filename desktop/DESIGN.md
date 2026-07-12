# Genesis Desktop Design Contract

## Status and visual thesis

- Status: Active
- Last refreshed: 2026-07-12
- CSS strategy: plain component-local Vue markup with one shared `src/styles.css`
- Visual thesis: a quiet technical workbench — crisp white canvas, graphite
  project rail, and a restrained deep-teal action color. It should feel like a
  place to work for hours, not a dashboard or a provider-control utility.

The first screen must orient the user, show the active model truthfully, and
let them start a conversation. Decorative artwork, gradients, and marketing
hero layouts are deliberately absent.

## Product model and information architecture

- A **Project** is a durable desktop container with one workspace root and
  several sessions that share it.
- A workspace root belongs to at most one Project. Selecting an already-known
  folder opens a new session in that Project instead of creating a duplicate.
- A **Task** is a durable standalone session with its own workspace under
  `C:\Users\Tomczz\Documents\Genesis`.
- A **Chat** is a durable local transcript without a project workspace.
- Project and session catalog metadata is desktop-owned state stored under
  `~/.genesis/desktop/catalog.json`, so it survives WebView cache removal and
  travels with Genesis Home. It is not kernel truth.
- The rail holds entry actions, a demand-opened search field, projects and
  their nested sessions, then standalone tasks and chats.
- The central canvas owns the transcript and composer. Settings contain
  diagnostics; diagnostics never occupy the normal chat surface.
- The composer owns the current session's model selector. The header's
  "模型" action opens provider configuration and readiness; it must never
  claim that a merely highlighted profile is the current session binding.

## Palette and typography

| Token | Value | Role |
| --- | --- | --- |
| Canvas | `#ffffff` | conversation and composer base |
| Rail | `oklch(0.98 0.004 250)` | calm structural separation |
| Ink | `oklch(0.24 0.016 250)` | primary text |
| Muted | `oklch(0.50 0.018 250)` | secondary text and metadata |
| Teal | `oklch(0.49 0.105 168)` | primary action and ready state |
| Amber | `oklch(0.54 0.11 72)` | attention and transitional state |
| Red | `oklch(0.50 0.18 28)` | action failure only |

Use the installed Windows system UI stack (`Bahnschrift`, `Segoe UI`, system
UI) for direct desktop legibility; use Cascadia Mono only for commands and
opaque refs. Headings use tight tracking; operational labels remain compact.

## Components and interaction

- Radius scale: 8px controls, 12px elevated panels, pill only for small
  status metadata.
- All interactive targets are at least 40px where space permits, have an
  explicit focus outline, and press with `scale(.96)`.
- The model sheet is an inline elevated surface below the header, never a
  modal. Cloud profiles can be verified and set as the **global coordinator
  default**. Applying it announces that the owned Genesis service restarts and
  that future, not historical, turns use the selected model.
  A local profile reveals its own start/stop control; selecting it never
  starts llama.cpp automatically.
- The attachment row has separate **Add archive** and **Add folder** actions.
  The desktop packages a selected folder privately before it reaches the
  existing kernel upload route. Source archives omit credentials, VCS metadata,
  and common dependency/build trees; the user sees this policy before sending.
- A send failure is attached to the composer and preserves a retry path. It
  must use a concise, user-facing explanation rather than raw provider or
  socket text.

## Depth, motion, and responsive behavior

Use divider lines for permanent layout boundaries and one soft shadow only for
temporary elevated surfaces (model sheet and composer). Do not create grids of
identical cards. Hover treatment is enabled only on pointer devices. Motion is
limited to opacity/transform at 120–140ms and is disabled by reduced-motion
preferences.

Below 1040px the rail, conversation, and inspector stack; controls retain
their tap targets and no horizontal clipping is allowed.

## Guardrails

- Do not show provider, kernel, token, ledger, or tool vocabulary in the
  normal conversation path.
- Do not provide a default local model binding merely because a local profile
  exists.
- Do not expose local model lifecycle controls while a cloud model is selected.
- Do not let a normal user need a CLI, URL, port, token, or manual restart to
  select a configured cloud model and send a message.
- Keep all kernel HTTP access behind `desktop/frontend/src/api/kernelApi.ts`.
- Desktop projects are user-space metadata; session bindings and transcript
  facts remain kernel projections.
