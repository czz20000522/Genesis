package kernel

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadinessDTOsDoNotExposeGenericStatusTags(t *testing.T) {
	for _, value := range []any{
		ReadyResponse{},
		CapabilitiesResponse{},
		ProviderStatus{},
		ReadyCheck{},
	} {
		assertNoJSONStatusTag(t, reflect.TypeOf(value))
	}
}

func TestReadinessSurfacesUseReadinessAxis(t *testing.T) {
	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:   NewBlockedProvider("blocked-provider", "provider_api_key_missing"),
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	ready := k.Ready()
	if ready.Readiness != ReadinessNotReady || ready.ReadinessReason == "" {
		t.Fatalf("ready = %+v, want not_ready with reason", ready)
	}
	if ready.Provider.Readiness != ReadinessNotReady || ready.Provider.ReadinessReason != "provider_api_key_missing" {
		t.Fatalf("provider readiness = %+v, want provider not_ready reason", ready.Provider)
	}
	if ready.RuntimeAuth.Readiness != ReadinessNotReady || ready.RuntimeAuth.ReadinessReason != "runtime_token_missing" {
		t.Fatalf("runtime auth readiness = %+v, want runtime token missing", ready.RuntimeAuth)
	}
	if ready.Ledger.Readiness != ReadinessReady {
		t.Fatalf("ledger readiness = %+v, want ready", ready.Ledger)
	}

	encodedReady, err := json.Marshal(ready)
	if err != nil {
		t.Fatalf("marshal ready: %v", err)
	}
	var readyMap map[string]any
	if err := json.Unmarshal(encodedReady, &readyMap); err != nil {
		t.Fatalf("unmarshal ready: %v", err)
	}
	if _, ok := readyMap["status"]; ok {
		t.Fatalf("/ready JSON = %s, must not expose top-level status for readiness", string(encodedReady))
	}
	assertNoNestedStatus(t, readyMap, "provider")
	assertNoNestedStatus(t, readyMap, "runtime_auth")
	assertNoNestedStatus(t, readyMap, "ledger")

	capabilities := k.Capabilities()
	if capabilities.Readiness != ReadinessNotReady || capabilities.Provider.Readiness != ReadinessNotReady {
		t.Fatalf("capabilities = %+v, want readiness axis", capabilities)
	}
	encodedCapabilities, err := json.Marshal(capabilities)
	if err != nil {
		t.Fatalf("marshal capabilities: %v", err)
	}
	var capabilityMap map[string]any
	if err := json.Unmarshal(encodedCapabilities, &capabilityMap); err != nil {
		t.Fatalf("unmarshal capabilities: %v", err)
	}
	if _, ok := capabilityMap["status"]; ok {
		t.Fatalf("capabilities JSON = %s, must not expose top-level status for readiness", string(encodedCapabilities))
	}
	assertNoNestedStatus(t, capabilityMap, "provider")
	assertNoNestedStatus(t, capabilityMap, "runtime_auth")
	assertNoNestedStatus(t, capabilityMap, "ledger")
}

func TestContextRuntimeReadinessDoesNotUseProviderStatus(t *testing.T) {
	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:   NewBlockedProvider("unsafe provider name", "provider_api_key_missing"),
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	snapshot := k.contextRuntimeSnapshot()
	if snapshot.Provider.Readiness != ReadinessNotReady || snapshot.Provider.ReadinessReason != "provider_api_key_missing" {
		t.Fatalf("runtime provider = %+v, want readiness axis", snapshot.Provider)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	var snapshotMap map[string]any
	if err := json.Unmarshal(encoded, &snapshotMap); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	assertNoNestedStatus(t, snapshotMap, "provider")
}

func TestToolDenialMayStillUseBlockedAsModelVisibleOutcome(t *testing.T) {
	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:   FakeProvider{},
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModePlan,
			WorkspaceRoot:  testTempDir(t),
			SandboxProfile: SandboxProfileReadOnly,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	operation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "tool-denial-state-axis",
		Command:   writeFileCommand("denied.txt", "blocked"),
		CWD:       ".",
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if operation.Status != "blocked" || operation.BlockedReason == "" {
		t.Fatalf("operation = %+v, want model-visible blocked tool denial", operation)
	}
}

func assertNoNestedStatus(t *testing.T, root map[string]any, key string) {
	t.Helper()
	value, ok := root[key]
	if !ok {
		t.Fatalf("missing %q in %+v", key, root)
	}
	nested, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%q = %T, want object", key, value)
	}
	if _, ok := nested["status"]; ok {
		t.Fatalf("%q object = %+v, must use readiness/readiness_reason instead of status", key, nested)
	}
}

func assertNoJSONStatusTag(t *testing.T, typ reflect.Type) {
	t.Helper()
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		tagName := strings.Split(field.Tag.Get("json"), ",")[0]
		if tagName == "status" {
			t.Fatalf("%s.%s still exposes generic json status tag", typ.Name(), field.Name)
		}
	}
}
