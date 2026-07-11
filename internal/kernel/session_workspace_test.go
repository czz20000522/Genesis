package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionWorkspaceBindingPersistsModeWithoutLeakingRoot(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	workspace := testTempDir(t)
	k := newTestKernel(t, ledgerPath)

	if err := k.BindSessionWorkspace("project-session", SessionWorkspaceBindingRequest{
		Kind: SessionWorkspaceKindProject,
		Root: workspace,
	}); err != nil {
		t.Fatalf("BindSessionWorkspace returned error: %v", err)
	}
	k.Close()

	restarted := newTestKernel(t, ledgerPath)
	defer restarted.Close()
	projection, err := restarted.Session("project-session")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if projection.WorkspaceMode != SessionWorkspaceKindProject {
		t.Fatalf("WorkspaceMode = %q, want project", projection.WorkspaceMode)
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("marshal session projection: %v", err)
	}
	if strings.Contains(string(encoded), workspace) {
		t.Fatalf("session projection leaked workspace root: %s", string(encoded))
	}
}

func TestSubmitTurnUsesBoundWorkspaceInsteadOfGlobalRoot(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	globalRoot := testTempDir(t)
	projectRoot := testTempDir(t)
	provider := &toolFeedbackProvider{calls: []ModelToolCall{{
		ToolCallID: "call_workspace",
		Name:       "shell_exec",
		Arguments:  json.RawMessage(`{"command":"pwd"}`),
	}}}
	k, err := New(Config{
		LedgerPath: ledgerPath,
		Provider:   provider,
		ToolPolicy: ToolPolicy{PermissionMode: PermissionModeDefault, WorkspaceRoot: globalRoot},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer k.Close()
	if err := k.BindSessionWorkspace("project-session", SessionWorkspaceBindingRequest{
		Kind: SessionWorkspaceKindProject,
		Root: projectRoot,
	}); err != nil {
		t.Fatalf("BindSessionWorkspace returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "project-session",
		InputItems: []InputItem{{Type: "text", Text: "show the current directory"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	projection, err := k.Session("project-session")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 1 || projection.Operations[0].CWD != projectRoot {
		t.Fatalf("operations = %+v, want bound project root %q", projection.Operations, projectRoot)
	}
}

func TestDefaultSessionCanReadExplicitSiblingWorkspace(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	primaryRoot := testTempDir(t)
	referenceRoot := testTempDir(t)
	writeTestFile(t, filepath.Join(referenceRoot, "reference.txt"), "reference implementation\n")
	provider := &toolFeedbackProvider{calls: []ModelToolCall{{
		ToolCallID: "call_reference",
		Name:       "shell_exec",
		Arguments:  json.RawMessage(`{"cwd":"` + strings.ReplaceAll(referenceRoot, `\`, `\\`) + `","command":"cat reference.txt"}`),
	}}}
	k, err := New(Config{
		LedgerPath: ledgerPath,
		Provider:   provider,
		ToolPolicy: ToolPolicy{PermissionMode: PermissionModeDefault, WorkspaceRoot: primaryRoot},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer k.Close()
	if err := k.BindSessionWorkspace("project-session", SessionWorkspaceBindingRequest{Kind: SessionWorkspaceKindProject, Root: primaryRoot}); err != nil {
		t.Fatalf("BindSessionWorkspace returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{SessionID: "project-session", InputItems: []InputItem{{Type: "text", Text: "inspect sibling"}}}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	projection, err := k.Session("project-session")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 1 || projection.Operations[0].Status != "completed" || projection.Operations[0].Stdout != "reference implementation\n" {
		t.Fatalf("operations = %+v, want completed sibling read", projection.Operations)
	}
}

func TestPlanSessionCanReadExplicitSiblingWorkspace(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	referenceRoot := testTempDir(t)
	writeTestFile(t, filepath.Join(referenceRoot, "reference.txt"), "plan reference\n")
	provider := &toolFeedbackProvider{calls: []ModelToolCall{{
		ToolCallID: "call_reference",
		Name:       "shell_exec",
		Arguments:  json.RawMessage(`{"cwd":"` + strings.ReplaceAll(referenceRoot, `\`, `\\`) + `","command":"cat reference.txt"}`),
	}}}
	k, err := New(Config{
		LedgerPath: ledgerPath,
		Provider:   provider,
		ToolPolicy: ToolPolicy{PermissionMode: PermissionModePlan},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer k.Close()
	if err := k.BindSessionWorkspace("chat-session", SessionWorkspaceBindingRequest{Kind: SessionWorkspaceKindNone}); err != nil {
		t.Fatalf("BindSessionWorkspace returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{SessionID: "chat-session", InputItems: []InputItem{{Type: "text", Text: "inspect sibling"}}}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	projection, err := k.Session("chat-session")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 1 || projection.Operations[0].Status != "completed" || projection.Operations[0].Stdout != "plan reference\n" {
		t.Fatalf("operations = %+v, want completed plan-mode sibling read", projection.Operations)
	}
}

func TestDefaultSessionBlocksExplicitSiblingWrite(t *testing.T) {
	primaryRoot := testTempDir(t)
	referenceRoot := testTempDir(t)
	writePath := filepath.Join(referenceRoot, "blocked.txt")
	k := newSessionWorkspaceWriteKernel(t, PermissionModeDefault, primaryRoot, referenceRoot, writePath)
	defer k.Close()
	projection := submitSessionWorkspaceWriteTurn(t, k, "project-session", primaryRoot)
	if len(projection.Operations) != 1 || projection.Operations[0].Status != "blocked" {
		t.Fatalf("operations = %+v, want blocked default-mode sibling write", projection.Operations)
	}
	if _, err := os.Stat(writePath); !os.IsNotExist(err) {
		t.Fatalf("sibling write target exists or stat failed: %v", err)
	}
}

func TestYoloSessionCanWriteExplicitSiblingWorkspace(t *testing.T) {
	primaryRoot := testTempDir(t)
	referenceRoot := testTempDir(t)
	writePath := filepath.Join(referenceRoot, "allowed.txt")
	k := newSessionWorkspaceWriteKernel(t, PermissionModeYolo, primaryRoot, referenceRoot, writePath)
	defer k.Close()
	projection := submitSessionWorkspaceWriteTurn(t, k, "project-session", primaryRoot)
	if len(projection.Operations) != 1 || projection.Operations[0].Status != "completed" {
		t.Fatalf("operations = %+v, want completed yolo sibling write", projection.Operations)
	}
	content, err := os.ReadFile(writePath)
	if err != nil || strings.TrimSpace(string(content)) != "cross-workspace" {
		t.Fatalf("sibling content = %q, err = %v", string(content), err)
	}
}

func newSessionWorkspaceWriteKernel(t *testing.T, permissionMode string, primaryRoot string, referenceRoot string, writePath string) *Kernel {
	t.Helper()
	provider := &toolFeedbackProvider{calls: []ModelToolCall{{
		ToolCallID: "call_write",
		Name:       "shell_exec",
		Arguments:  json.RawMessage(`{"cwd":"` + strings.ReplaceAll(referenceRoot, `\`, `\\`) + `","command":"Set-Content -LiteralPath ` + strings.ReplaceAll(writePath, `\`, `\\`) + ` -Value cross-workspace"}`),
	}}}
	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:   provider,
		ToolPolicy: ToolPolicy{PermissionMode: permissionMode, WorkspaceRoot: primaryRoot},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return k
}

func submitSessionWorkspaceWriteTurn(t *testing.T, k *Kernel, sessionID string, primaryRoot string) SessionProjection {
	t.Helper()
	if err := k.BindSessionWorkspace(sessionID, SessionWorkspaceBindingRequest{Kind: SessionWorkspaceKindProject, Root: primaryRoot}); err != nil {
		t.Fatalf("BindSessionWorkspace returned error: %v", err)
	}
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{SessionID: sessionID, InputItems: []InputItem{{Type: "text", Text: "write sibling"}}}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	projection, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	return projection
}
