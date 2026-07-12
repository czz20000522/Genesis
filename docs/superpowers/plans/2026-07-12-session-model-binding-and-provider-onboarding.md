# Session Model Binding and Provider Onboarding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a blank Genesis Home import a curated provider, then let every
Project, Task, and Chat session persist its own selected coordinator model.

**Architecture:** `localconfig` owns curated template defaults, protected
credentials, routes, and materialized profiles. The kernel owns the durable
`session.model_bound` ledger fact and chooses the bound provider once for each
turn. Desktop invokes only those owners and shows the session chooser in the
composer. No ordinary chat action writes a global `coordinator` binding or
restarts `genesisd`.

**Tech Stack:** Go kernel and SQLite ledger, Go Wails bridge, Vue 3,
TypeScript, Element Plus, existing OpenAI-compatible discovery client.

---

## File Map

- `internal/kernel/session_model.go`: session binding owner and active-turn
  gate.
- `internal/kernel/http_session_model.go`: authenticated model-bind route.
- `internal/kernel/event_types.go`, `inspection_types.go`,
  `session_projection.go`: durable event and safe session projection.
- `internal/kernel/kernel.go`, `config_types.go`: per-turn session provider
  resolver; provider stays fixed inside one turn.
- `cmd/genesisd/main.go`: resolve a selected profile into a provider.
- `internal/kernel/provider_model_refresh.go`: route-level model discovery
  that does not require a global role/profile binding.
- `localconfig/provider_templates.go`: five curated templates and local config
  route/profile writes.
- `desktop/provider_control.go`: import bridge and safe provider projections.
- `desktop/app.go`: session-model bridge methods.
- `desktop/frontend/src/api/kernelApi.ts`: typed Wails and kernel calls.
- `desktop/frontend/src/components/ConversationPane.vue`: composer selector.
- `desktop/frontend/src/components/ProviderPanel.vue`: template-import and
  provider-management view.
- `desktop/frontend/src/App.vue`: session projection, bind, import, and no
  global coordinator apply flow.
- `features/kernel/session_model_binding.feature` and
  `features/applications/desktop-provider-onboarding.feature`: observable
  scenarios.

### Task 1: Persist session-level model binding

**Files:**
- Create: `internal/kernel/session_model.go`
- Create: `internal/kernel/http_session_model.go`
- Modify: `internal/kernel/event_types.go`
- Modify: `internal/kernel/inspection_types.go`
- Modify: `internal/kernel/session_projection.go`
- Modify: `internal/kernel/http.go`
- Test: `internal/kernel/session_model_test.go`
- Test: `internal/kernel/http_session_model_test.go`
- Create: `features/kernel/session_model_binding.feature`

- [ ] **Step 1: Add the observable feature and failing owner tests.**

```gherkin
Feature: Session model binding

  Scenario: Sessions retain independent selected models
    Given session A is bound to profile "deepseek-flash"
    And session B is bound to profile "local-qwen"
    When Genesis restarts
    Then session A projects "deepseek-flash"
    And session B projects "local-qwen"
```

```go
func TestBindSessionModelPersistsLatestBindingWithoutChangingAnotherSession(t *testing.T) {
    k := newTestKernel(t, testLedgerPath(t))
    if err := k.BindSessionModel("session-a", SessionModelBindingRequest{ProfileID: "deepseek-flash"}); err != nil { t.Fatal(err) }
    if err := k.BindSessionModel("session-b", SessionModelBindingRequest{ProfileID: "local-qwen"}); err != nil { t.Fatal(err) }
    projection, err := k.Session("session-a")
    if err != nil || projection.ModelProfileID != "deepseek-flash" { t.Fatalf("projection = %+v, err = %v", projection, err) }
}
```

- [ ] **Step 2: Run the focused test and confirm it is red.**

Run: `go test ./internal/kernel -run TestBindSessionModelPersistsLatestBindingWithoutChangingAnotherSession -count=1`

Expected: compile failure until the session-model owner exists.

- [ ] **Step 3: Add the smallest durable owner.**

```go
type SessionModelBindingRequest struct { ProfileID string }
type SessionModelBinding struct { ProfileID string `json:"profile_id"` }

func (k *Kernel) BindSessionModel(sessionID string, request SessionModelBindingRequest) error {
    sessionID, profileID := strings.TrimSpace(sessionID), strings.TrimSpace(request.ProfileID)
    if sessionID == "" || profileID == "" || k.sessionProviderResolver == nil { return ErrSessionModelInvalid }
    if k.sessionHasActiveTurn(sessionID) { return ErrSessionModelChangeBlockedActiveTurn }
    if _, err := k.sessionProviderResolver(profileID); err != nil { return err }
    return k.appendEvent(StoredEvent{EventID: newID("evt", k.clock()), SessionID: sessionID, Type: "session.model_bound", CreatedAt: k.clock(), Data: EventData{SessionModel: &SessionModelBinding{ProfileID: profileID}}})
}
```

Add `SessionModel *SessionModelBinding` to `EventData`,
`ModelProfileID string` to `SessionProjection`, and reduce the latest binding
in `session_projection.go`. The route accepts `{ "profile_id": "..." }` at
`POST /sessions/{session_id}/model`; unselected/invalid is `400`, and an active
turn is `409 session_model_change_blocked_active_turn`.

- [ ] **Step 4: Prove projection and HTTP error semantics.**

Run: `go test ./internal/kernel -run 'Test(BindSessionModel|HTTPSessionModel)' -count=1`

Expected: PASS.

### Task 2: Resolve one bound provider for each turn

**Files:**
- Modify: `internal/kernel/config_types.go`
- Modify: `internal/kernel/kernel.go`
- Modify: `internal/kernel/turn_interrupt.go`
- Test: `internal/kernel/session_model_turn_test.go`

- [ ] **Step 1: Add a failing cross-session provider test.**

```go
func TestBoundSessionModelUsesItsProviderForEveryRound(t *testing.T) {
    providers := map[string]*FakeProvider{"deepseek-flash": {Response: ModelResponse{Text: "cloud"}}, "local-qwen": {Response: ModelResponse{Text: "local"}}}
    k := newSessionModelKernel(t, providers)
    mustBindSessionModel(t, k, "a", "deepseek-flash")
    mustBindSessionModel(t, k, "b", "local-qwen")
    if got, _ := k.SubmitTurn(context.Background(), TurnRequest{SessionID: "a", InputItems: []InputItem{{Type: "text", Text: "a"}}}); got.Final.Text != "cloud" { t.Fatalf("a = %+v", got) }
    if got, _ := k.SubmitTurn(context.Background(), TurnRequest{SessionID: "b", InputItems: []InputItem{{Type: "text", Text: "b"}}}); got.Final.Text != "local" { t.Fatalf("b = %+v", got) }
}
```

- [ ] **Step 2: Route provider use through the session resolver.**

Add `SessionProviderResolver func(profileID string) (Provider, error)` to
`Config` and `Kernel`. Before `tryBeginActiveTurn`, resolve the latest session
binding and hold that returned provider in the turn execution path:

```go
func (k *Kernel) providerForSession(sessionID string) (Provider, error) {
    profileID, ok, err := k.sessionModelProfileID(sessionID)
    if err != nil || !ok { return nil, ErrSessionModelUnselected }
    return k.sessionProviderResolver(profileID)
}
```

Change `runTurnModelLoop`, `completeProviderStep`, provider-context prefix
fingerprinting, hydrated-context conversion, session debug capture, and
compaction calls to receive that provider explicitly. Keep `k.provider` only
as the fallback for non-session legacy/test callers; never use it after a
desktop session has a binding.

- [ ] **Step 3: Guard model changes against a running turn.**

Expose a small locked helper:

```go
func (k *Kernel) sessionHasActiveTurn(sessionID string) bool {
    k.activeTurnMu.Lock(); defer k.activeTurnMu.Unlock()
    return k.activeTurns[strings.TrimSpace(sessionID)] != nil
}
```

Run: `go test ./internal/kernel -run 'Test(BoundSessionModel|SessionModelChange)' -count=1`

Expected: PASS, including no event appended for a blocked change.

### Task 3: Build selected providers in genesisd without a global coordinator

**Files:**
- Modify: `cmd/genesisd/main.go`
- Test: `cmd/genesisd/main_test.go`

- [ ] **Step 1: Add a failing daemon configuration test.**

```go
func TestGenesisConfigBuildsProviderForExplicitSessionProfile(t *testing.T) {
    provider, err := buildProvider(providerBuildRequest{name: "genesis-config", configRoot: testConfigRoot(t), modelProfileID: "deepseek-flash"})
    if err != nil { t.Fatal(err) }
    if provider.Ready().Readiness != kernel.ReadinessReady { t.Fatalf("ready = %+v", provider.Ready()) }
}
```

- [ ] **Step 2: Wire the resolver once at kernel construction.**

```go
SessionProviderResolver: func(profileID string) (kernel.Provider, error) {
    return buildProvider(providerBuildRequest{
        name: "genesis-config", configRoot: *configRoot,
        credentialStoreRoot: *credentialStoreRoot, modelProfileID: profileID,
    })
},
```

Do not pass `modelRole`; a selected session profile is complete provider input.

- [ ] **Step 3: Verify daemon and kernel integration.**

Run: `go test ./cmd/genesisd ./internal/kernel -count=1`

Expected: PASS.

### Task 4: Import curated provider templates and materialize profiles

**Files:**
- Create: `localconfig/provider_templates.go`
- Modify: `localconfig/models.go`
- Modify: `internal/kernel/provider_model_refresh.go`
- Modify: `internal/kernel/http.go`
- Create: `internal/kernel/http_provider_discovery.go`
- Modify: `desktop/provider_control.go`
- Test: `localconfig/provider_templates_test.go`
- Test: `internal/kernel/http_provider_discovery_test.go`
- Test: `desktop/provider_control_test.go`
- Create: `features/applications/desktop-provider-onboarding.feature`

- [ ] **Step 1: Add fixed-template and no-secret tests.**

```go
func TestImportProviderTemplateWritesRouteAndMaterializesDiscoveredProfiles(t *testing.T) {
    result, err := ImportProviderTemplate(ProviderTemplateImportRequest{ConfigRoot: t.TempDir(), TemplateID: "deepseek", APIKey: "secret", Models: []string{"deepseek-v4-flash", "deepseek-v4-pro"}, Protector: seal})
    if err != nil { t.Fatal(err) }
    if result.RouteID != "deepseek" || len(result.ProfileIDs) != 2 { t.Fatalf("result = %+v", result) }
}
```

- [ ] **Step 2: Define the five templates in localconfig.**

```go
type ProviderTemplate struct { ID, Name, Protocol, BaseURL, CredentialRef, AdapterID string; RequiresCredential, SupportsDiscovery, Advanced bool }
func ProviderTemplates() []ProviderTemplate { return []ProviderTemplate{deepSeekTemplate(), openAITemplate(), openCodeGoTemplate(), localLlamaCPPTemplate(), advancedOpenAICompatibleTemplate()} }
```

The import request validates the fixed template id, writes its route and a
protected credential, and materializes one cloud/local `GatewayProfile` for
each normalized discovered model. The advanced template requires explicit
base URL and model id; ordinary templates do not expose endpoint fields.

- [ ] **Step 3: Expose route-level discovery without a role binding.**

Add `POST /providers/{route_id}/models/discover`. It resolves the configured
route and its credential, calls the existing bounded OpenAI-compatible `/models`
client, and returns only model ids/reason. It does not write config. Desktop
calls localconfig import once discovery succeeds; if discovery fails, it keeps
the saved route and reports a retryable reason.

- [ ] **Step 4: Implement the desktop import bridge in two durable steps.**

```go
func (a *App) ImportProviderTemplate(templateID, apiKey, baseURL, modelID string) (ProviderImportProjection, error) {
    route, err := localconfig.ImportProviderTemplateRoute(localconfig.ProviderTemplateRouteImportRequest{ConfigRoot: a.providerControl.ConfigRoot, CredentialStoreRoot: a.providerControl.CredentialStoreRoot, TemplateID: templateID, APIKey: apiKey, BaseURL: baseURL, ExplicitModelID: modelID, Protector: a.providerControl.secretProtector})
    if err != nil { return ProviderImportProjection{}, err }
    models, reason := a.discoverProviderModels(route.RouteID)
    if reason != "" { return ProviderImportProjection{RouteID: route.RouteID, DiscoveryReason: reason}, nil }
    return projectProviderImport(localconfig.MaterializeProviderTemplateModels(localconfig.ProviderTemplateModelsRequest{ConfigRoot: a.providerControl.ConfigRoot, RouteID: route.RouteID, Models: models}))
}
```

The implementation must avoid putting `apiKey` in any projection, Wails event,
or frontend storage. The route is intentionally retained when its discovery
fails, so the UI can show a retryable reason. Local llama.cpp import
materializes its configured local profile without starting the service.

- [ ] **Step 5: Run focused import/discovery verification.**

Run: `go test ./localconfig ./internal/kernel ./desktop -run 'Test(ImportProviderTemplate|HTTPProviderDiscovery|DesktopProviderImport)' -count=1`

Expected: PASS.

### Task 5: Put the model selector in the composer

**Files:**
- Modify: `desktop/app.go`
- Modify: `desktop/frontend/src/api/kernelApi.ts`
- Modify: `desktop/frontend/src/App.vue`
- Modify: `desktop/frontend/src/components/ConversationPane.vue`
- Modify: `desktop/frontend/src/components/ProviderPanel.vue`
- Modify: `desktop/frontend/src/styles.css`
- Modify: `desktop/frontend/tests/kernelApi.test.ts`

- [ ] **Step 1: Add bridge and frontend regression assertions.**

```ts
assert.equal(conversationSource.includes('sessionModelProfileId'), true)
assert.equal(conversationSource.includes('选择模型'), true)
assert.equal(appSource.includes('bindSessionModel'), true)
```

- [ ] **Step 2: Add a typed session bind call.**

```ts
export async function bindSessionModel(config: KernelConfig, sessionId: string, profileId: string) {
  return requestKernel<SessionProjection>(config, `/sessions/${encodeURIComponent(requiredSessionId(sessionId))}/model`, {
    method: 'POST', body: JSON.stringify({ profile_id: String(profileId || '').trim() }),
  })
}
```

The Wails bridge calls the same protected endpoint. `App.vue` reloads the
current session projection after a selection and never calls
`ApplyProviderRole` for an ordinary conversation.

- [ ] **Step 3: Render only session-local state.**

`ConversationPane` receives `modelProfiles`, `sessionModelProfileId`, and a
`selectModel` event. It renders an Element Plus select beside composer actions,
with grouped safe profiles and `选择模型` placeholder. Send is disabled when a
session exists but its profile id is empty. A selector is disabled while a turn
is active; it does not create a new session or alter the rail.

- [ ] **Step 4: Replace the blank-home fixed DeepSeek form with template cards.**

ProviderPanel renders template cards only when there are no profiles; card
selection reveals one password field for ordinary templates and a collapsed
advanced form only for the generic route. It reloads safe projections after a
successful import and returns focus to the composer model selector.

- [ ] **Step 5: Run frontend verification.**

Run: `npm test && npm run build`

Expected: PASS.

### Task 6: End-to-end proof and release

**Files:**
- Modify: `docs/operations/application-issues.md`
- Modify: `docs/implementation-plans/desktop-provider-control.md`

- [ ] **Step 1: Run full automated verification.**

Run: `go test ./... -count=1`

Run: `go build ./...`

Run: `go test ./... -count=1` from `desktop`

Run: `npm test && npm run build` from `desktop/frontend`

Run: `git diff --check`

- [ ] **Step 2: Run real DeepSeek proof with two sessions.**

Create two durable sessions; bind one to `deepseek-flash`, bind the other to a
different configured profile. Submit a settled DeepSeek Flash turn, restart
`genesisd`, then verify its bound profile and timeline survive. A second local
model proof is `manual_test_pending` if the local llama service is unavailable.

- [ ] **Step 3: Build and install the next desktop release.**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build_desktop_release.ps1`

Install only after the installer hash is generated; verify the installed
`D:\software\Genesis\genesis-desktop.exe` hash matches the built executable.

## Plan Self-Review

- Session facts, per-turn resolver, template import, composer selection, and
  real restart proof each map to a requirement acceptance criterion.
- The plan has no provider marketplace, global chat coordinator, model fallback,
  automatic local launch, or desktop-owned session truth.
- Every provider configuration write is in `localconfig`; every session bind is
  in the kernel ledger; every provider wire call remains in the Model Gateway.
