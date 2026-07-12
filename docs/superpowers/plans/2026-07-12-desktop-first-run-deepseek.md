# Desktop First-Run DeepSeek Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a profile-less Genesis Home become usable from the desktop with one DeepSeek Flash API key, verification, and explicit coordinator activation.

**Architecture:** Move OpenAI-compatible profile persistence into `localconfig`, which already owns models.json and protected credential records. Kernel keeps the generic wrapper and upstream verification; the desktop calls one fixed DeepSeek Flash setup method, reloads safe profiles, verifies through the existing kernel route, and leaves restart to the existing apply action.

**Tech Stack:** Go, Wails, Vue 3, TypeScript, Element Plus, existing protected credential store.

**Execution status:** automated implementation and configured DeepSeek live
acceptance are complete. The installed empty-Home click path remains
`manual_test_pending`; it is the only evidence not reproducible by the current
terminal environment.

---

## File Map

- `localconfig/provider_setup.go`: shared configuration setup request/result and canonical DeepSeek Flash preset metadata.
- `localconfig/provider_control_test.go`: setup persistence, secret redaction, and profile assertions.
- `internal/kernel/provider_setup.go`: delegate configuration persistence to `localconfig`; retain provider-resolution verification.
- `internal/kernel/provider_setup_test.go`: preserve generic setup and verification behavior through the delegated writer.
- `cmd/genesisctl/provider_presets.go`: obtain DeepSeek Flash preset metadata from `localconfig` rather than duplicating it.
- `desktop/provider_control.go`: Wails bridge for the fixed first-run setup.
- `desktop/provider_control_test.go`: prove the bridge uses the shared writer and returns no secret.
- `desktop/frontend/src/api/kernelApi.ts`: typed Wails bridge function.
- `desktop/frontend/src/App.vue`: setup, reload, selected-profile, and verification sequence.
- `desktop/frontend/src/components/ProviderPanel.vue`: no-profile DeepSeek Flash form using existing Element Plus controls.
- `desktop/frontend/tests/kernelApi.test.ts`: bridge and empty-state regression checks.
- `features/applications/desktop-provider-control.feature`: observable first-run scenario.

### Task 1: Lock the first-run observable contract

**Files:**
- Create: `features/applications/desktop-provider-control.feature`
- Test: `desktop/provider_control_test.go`

- [ ] **Step 1: Add the failing observable scenario**

```gherkin
Scenario: A user configures DeepSeek Flash from an empty Genesis Home
  Given Genesis Home has no model profiles
  When the user submits a DeepSeek Flash API key from the desktop
  Then Genesis Home contains the DeepSeek Flash profile without the API key
  And the desktop can verify the profile before applying it to coordinator
```

- [ ] **Step 2: Add a failing desktop backend test**

```go
func TestSetupDeepSeekFlashCreatesSafeProfileWithoutReturningSecret(t *testing.T) {
    root := desktopTestTempDir(t)
    app := &App{providerControl: desktopProviderControlConfig{
        ConfigRoot: root, CredentialStoreRoot: filepath.Join(root, "credentials"),
        secretProtector: func(data []byte) ([]byte, error) { return append([]byte("sealed:"), data...), nil },
    }}
    result, err := app.SetupDeepSeekFlash("secret-key")
    if err != nil || result.ProfileID != "deepseek-flash" || !result.CredentialPresent {
        t.Fatalf("setup result = %+v, err = %v", result, err)
    }
}
```

- [ ] **Step 3: Run the focused test and confirm it fails because the bridge is absent**

Run: `go test ./desktop -run TestSetupDeepSeekFlashCreatesSafeProfileWithoutReturningSecret -count=1`

Expected: FAIL with `app.SetupDeepSeekFlash undefined`.

### Task 2: Create the shared configuration owner

**Files:**
- Modify: `localconfig/models.go`
- Modify: `localconfig/provider_control_test.go`

- [ ] **Step 1: Add the failing shared-owner tests**

```go
func TestSetupDeepSeekFlashWritesCanonicalProfileAndProtectedSecret(t *testing.T) {
    root := t.TempDir()
    result, err := SetupDeepSeekFlash(DeepSeekFlashSetupRequest{
        ConfigRoot: root, CredentialStoreRoot: filepath.Join(root, "credentials"), APIKey: "secret-key",
        Protector: func(data []byte) ([]byte, error) { return append([]byte("sealed:"), data...), nil },
    })
    if err != nil || result.ProfileID != "deepseek-flash" { t.Fatalf("result = %+v, err = %v", result, err) }
    payload, _ := os.ReadFile(filepath.Join(root, "models.json"))
    if strings.Contains(string(payload), "secret-key") { t.Fatal("models.json leaked API key") }
}
```

- [ ] **Step 2: Run the focused package tests and confirm the new owner is absent**

Run: `go test ./localconfig -run TestSetupDeepSeekFlashWritesCanonicalProfileAndProtectedSecret -count=1`

Expected: FAIL with `SetupDeepSeekFlash undefined`.

- [ ] **Step 3: Implement the smallest shared owner**

```go
type DeepSeekFlashSetupRequest struct {
    ConfigRoot, CredentialStoreRoot, APIKey string
    Protector func([]byte) ([]byte, error)
}

type DeepSeekFlashSetupResult struct {
    ProfileID, GatewayRoute string
    CredentialPresent bool
}

func SetupDeepSeekFlash(req DeepSeekFlashSetupRequest) (DeepSeekFlashSetupResult, error) {
    // validate the non-empty key, write secret://models/deepseek/local,
    // and upsert only deepseek/deepseek-flash metadata without binding coordinator.
}
```

Use the existing `ReadModels`, `WriteModels`, `WriteCredentialSecret`,
`ResolveConfigRoot`, and `DefaultModelRole` helpers. The fixed route is
`https://api.deepseek.com`, model is `deepseek-v4-flash`, timeout is 60 seconds,
context window is 1,000,000, and adapter metadata is `deepseek` /
`deepseek-v4-flash`. Return only profile identity and credential presence.

- [ ] **Step 4: Run the focused shared-owner test**

Run: `go test ./localconfig -run TestSetupDeepSeekFlashWritesCanonicalProfileAndProtectedSecret -count=1`

Expected: PASS.

### Task 3: Keep CLI and kernel setup on the same write path

**Files:**
- Modify: `internal/kernel/provider_setup.go`
- Modify: `internal/kernel/provider_setup_test.go`
- Modify: `cmd/genesisctl/provider_presets.go`
- Modify: `cmd/genesisctl/main_test.go`

- [ ] **Step 1: Add regression tests before changing the adapter**

```go
func TestSetupOpenAICompatibleProviderKeepsVerificationAfterLocalconfigWrite(t *testing.T) {
    // Existing test server returns a completion; assert setup still reports Verified.
}
```

```go
func TestProviderUseDeepSeekFlashUsesCanonicalSharedPreset(t *testing.T) {
    // Run provider use deepseek/deepseek-v4-flash with a test key and inspect
    // models.json for the localconfig DeepSeek Flash profile metadata.
}
```

- [ ] **Step 2: Run the focused tests and confirm they fail for the intended delegation**

Run: `go test ./internal/kernel ./cmd/genesisctl -run 'TestSetupOpenAICompatibleProviderKeepsVerificationAfterLocalconfigWrite|TestProviderUseDeepSeekFlashUsesCanonicalSharedPreset' -count=1`

Expected: FAIL until setup mutation delegates to `localconfig` and the CLI no
longer owns DeepSeek Flash metadata.

- [ ] **Step 3: Delegate persistence while retaining verification**

Keep `SetupOpenAICompatibleProvider` as the kernel API used by the CLI. Replace
its direct models.json and credential writes with a typed `localconfig`
OpenAI-compatible setup operation. After that operation returns, retain the
existing kernel-only provider-resolution verification. Make the DeepSeek Flash
CLI preset read the fixed metadata from `localconfig`; leave unrelated DeepSeek
Pro and SCNet presets unchanged.

- [ ] **Step 4: Run focused kernel and CLI tests**

Run: `go test ./internal/kernel ./cmd/genesisctl -count=1`

Expected: PASS.

### Task 4: Expose the fixed setup through the desktop backend

**Files:**
- Modify: `desktop/provider_control.go`
- Modify: `desktop/provider_control_test.go`
- Modify: `desktop/frontend/src/api/kernelApi.ts`

- [ ] **Step 1: Implement the backend bridge after Task 1's test is red**

```go
type FirstRunDeepSeekProjection struct {
    ProfileID string `json:"profile_id"`
    CredentialPresent bool `json:"credential_present"`
}

func (a *App) SetupDeepSeekFlash(apiKey string) (FirstRunDeepSeekProjection, error) {
    result, err := localconfig.SetupDeepSeekFlash(localconfig.DeepSeekFlashSetupRequest{
        ConfigRoot: a.providerControl.ConfigRoot, CredentialStoreRoot: a.providerControl.CredentialStoreRoot,
        APIKey: apiKey, Protector: a.providerControl.secretProtector,
    })
    if err != nil { return FirstRunDeepSeekProjection{}, err }
    return FirstRunDeepSeekProjection{ProfileID: result.ProfileID, CredentialPresent: result.CredentialPresent}, nil
}
```

Do not verify, bind a different profile, or restart a sidecar in this method.

- [ ] **Step 2: Add the typed frontend bridge**

```ts
export async function setupDeepSeekFlash(apiKey: string): Promise<FirstRunDeepSeek> {
  const bridge = wailsAppBridge()
  if (!bridge?.SetupDeepSeekFlash) throw new Error('DeepSeek 首次配置仅在 Genesis 桌面客户端中可用')
  return bridge.SetupDeepSeekFlash(String(apiKey || '').trim()) as Promise<FirstRunDeepSeek>
}
```

- [ ] **Step 3: Run backend and bridge tests**

Run: `go test ./desktop -run TestSetupDeepSeekFlashCreatesSafeProfileWithoutReturningSecret -count=1`

Run: `node --experimental-strip-types ./tests/kernelApi.test.ts`

Expected: PASS.

### Task 5: Render the compact first-run form

**Files:**
- Modify: `desktop/frontend/src/App.vue`
- Modify: `desktop/frontend/src/components/ProviderPanel.vue`
- Modify: `desktop/frontend/tests/kernelApi.test.ts`

- [ ] **Step 1: Add the failing frontend assertions**

```ts
assert.equal(providerPanelSource.includes('配置 DeepSeek Flash'), true)
assert.equal(providerPanelSource.includes('setupDeepSeekFlash'), true)
assert.equal(appSource.includes('setupDeepSeekFlash'), true)
assert.equal(providerPanelSource.includes('type="password"'), true)
```

- [ ] **Step 2: Implement one empty-state form using existing controls**

Use the existing `providerCredential` ref and Element Plus `el-input` /
`el-button`. The input is rendered only when `profiles.length === 0`; its action
emits `setupDeepSeekFlash`. In `App.vue`, clear the key in `finally`, reload
profiles, select `deepseek-flash`, then invoke the existing verification flow.
Do not auto-apply or add a second page, modal, route, endpoint input, or new
dependency.

- [ ] **Step 3: Run frontend tests and build**

Run: `npm test && npm run build`

Expected: PASS.

### Task 6: Perform live acceptance and release checks

**Files:**
- Modify: `docs/operations/application-issues.md`
- Modify: `docs/operations/application-retirement-log.md` only if the desktop
  click path has been manually exercised.

- [ ] **Step 1: Run complete automated verification**

Run: `go test ./... -count=1`

Run: `go build ./...`

Run: `go test ./... -count=1` from `desktop`

Run: `npm test && npm run build` from `desktop/frontend`

Run: `git diff --check`

Expected: PASS.

- [ ] **Step 2: Run the real configured DeepSeek acceptance**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/first_run_live_llm_acceptance.ps1 -UseConfiguredProfile -ProfileId deepseek-flash -Addr 127.0.0.1:8771 -WorkRoot .test-tmp/deepseek-flash-first-run`

Expected: JSON with `ok: true`, `provider_model: deepseek-v4-flash`, a final
answer, and restart replay evidence. Remove the temporary work root after the
command completes.

- [ ] **Step 3: Record only the remaining manual evidence**

Keep the issue `manual_test_pending` until the user clicks `保存并验证` then
`应用并重启服务` in the installed desktop and completes one settled turn. Do
not claim that GUI interaction was tested from CLI evidence.
