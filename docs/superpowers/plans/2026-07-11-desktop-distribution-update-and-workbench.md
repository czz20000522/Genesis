# Desktop Distribution, Update, and Workbench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `test-driven-development` before each behavior change. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a safe Windows desktop installer whose one visible executable starts its internal kernel child, preserves user directories on uninstall, and exposes explicit release updates in the approved workbench shell.

**Architecture:** `cmd/genesisd` and the Wails child mode share an internal server bootstrap package. The desktop starts its own executable with `--genesisd-sidecar`, keeping the HTTP boundary while exposing only one application executable. A custom NSIS project owns install-location persistence and safe known-file deletion. The Go desktop bridge owns private-release HTTP, protected-token lookup, checksum validation, and installer launch; Vue only projects the resulting state through `kernelApi.ts`.

**Tech Stack:** Go standard library, existing `localconfig` DPAPI secret storage, Wails v2, NSIS, Vue 3, TypeScript, scoped plain CSS.

---

### Task 1: Run the kernel as a hidden same-executable child

**Files:**
- Create: `internal/genesisdserver/server.go`
- Modify: `cmd/genesisd/main.go`
- Modify: `desktop/main.go`
- Modify: `desktop/local_service_supervisor.go`
- Modify: `desktop/app_test.go`

- [ ] **Step 1: Write the failing same-executable child resolution test**

```go
func TestGenesisdCommandUsesDesktopChildMode(t *testing.T) {
    // Override the executable-path seam and assert the command is the desktop
    // executable with --genesisd-sidecar, never a sibling genesisd.exe.
}
```

- [ ] **Step 2: Run the focused test and confirm it fails because the sibling is not considered.**

Run: `cd desktop; go test . -run TestGenesisdCommandUsesDesktopChildMode -count=1`

- [ ] **Step 3: Move the server bootstrap behind an internal package and invoke the desktop child mode.**

```go
if executable, err := desktopExecutablePath(); err == nil {
    return executable, []string{"--genesisd-sidecar"}, filepath.Dir(executable), nil
}
```

```powershell
if slices.Contains(os.Args[1:], "--genesisd-sidecar") { genesisdserver.Run(); return }
```

- [ ] **Step 4: Run focused desktop tests and inspect the installer to confirm no kernel executable is present.**

Run: `cd desktop; go test . -count=1`

### Task 2: Persist installation location and make uninstall non-destructive

**Files:**
- Modify: `desktop/build/windows/installer/project.nsi`

- [ ] **Step 1: Add a static installer-contract check.**

```powershell
Select-String desktop/build/windows/installer/project.nsi -Pattern 'InstallDirRegKey|RMDir /r \$INSTDIR'
```

- [ ] **Step 2: Confirm the current template lacks location persistence and contains recursive removal.**

- [ ] **Step 3: Add registry-backed default selection and only delete known application files.**

```nsis
InstallDirRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${UNINST_KEY_NAME}" "InstallLocation"
WriteRegStr HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"

Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
Delete "$INSTDIR\uninstall.exe"
RMDir "$INSTDIR"
```

- [ ] **Step 4: Build NSIS and inspect the generated installer script for the exact safe statements.**

Run: `cd desktop; wails build -nsis`

### Task 3: Add explicit private-release update checking

**Files:**
- Create: `desktop/update_service.go`
- Create: `desktop/update_service_test.go`
- Modify: `desktop/app.go`
- Modify: `desktop/frontend/src/api/kernelApi.ts`
- Modify: `desktop/frontend/tests/kernelApi.test.ts`
- Modify: `desktop/frontend/src/components/InspectorDrawer.vue`
- Modify: `desktop/frontend/src/App.vue`

- [ ] **Step 1: Write failing tests for version comparison, latest-release parsing, checksum verification, and no-token refusal.**

```go
func TestCheckForUpdateRejectsMissingToken(t *testing.T) { /* want update_token_missing */ }
func TestCheckForUpdateProjectsVerifiedNewerInstaller(t *testing.T) { /* httptest latest release */ }
func TestVerifyInstallerChecksumRejectsMismatch(t *testing.T) { /* want ErrUpdateChecksum */ }
```

- [ ] **Step 2: Run the new tests and confirm the bridge does not yet expose update behavior.**

Run: `cd desktop; go test . -run 'TestCheckForUpdate|TestVerifyInstallerChecksum' -count=1`

- [ ] **Step 3: Implement the smallest explicit update bridge.**

```go
type UpdateProjection struct { CurrentVersion, LatestVersion, ReleaseURL, InstallerURL, Checksum string; Available bool; Reason string }
func (a *App) CheckForUpdate() (UpdateProjection, error) { /* GET latest release with Bearer token */ }
func (a *App) InstallVerifiedUpdate() error { /* download temp installer, SHA-256 verify, launch, Quit */ }
```

- [ ] **Step 4: Store the GitHub token with the existing protected secret mechanism and expose only `credential_present`.**

```ts
export async function checkForUpdate(): Promise<UpdateProjection> {
  return bridge.CheckForUpdate() as Promise<UpdateProjection>
}
```

- [ ] **Step 5: Run Go and frontend tests.**

Run: `cd desktop; go test . -count=1`

Run: `cd desktop/frontend; npm test`

### Task 4: Restore usable connection and local-model state in the workbench

**Files:**
- Modify: `desktop/frontend/src/App.vue`
- Modify: `desktop/frontend/src/components/KernelTopBar.vue`
- Modify: `desktop/frontend/src/components/InspectorDrawer.vue`
- Modify: `desktop/frontend/src/styles.css` and existing scoped component styles
- Modify: `desktop/frontend/tests/kernelApi.test.ts`

- [ ] **Step 1: Write a failing frontend test for `checking`, `connected`, and `starting local model` labels.**

```ts
assert.equal(readinessLabel('checking'), '检查中')
assert.equal(localModelLabel({ ownership: 'owned', readiness: 'not_ready', reason: 'sidecar_starting' }), '正在加载本地模型')
```

- [ ] **Step 2: Add a local `modelStarting` state so one click disables the action until the bridge returns.**

```ts
modelStarting.value = true
try { localModel.value = await startLocalModel() } finally { modelStarting.value = false }
```

- [ ] **Step 3: Recompose the approved workbench without new dependencies.**

```css
/* Canvas, rail, and composer use warm-white/graphite steps; teal is reserved for active and healthy states. */
button:active { transform: scale(.96); }
@media (prefers-reduced-motion: reduce) { * { transition-duration: 0ms; } }
```

- [ ] **Step 4: Run frontend tests and production build; inspect desktop at wide and 375px viewports.**

Run: `cd desktop/frontend; npm test; npm run build`

### Task 5: Release and validate the complete installer

**Files:**
- Modify: `desktop/wails.json`
- Generated: `desktop/build/bin/genesis-desktop-amd64-installer.exe`

- [ ] **Step 1: Bump the product version only after all focused checks are green.**
- [ ] **Step 2: Build with the release script and verify the installer version, SHA-256 file, and NSIS content.**
- [ ] **Step 3: Install over the previous selected directory, confirm the sidecar reaches ready, and confirm uninstall leaves a sentinel user file untouched.**
- [ ] **Step 4: Publish the installer and checksum as the private GitHub Release assets.**
