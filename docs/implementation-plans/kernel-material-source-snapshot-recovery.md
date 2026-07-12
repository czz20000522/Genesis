# Implementation Plan: Uploaded Source Snapshot Recovery

- Requirement: `docs/requirements/kernel-material-source-snapshot.md`.
- Design: `docs/design/kernel-material-source-snapshot.md`.

## Reference Scan

Reasonix `desktop.App.AttachDropped` decides whether a dropped item is an
in-workspace reference or an owned attachment; `internal/control/attachments.go`
creates and validates the owned attachment path, and
`desktop/attach_dropped_test.go` proves both storage branches. Codex's
`FileSystemSandboxPolicy` suite models explicit read/write/deny roots and its
`view_image` turn test reaches a local file only through a runtime helper.

Genesis aligns with both references by making upload-copy and file admission
owner-controlled. It differs from Reasonix by never putting a relative storage
path in model context, and from Codex by not turning a persisted upload record
into general post-restart filesystem authority.

## Reference Behavior Red Tests

- Restart an uploaded zip with the same ledger and material store, then use the
  original snapshot ref and original source-file ref successfully.
- Restart after a local-path intake and prove its ref is absent.
- Delete a stored upload after restart and prove the original opaque ref fails
  closed without a host or object path projection.
- Tamper a valid private index toward an outside path, remove an admitted index
  record, or add malformed data, and prove recovery becomes unavailable.
- Replace an owned archive before or after restart and prove both the source
  context descriptor and every tree/read admission fail closed.

## Phase A

- Deliverable: persistent source-owner index for uploads, immutable
  `material.intake.admitted` ledger fact, restart reconstruction gated by the
  ledger fact, and truthful capability readiness.
- Red lines: no persistence for local paths, no host/storage path in events or
  projections, no archive contents in the ledger, no generic file reader.
- Completion evidence: same opaque refs survive restart for an uploaded archive;
  only the file handles admitted at intake are restored; missing/corrupt objects
  fail closed; capability reports uploaded recovery ready.
- Still short: retention, object-store migration, user-visible attachment
  management, search, OCR, and non-zip materials.

## Phase B

- Deliverable: desktop regression coverage for selecting a file or folder,
  restarting the owned kernel, and retaining the attachment chip/session
  history without exposing internal storage details.
- Red lines: no desktop material truth, no automatic local model start, no
  arbitrary directory read.
- Completion evidence: Wails bridge/frontend tests and manual desktop
  acceptance with DeepSeek Flash.

## Phase A Completion

- Completed: owner-index persistence, admission ledger fact, ledger-gated
  restart restore, original snapshot/file refs, strict index parsing, and
  upload-only durability.
- Evidence: focused restart, path-escape, missing-index, corruption, missing
  object, and before/after-restart integrity tests; full Go test and build;
  live DeepSeek Flash upload/restart/read smoke.
- Still pending: Phase B Wails acceptance for both file and folder selection.
