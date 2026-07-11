# Requirement: User Capability Packages

- **Status:** approved for Stage 3 implementation.
- **Owner:** user-space capability package runtime owns package inspection and
  entrypoint execution; kernel owns only safe descriptor discovery projection.

## Production Target

A package placed in `~/.genesis/capabilities/<id>` is discovered through the
same manifest validation used by `genesisctl`, exposes safe availability through
`genesisd`, and runs only its declared package-local entrypoint. Its output and
state stay in the package unless a later explicit owner promotes them.

## Semantics

1. Discovery scans only direct package directories under the configured root.
   It validates manifest id, relative entrypoint, optional skill path, and
   availability without returning host paths, command arguments, or secrets.
2. `genesisctl capability list`, `doctor`, and `run` use the shared package
   inspector. `run` remains user-space process execution, not a kernel tool.
3. `genesisd` receives descriptors derived from the same inspector at startup
   and exposes only ref, name, description, inputs, outputs, and readiness in
   its existing discovery projection.
4. An invalid package is projected not-ready with a stable reason and does not
   prevent healthy sibling packages from discovery.
5. Package-local `data/` is the default boundary for outputs, cache, logs, and
   state. No output is automatically made a resource, transcript, memory, or
   audit fact.

## Non-Goals

- No marketplace, arbitrary filesystem scan, package install/update manager,
  per-domain kernel API, automatic tool grant, or output promotion.
- No workflow scheduling or long-running recovery in this stage.

## Acceptance

- When real packages exist, two distinct packages pass list/doctor/run through
  one shared path.
- `genesisd` projects both descriptors without host runner details.
- A malformed package remains isolated and cannot escape its directory.
- Until a first real package exists, the stage remains
  `no_real_package_available`; fixture tests prove only the generic mechanism.
