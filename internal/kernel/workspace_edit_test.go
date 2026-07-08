package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceEditRegisteredAsSerialWorkspaceWriteTool(t *testing.T) {
	workspace := testTempDir(t)
	writeTestFile(t, filepath.Join(workspace, "note.txt"), "old\n")
	k := newWorkspaceEditTestKernelWithWorkspace(t, workspace, PermissionModeYolo)
	manifest := k.toolGateway().ToolManifest()
	if !containsString(toolSpecNames(manifest), "workspace_edit") {
		t.Fatalf("tool manifest names = %v, want workspace_edit", toolSpecNames(manifest))
	}

	args := mustMarshalToolArgs(t, map[string]interface{}{
		"path":       "note.txt",
		"old_string": "old",
		"new_string": "new",
	})
	prepared, err := k.toolGateway().PrepareBatch([]ModelToolCall{{
		ToolCallID:      "call_workspace_edit",
		ToolCallEventID: "evt_workspace_edit",
		Name:            "workspace_edit",
		Arguments:       args,
	}})
	if err != nil {
		t.Fatalf("PrepareBatch returned error: %v", err)
	}
	if len(prepared) != 1 {
		t.Fatalf("prepared calls = %d, want 1", len(prepared))
	}
	plan := prepared[0].accessPlan
	if plan.ToolName != "workspace_edit" || plan.EffectClass != ToolEffectClassWorkspaceWrite || plan.ParallelPolicy != ToolParallelPolicySerialFence || !plan.Trusted {
		t.Fatalf("workspace_edit access plan = %+v, want trusted serial workspace write", plan)
	}
	if plan.ParallelClass() != "" {
		t.Fatalf("workspace_edit parallel class = %q, want serial", plan.ParallelClass())
	}
}

func TestWorkspaceEditReplacesUniqueString(t *testing.T) {
	workspace := testTempDir(t)
	filePath := filepath.Join(workspace, "src", "note.txt")
	writeTestFile(t, filePath, "alpha old omega\n")
	k := newWorkspaceEditTestKernelWithWorkspace(t, workspace, PermissionModeYolo)

	result := executeWorkspaceEditTool(t, k, map[string]interface{}{
		"path":       "src/note.txt",
		"old_string": "old",
		"new_string": "new",
	})

	content := readTestFile(t, filePath)
	if content != "alpha new omega\n" {
		t.Fatalf("file content = %q, want replacement", content)
	}
	payload := decodeJSONMap(t, result.Content)
	if payload["status"] != "completed" || payload["executed"] != true || payload["tool"] != "workspace_edit" || payload["path"] != "src/note.txt" {
		t.Fatalf("workspace_edit result = %+v, want completed semantic payload", payload)
	}
	if payload["replacements"] != float64(1) {
		t.Fatalf("replacements = %#v, want 1", payload["replacements"])
	}
	for _, forbidden := range []string{workspace, filePath} {
		if strings.Contains(result.Content, forbidden) {
			t.Fatalf("workspace_edit result leaked %q: %s", forbidden, result.Content)
		}
	}
}

func TestWorkspaceEditAppliesOrderedMultiEditAtomically(t *testing.T) {
	workspace := testTempDir(t)
	filePath := filepath.Join(workspace, "src", "note.txt")
	writeTestFile(t, filePath, "package old\nold value\n")
	k := newWorkspaceEditTestKernelWithWorkspace(t, workspace, PermissionModeYolo)

	result := executeWorkspaceEditTool(t, k, map[string]interface{}{
		"path": "src/note.txt",
		"edits": []map[string]interface{}{
			{"old_string": "package old", "new_string": "package new"},
			{"old_string": "old value", "new_string": "new value"},
		},
	})

	if content := readTestFile(t, filePath); content != "package new\nnew value\n" {
		t.Fatalf("file content = %q, want ordered multi-edit", content)
	}
	payload := decodeJSONMap(t, result.Content)
	if payload["status"] != "completed" || payload["executed"] != true || payload["path"] != "src/note.txt" {
		t.Fatalf("workspace_edit result = %+v, want completed multi-edit payload", payload)
	}
	if payload["replacements"] != float64(2) {
		t.Fatalf("replacements = %#v, want 2", payload["replacements"])
	}
}

func TestWorkspaceEditMultiEditFailureLeavesFileUnchanged(t *testing.T) {
	workspace := testTempDir(t)
	filePath := filepath.Join(workspace, "note.txt")
	original := "alpha beta gamma\n"
	writeTestFile(t, filePath, original)
	k := newWorkspaceEditTestKernelWithWorkspace(t, workspace, PermissionModeYolo)

	result := executeWorkspaceEditTool(t, k, map[string]interface{}{
		"path": "note.txt",
		"edits": []map[string]interface{}{
			{"old_string": "alpha", "new_string": "ALPHA"},
			{"old_string": "missing", "new_string": "MISSING"},
		},
	})

	if content := readTestFile(t, filePath); content != original {
		t.Fatalf("file content = %q, want unchanged %q", content, original)
	}
	assertWorkspaceEditInvalid(t, result.Content, "workspace_edit_old_string_not_found")
}

func TestWorkspaceEditPlanModeDeniesWithoutMutation(t *testing.T) {
	workspace := testTempDir(t)
	filePath := filepath.Join(workspace, "note.txt")
	writeTestFile(t, filePath, "old\n")
	k := newWorkspaceEditTestKernelWithWorkspace(t, workspace, PermissionModePlan)

	result := executeWorkspaceEditTool(t, k, map[string]interface{}{
		"path":       "note.txt",
		"old_string": "old",
		"new_string": "new",
	})

	if content := readTestFile(t, filePath); content != "old\n" {
		t.Fatalf("file content = %q, want unchanged", content)
	}
	var payload ToolRequestInvalidProjection
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("unmarshal denial payload: %v\n%s", err, result.Content)
	}
	if payload.Status != "permission_denied" || payload.Executed || payload.Error.Code != "permission_denied" {
		t.Fatalf("denial payload = %+v, want permission_denied", payload)
	}
}

func TestWorkspaceEditRejectsTraversalOutsideWorkspace(t *testing.T) {
	workspace := testTempDir(t)
	outside := filepath.Join(filepath.Dir(workspace), "outside.txt")
	writeTestFile(t, outside, "old\n")
	k := newWorkspaceEditTestKernelWithWorkspace(t, workspace, PermissionModeYolo)

	result := executeWorkspaceEditTool(t, k, map[string]interface{}{
		"path":       "../outside.txt",
		"old_string": "old",
		"new_string": "new",
	})

	if content := readTestFile(t, outside); content != "old\n" {
		t.Fatalf("outside content = %q, want unchanged", content)
	}
	assertWorkspaceEditInvalid(t, result.Content, "path_outside_workspace")
	if strings.Contains(result.Content, outside) || strings.Contains(result.Content, workspace) {
		t.Fatalf("invalid result leaked host path: %s", result.Content)
	}
}

func TestWorkspaceEditRejectsSymlinkEscape(t *testing.T) {
	workspace := testTempDir(t)
	outsideDir := testTempDir(t)
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	writeTestFile(t, outsideFile, "old\n")
	linkPath := filepath.Join(workspace, "link.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Skipf("symlink unavailable on this host: %v", err)
	}
	k := newWorkspaceEditTestKernelWithWorkspace(t, workspace, PermissionModeYolo)

	result := executeWorkspaceEditTool(t, k, map[string]interface{}{
		"path":       "link.txt",
		"old_string": "old",
		"new_string": "new",
	})

	if content := readTestFile(t, outsideFile); content != "old\n" {
		t.Fatalf("outside symlink target content = %q, want unchanged", content)
	}
	assertWorkspaceEditInvalid(t, result.Content, "path_outside_workspace")
}

func TestWorkspaceEditRejectsMissingAndNonUniqueOldStringWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		old     string
		code    string
	}{
		{name: "not found", content: "alpha beta\n", old: "old", code: "workspace_edit_old_string_not_found"},
		{name: "not unique", content: "old alpha old\n", old: "old", code: "workspace_edit_old_string_not_unique"},
		{name: "empty old", content: "alpha beta\n", old: "", code: "workspace_edit_old_string_required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace := testTempDir(t)
			filePath := filepath.Join(workspace, "note.txt")
			writeTestFile(t, filePath, tc.content)
			k := newWorkspaceEditTestKernelWithWorkspace(t, workspace, PermissionModeYolo)

			result := executeWorkspaceEditTool(t, k, map[string]interface{}{
				"path":       "note.txt",
				"old_string": tc.old,
				"new_string": "new",
			})

			if content := readTestFile(t, filePath); content != tc.content {
				t.Fatalf("file content = %q, want unchanged %q", content, tc.content)
			}
			assertWorkspaceEditInvalid(t, result.Content, tc.code)
		})
	}
}

func newWorkspaceEditTestKernel(t *testing.T, permissionMode string) *Kernel {
	t.Helper()
	workspace := testTempDir(t)
	return newWorkspaceEditTestKernelWithWorkspace(t, workspace, permissionMode)
}

func newWorkspaceEditTestKernelWithWorkspace(t *testing.T, workspace string, permissionMode string) *Kernel {
	t.Helper()
	return newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: permissionMode,
		WorkspaceRoot:  workspace,
	})
}

func executeWorkspaceEditTool(t *testing.T, k *Kernel, args map[string]interface{}) ModelToolResult {
	t.Helper()
	prepared, err := k.toolGateway().PrepareBatch([]ModelToolCall{{
		ToolCallID:      "call_workspace_edit",
		ToolCallEventID: "evt_workspace_edit",
		Name:            "workspace_edit",
		Arguments:       mustMarshalToolArgs(t, args),
	}})
	if err != nil {
		t.Fatalf("PrepareBatch returned error: %v", err)
	}
	if len(prepared) != 1 {
		t.Fatalf("prepared calls = %d, want 1", len(prepared))
	}
	result, err := k.toolGateway().Execute(context.Background(), "workspace-edit-session", "turn-workspace-edit", prepared[0])
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	return result
}

func assertWorkspaceEditInvalid(t *testing.T, content string, code string) {
	t.Helper()
	var payload ToolRequestInvalidProjection
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("unmarshal invalid payload: %v\n%s", err, content)
	}
	if payload.Status != "tool_request_invalid" || payload.Executed || payload.Error.Code != code {
		t.Fatalf("invalid payload = %+v, want code %q", payload, code)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
