package kernel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSessionDebugDefaultOffDoesNotCreateArtifactOrBreakResume(t *testing.T) {
	sessionID := "debug-default-off"
	materialRoot := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k, err := New(Config{
		LedgerPath:        ledgerPath,
		MaterialStorePath: materialRoot,
		Provider:          &recordingTextProvider{text: "plain final"},
		RuntimeToken:      testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "debug off"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "plain final" {
		t.Fatalf("final = %q", resp.Final.Text)
	}
	export, err := k.SessionDebugExport(sessionID)
	if err != nil {
		t.Fatalf("SessionDebugExport returned error: %v", err)
	}
	if export.Readiness != ReadinessNotReady || export.ReadinessReason != "session_debug_disabled" {
		t.Fatalf("debug export = %+v, want disabled", export)
	}
	if _, err := os.Stat(filepath.Join(materialRoot, "session-debug")); !os.IsNotExist(err) {
		t.Fatalf("session debug artifact dir stat err = %v, want not exist", err)
	}

	restarted, err := New(Config{
		LedgerPath:        ledgerPath,
		MaterialStorePath: materialRoot,
		Provider:          &recordingTextProvider{text: "unused"},
		RuntimeToken:      testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("restart New returned error: %v", err)
	}
	session, err := restarted.Session(sessionID)
	if err != nil {
		t.Fatalf("Session after restart returned error: %v", err)
	}
	if len(session.Turns) != 1 || session.Turns[0].FinalMessage.Text != "plain final" {
		t.Fatalf("session after restart = %+v, want ledger-backed final", session.Turns)
	}
}

func TestSessionDebugCapturesProviderStepsAndToolLoopWithoutHostPaths(t *testing.T) {
	sessionID := "debug-tool-loop"
	materialRoot := testTempDir(t)
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":     workspace,
		"command": echoCommand("debug-output"),
	})
	if err != nil {
		t.Fatalf("marshal shell arguments: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{{ToolCallID: "call_debug_shell", Name: "shell_exec", Arguments: arguments}},
		final: "debug final",
		usages: []*TokenUsage{
			{InputTokens: 11, OutputTokens: 3, TotalTokens: 14},
			{InputTokens: 22, OutputTokens: 4, TotalTokens: 26},
		},
	}
	k, err := New(Config{
		LedgerPath:        filepath.Join(testTempDir(t), "events.sqlite"),
		MaterialStorePath: materialRoot,
		Provider:          provider,
		RuntimeToken:      testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if _, err := k.EnableSessionDebug(sessionID); err != nil {
		t.Fatalf("EnableSessionDebug returned error: %v", err)
	}
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "run a debug shell command"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}

	export, err := k.SessionDebugExport(sessionID)
	if err != nil {
		t.Fatalf("SessionDebugExport returned error: %v", err)
	}
	if export.Readiness != ReadinessReady || len(export.Steps) != 2 {
		t.Fatalf("debug export = %+v, want two provider steps", export)
	}
	if !containsString(export.Steps[0].ModelInputKinds, ModelInputKindUserText) {
		t.Fatalf("step 0 input kinds = %+v, want user text", export.Steps[0].ModelInputKinds)
	}
	if !containsString(inspectedToolNames(export.Steps[0].ToolManifest), "shell_exec") {
		t.Fatalf("tool manifest = %+v, want shell_exec", inspectedToolNames(export.Steps[0].ToolManifest))
	}
	if len(export.Steps[0].ToolCalls) != 1 || export.Steps[0].ToolCalls[0].Name != "shell_exec" {
		t.Fatalf("step 0 tool calls = %+v, want shell_exec summary", export.Steps[0].ToolCalls)
	}
	if !containsString(export.Steps[0].ToolCalls[0].ArgumentFields, "cwd") || !containsString(export.Steps[0].ToolCalls[0].ArgumentFields, "command") {
		t.Fatalf("argument fields = %+v, want cwd/command names without raw values", export.Steps[0].ToolCalls[0].ArgumentFields)
	}
	if len(export.Steps[1].ToolRounds) != 1 || len(export.Steps[1].ToolRounds[0].Results) != 1 {
		t.Fatalf("step 1 tool rounds = %+v, want one result", export.Steps[1].ToolRounds)
	}
	if !strings.Contains(export.Steps[1].ToolRounds[0].Results[0].ContentPreview, "debug-output") {
		t.Fatalf("tool result preview = %q, want shell stdout", export.Steps[1].ToolRounds[0].Results[0].ContentPreview)
	}
	if export.Steps[1].Final == nil || export.Steps[1].Final.Text != "debug final" {
		t.Fatalf("step 1 final = %+v, want debug final", export.Steps[1].Final)
	}
	if export.Steps[1].Usage == nil || export.Steps[1].Usage.TotalTokens != 26 {
		t.Fatalf("step 1 usage = %+v, want provider usage", export.Steps[1].Usage)
	}
	exportJSON, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	for _, forbidden := range []string{workspace, "Authorization", "secret://", "SKILL.md", "storage_ref", "input_schema"} {
		if strings.Contains(string(exportJSON), forbidden) {
			t.Fatalf("debug export leaked %q: %s", forbidden, string(exportJSON))
		}
	}
}

func TestSessionDebugExportIsStepBoundedAndUTF8Safe(t *testing.T) {
	sessionID := "debug-bounds"
	materialRoot := testTempDir(t)
	k, err := New(Config{
		LedgerPath:        filepath.Join(testTempDir(t), "events.sqlite"),
		MaterialStorePath: materialRoot,
		Provider:          &recordingTextProvider{text: strings.Repeat("界", sessionDebugTextBytes)},
		RuntimeToken:      testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if _, err := k.EnableSessionDebug(sessionID); err != nil {
		t.Fatalf("EnableSessionDebug returned error: %v", err)
	}
	for i := 0; i < sessionDebugMaxSteps+2; i++ {
		if _, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  sessionID,
			InputItems: []InputItem{{Type: "text", Text: "debug " + strconv.Itoa(i)}},
		}); err != nil {
			t.Fatalf("SubmitTurn %d returned error: %v", i, err)
		}
	}

	export, err := k.SessionDebugExport(sessionID)
	if err != nil {
		t.Fatalf("SessionDebugExport returned error: %v", err)
	}
	if len(export.Steps) != sessionDebugMaxSteps {
		t.Fatalf("debug steps = %d, want max %d", len(export.Steps), sessionDebugMaxSteps)
	}
	if !export.CaptureBounds.Truncated || export.CaptureBounds.MaxSteps != sessionDebugMaxSteps {
		t.Fatalf("capture bounds = %+v, want truncated max-steps evidence", export.CaptureBounds)
	}
	last := export.Steps[len(export.Steps)-1]
	if last.Final == nil || !utf8.ValidString(last.Final.Text) || !last.CaptureBounds.Truncated {
		t.Fatalf("last final/capture bounds = %+v %+v, want utf8-safe bounded final", last.Final, last.CaptureBounds)
	}
}

func TestSessionDebugArtifactDeletionDoesNotAffectSessionOrInspection(t *testing.T) {
	sessionID := "debug-delete-artifact"
	materialRoot := testTempDir(t)
	k, err := New(Config{
		LedgerPath:        filepath.Join(testTempDir(t), "events.sqlite"),
		MaterialStorePath: materialRoot,
		Provider:          &recordingTextProvider{text: "kept final"},
		RuntimeToken:      testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if _, err := k.EnableSessionDebug(sessionID); err != nil {
		t.Fatalf("EnableSessionDebug returned error: %v", err)
	}
	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "capture then delete"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if export, err := k.SessionDebugExport(sessionID); err != nil || export.Readiness != ReadinessReady || len(export.Steps) != 1 {
		t.Fatalf("debug export before deletion = %+v err=%v", export, err)
	}
	if err := os.RemoveAll(filepath.Join(materialRoot, "session-debug")); err != nil {
		t.Fatalf("remove debug artifact dir: %v", err)
	}

	session, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error after deleting artifact: %v", err)
	}
	if len(session.Turns) != 1 || session.Turns[0].FinalMessage.Text != "kept final" {
		t.Fatalf("session = %+v, want final from ledger", session.Turns)
	}
	inspection, err := k.ContextInspection(resp.TurnID)
	if err != nil {
		t.Fatalf("ContextInspection returned error after deleting artifact: %v", err)
	}
	if inspection.Readiness != ReadinessReady || !containsString(inspection.ModelInputKinds, ModelInputKindUserText) {
		t.Fatalf("context inspection = %+v, want ledger-backed context", inspection)
	}
}

func TestSessionDebugHTTPEnableAndExport(t *testing.T) {
	sessionID := "debug-http"
	k, err := New(Config{
		LedgerPath:        filepath.Join(testTempDir(t), "events.sqlite"),
		MaterialStorePath: testTempDir(t),
		Provider:          &recordingTextProvider{text: "http debug final"},
		RuntimeToken:      testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	enableResp, err := postJSONWithAuth(server.URL+"/sessions/"+sessionID+"/debug/enable", []byte(`{}`))
	if err != nil {
		t.Fatalf("enable debug request failed: %v", err)
	}
	defer enableResp.Body.Close()
	if enableResp.StatusCode != http.StatusOK {
		t.Fatalf("enable debug status = %d, want 200", enableResp.StatusCode)
	}
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "http debug"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}

	exportResp, err := getWithAuth(server.URL + "/sessions/" + sessionID + "/debug")
	if err != nil {
		t.Fatalf("get debug export failed: %v", err)
	}
	defer exportResp.Body.Close()
	if exportResp.StatusCode != http.StatusOK {
		t.Fatalf("debug export status = %d, want 200", exportResp.StatusCode)
	}
	var export SessionDebugExportResponse
	if err := json.NewDecoder(exportResp.Body).Decode(&export); err != nil {
		t.Fatalf("decode debug export: %v", err)
	}
	if export.Readiness != ReadinessReady || len(export.Steps) != 1 || export.Steps[0].Final == nil || export.Steps[0].Final.Text != "http debug final" {
		t.Fatalf("debug export = %+v, want final provider step", export)
	}
}

func TestSessionDebugCapturesProviderAuthFailureWithoutSecret(t *testing.T) {
	sessionID := "debug-provider-auth"
	const secret = "sk-debug-auth-secret"
	k, err := New(Config{
		LedgerPath:        filepath.Join(testTempDir(t), "events.sqlite"),
		MaterialStorePath: testTempDir(t),
		Provider:          sessionDebugFailingProvider{err: newProviderStatusError(http.StatusUnauthorized, "invalid "+secret, 0)},
		RuntimeToken:      testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if _, err := k.EnableSessionDebug(sessionID); err != nil {
		t.Fatalf("EnableSessionDebug returned error: %v", err)
	}
	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "trigger provider auth failure"}},
	})
	if err == nil {
		t.Fatal("SubmitTurn returned nil error, want provider auth failure")
	}

	export, err := k.SessionDebugExport(sessionID)
	if err != nil {
		t.Fatalf("SessionDebugExport returned error: %v", err)
	}
	if export.Readiness != ReadinessReady || len(export.Steps) != 1 || export.Steps[0].Error == nil {
		t.Fatalf("debug export = %+v, want one failure step", export)
	}
	if export.Steps[0].Error.ReasonCode != "provider_auth_failed" {
		t.Fatalf("debug error = %+v, want provider_auth_failed", export.Steps[0].Error)
	}
	exportJSON, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	if strings.Contains(string(exportJSON), secret) || strings.Contains(string(exportJSON), "invalid "+secret) {
		t.Fatalf("debug export leaked provider auth secret: %s", string(exportJSON))
	}
}

type sessionDebugFailingProvider struct {
	err error
}

func (p sessionDebugFailingProvider) Name() string {
	return "debug-failing-provider"
}

func (p sessionDebugFailingProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p sessionDebugFailingProvider) Complete(context.Context, ModelRequest) (ModelResponse, error) {
	return ModelResponse{}, p.err
}
