# Requirement: Material Intake And Source Snapshot

## Background

Genesis can now call a real model, but material entry is still incomplete. A
user can point at a local code package, dropped file, or future uploaded
artifact, yet the model must not receive a host path as a free filesystem
capability. The system needs a governed intake step that turns an external
locator into a source snapshot with opaque refs, bounded descriptors, and typed
read tools.

## Production Goal

Genesis accepts user-provided material through an application/control surface,
creates a source snapshot owned by the resource/source layer, and lets the model
inspect that snapshot only through typed tools. The model sees snapshot and file
refs plus available operations; it does not see host paths, storage refs,
object keys, upload paths, or raw external payload ids.

## Roles

- User/operator/application: submits a material locator or upload through a
  control surface.
- Material intake owner: validates locator shape, purpose, size, and source
  type, then asks the source snapshot owner to register an admitted snapshot.
- Source snapshot owner: owns snapshot refs, file refs, descriptors, listing,
  bounded file reads, binary detection, zip-entry validation, and source read
  admission.
- Kernel Tool Runtime: exposes typed `source_tree` and `source_read` tools and
  records terminal-equivalent tool results.
- LLM: chooses visible source refs and typed operations; it cannot invent host
  paths, internal refs, storage refs, or upload paths.

## Core Semantics

- A local path is an input locator. It is not a `resource_ref`,
  `source_snapshot_ref`, `source_file_ref`, or model-visible authority.
- Local path intake is a control/operator surface, not a model-visible tool.
- `source_snapshot_ref` and `source_file_ref` are opaque system-generated refs.
  They must not encode host paths, upload storage paths, object keys, profile
  names, credentials, or raw payload ids.
- A source descriptor projects current available operations, such as
  `source_tree` and `source_read`. This projection is not authority truth; tool
  execution must re-run admission.
- Local path snapshots are live reads of the original file for this phase. If
  the file disappears, later source operations fail with structured
  `resource_unavailable` evidence rather than pretending the snapshot body was
  durable.
- Uploaded material is different from a local locator: upload bodies are
  transient, so the uploaded bytes must be stored in an object/file store before
  snapshot parsing. Only metadata and refs belong in kernel facts.
- Zip source snapshots must validate entry names before exposure. Entries with
  `..`, absolute paths, Windows drive paths, backslash escaping, empty names, or
  duplicate normalized paths fail closed or are refused with bounded
  diagnostics.
- File count, per-file size, total uncompressed size, tree output, and read
  output are bounded by `SourceSnapshotPolicy` and projected through runtime
  limits. Intake defaults must be sized for ordinary project-scale code
  packages, while per-read provider output remains small and explicitly
  truncated with byte metadata.
- Binary entries can appear in the tree but `source_read` must not return
  garbled text.
- Source tools are `pure_read` only for source refs admitted by the source owner
  and only after the owner can serve tree/read operations without mutating
  shared resolver state during tool execution. The scheduler must not infer
  shell reads from command text.
- Uploaded source snapshots are durable owner state: after the upload body is
  committed to the material store and the admission fact is appended, a restart
  with the same ledger and material store restores the same snapshot and file
  refs. The admission descriptor records the archive SHA-256; the source owner
  verifies it before every source projection or operation. The private owner
  index may contain an object name and storage metadata; those details must never
  enter the ledger payload, provider context, tool result, transcript, or public
  descriptor.
- A local-path snapshot remains a live locator in this phase. It is deliberately
  not reconstructed after restart, even when its path still exists: persistence
  is a property of a copied upload body, not a new grant of host filesystem
  authority.
- A missing or corrupted durable archive does not prevent kernel startup. The
  recovered opaque ref remains scoped to its session, and each attempted source
  operation fails closed with structured `resource_unavailable` or
  `invalid_source_archive` evidence.

## Non-Goals

- No arbitrary host filesystem read tool.
- No universal `ref_read` tool.
- No `skill.read`, `read_skill`, connector attachment reader, OCR reader,
  document reader, or Feishu/mail/WeChat-specific kernel feature.
- No production object store, vector index, search index, binary rendering, OCR,
  dependency analysis, or code intelligence engine in this slice.
- No default provider context splicing of full archive contents.
- No WebUI, desktop UI, or live LLM automated acceptance requirement.

## Phased Delivery

- Phase A: document the material/source boundary and lock behavior with tests.
- Phase B: implement local-path intake for zip source snapshots with live-read
  semantics and bounded diagnostics.
- Phase C: expose `source_tree` and `source_read` as typed pure-read tools and
  project admitted snapshots into provider context without host paths.
- Phase D: implement minimal upload intake with traversal-safe filenames and
  object/file-store backed bodies.
- Phase E: use a real model only for manual smoke after fake-provider behavior
  tests prove the tool loop.
- Phase F: persist uploaded-source owner records, append a material-admission
  ledger fact, and restore only records backed by both the private owner index
  and that ledger fact.

## Current Implementation

The current slice implements Phase A through Phase F for zip source packages:

- `POST /materials/intake` admits an absolute local zip path as a source
  snapshot for an optional session. The target zip must remain available while
  the current process uses the snapshot.
- `POST /materials/upload` stores multipart upload bytes in the kernel material
  file store using a generated path, treats the uploaded filename as display
  metadata only, reuses zip snapshot parsing, records an opaque
  `material.intake.admitted` fact, and persists the private resolver record.
- `source_tree` and `source_read` are typed model-visible tools backed by
  source owner admission. They accept only source refs, not host paths, and run
  against handles generated at intake.
- Source intake/read limits are configurable through `SourceSnapshotPolicy` and
  visible through runtime limit inspection.
- Provider context may include a bounded `source_snapshot_context` notice with
  refs and operation names. It omits host paths, storage refs, upload paths, and
  archive bodies.

Remaining future work is retention/quarantine, richer source selection, source
search/span tools, code intelligence indexing, and live LLM/user-interface
smoke. Those gaps do not change the current source-ref contract.

## Acceptance Criteria

- Missing local path intake is refused with a structured resource-unavailable
  reason.
- Passing a host path directly to `resource_read`, `source_tree`, or
  `source_read` is rejected and never falls back to host filesystem reads.
- Zip slip entries cannot escape the archive namespace and cannot become source
  files.
- Oversized archives, too many files, oversized files, empty zips, corrupted
  zips, nested directories, duplicate paths, and binary entries are covered by
  behavior tests.
- Source descriptors list available operations, but source tools re-run
  admission at call time.
- Model-visible provider context and tool manifests contain source refs and
  operation names only; they do not contain host paths, upload storage paths,
  object keys, or raw payload paths.
- `source_tree` and `source_read` are typed tools with independent result
  schemas. They are not replaced by a broad `ref_read` API.
- Upload filenames are display labels only and never affect storage path.
- Uploaded zip snapshots reuse source snapshot parsing.
- Fake-provider tests demonstrate a turn can see a source snapshot, call
  `source_tree`, call `source_read`, and produce a final answer based on file
  content.
- Runtime capabilities expose effective source snapshot intake/read limits.
- A restarted kernel with the same ledger and material store can use an uploaded
  source snapshot's original opaque snapshot and file refs without revealing a
  host or storage path.
- A restarted kernel does not recover local-path snapshots and does not turn a
  missing uploaded archive into an invented source result.
- The ledger records the admitted snapshot descriptor, while the source owner
  keeps the private object location only in its durable index.

## Reference Alignment

- Reasonix `desktop.App.AttachDropped` distinguishes in-workspace references
  from out-of-workspace files, copying the latter into `.reasonix/attachments`;
  `internal/control/attachments.go` generates the stored name, validates its
  containment, and its tests prove both branches. Genesis aligns on copying an
  application upload into an owned store, but intentionally projects opaque
  source refs rather than Reasonix's relative paths.
- Codex `FileSystemSandboxPolicy` tests explicitly model read, write, and deny
  roots; its `view_image` turn test obtains a file only through the runtime
  helper. Genesis aligns on a checked owner boundary and rejects direct model
  paths, but differs by restoring only uploaded objects rather than granting a
  post-restart filesystem read capability.
