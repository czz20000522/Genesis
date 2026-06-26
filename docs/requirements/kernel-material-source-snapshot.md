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
  output are bounded. Long text is truncated with byte metadata. Binary entries
  can appear in the tree but `source_read` must not return garbled text.
- Source tools are `pure_read` only for snapshot-stable refs admitted by the
  source owner. The scheduler must not infer shell reads from command text.

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

## Reference Alignment

- Reasonix resolves pasted or dropped files through the TUI/controller before
  model submission; large files and binary content are bounded or summarized.
  Genesis keeps the same "application resolves external locators" idea but uses
  source refs instead of injecting raw file blocks into the prompt.
- Reasonix MCP resources are explicit resource references, not universal local
  filesystem authority.
- Codex local file operations pass through environment and sandbox surfaces; if
  local filesystem support is unavailable, requests fail closed.
- Codex image/file handlers validate local paths through a runtime filesystem
  boundary rather than letting the model invent hidden host access.

