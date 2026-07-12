# Genesis Desktop Workbench Redesign

## Goal

Turn the desktop shell into a quiet local-first workbench: a durable project
tree on the left and a readable conversation surface on the right. The shell
continues to project kernel facts; it does not become a second source of truth.

## Confirmed product model

- A **Project** is a durable desktop container, not a directory-picker result.
- A project has a name, a workspace root, and many sessions. Its sessions share
  that root.
- **New empty project** creates its root under
  `C:\Users\Tomczz\Documents\Genesis\<project-name>`.
- **Use existing folder** creates the same project record but binds that folder
  as its root.
- A **Task** remains a durable standalone session with its own default
  workspace. A **Chat** remains a durable local transcript without a project
  workspace.
- Project metadata is desktop user-space state. Kernel session and workspace
  bindings remain kernel projections.

## Visual thesis

Codex-style project navigation meets a ChatGPT-style conversation canvas:
quiet warm-neutral surfaces, text-first hierarchy, and one restrained deep-teal
action color. This is a desktop workbench, not a dashboard or a marketing page.

## Layout

- The rail contains compact actions, an on-demand one-line search, a **Projects**
  section, and a **Tasks** section. It has no permanent large search field.
- The Projects heading owns an overflow control and a `+` control. The `+`
  menu has exactly **New empty project** and **Use existing folder**.
- Project rows expand to reveal their sessions. There is no `Open project`
  primary action.
- The top bar has only the current project/session identity, a compact model
  selector, and a small connection indicator. Diagnostics stay in Settings.
- The conversation is the dominant canvas. The composer aligns with the
  transcript rather than the whole window.

## Conversation states

- A sent user message becomes a single right-aligned compact bubble.
- Assistant content is left-aligned readable text with Markdown; processing and
  reasoning are collapsible evidence directly associated with that response.
- A failed send appears beside the pending message with a clear reason and
  retry action. It is not a global red status line and must not leave duplicate
  user text in both transcript and composer.
- The composer contains attachment and send actions only. Connection and
  session-debug labels do not occupy its footer.

## Interaction and accessibility

- Every rail action and menu item has a 40px hit target and a visible keyboard
  focus ring.
- Buttons use a 120ms opacity/transform transition and `scale(.96)` on press;
  reduced-motion disables it.
- Search opens from an icon/action, keeps focus in the input, and renders
  matching sessions in the existing rail list.

## Non-goals

- No component library, router, global state store, or decorative imagery.
- No model-context construction, ledger writes, or permission decisions in the
  frontend.
- No project/task graph ownership change in this visual rewrite.

## Acceptance

- A user can create an empty project or bind an existing folder, create several
  sessions beneath it, restart the desktop app, and see the same project tree.
- The empty state, normal conversation, reasoning/processing, and failed-send
  state are visually distinct without persistent diagnostic clutter.
- The rail has no oversized search area, `Open project` action, composer state
  pills, or raw connection error line.
