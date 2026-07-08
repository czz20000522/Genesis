package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvocationToolGatewayExposesOnlyGrantedTools(t *testing.T) {
	k := newTestKernelWithResources(t, filepath.Join(testTempDir(t), "events.sqlite"), []ResourceDescriptor{{
		Ref:      "res_invocation",
		MimeType: "text/plain",
		Text:     "invocation resource",
	}})
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{
		SessionID:       "invocation-tool-gateway",
		Principal:       "application:test",
		CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}},
	})
	if err != nil {
		t.Fatalf("AdmitAgentInvocation returned error: %v", err)
	}
	gateway, err := k.ToolGatewayForInvocation(invocation.InvocationID)
	if err != nil {
		t.Fatalf("ToolGatewayForInvocation returned error: %v", err)
	}
	if names := strings.Join(toolSpecNames(gateway.ToolManifest()), ","); names != "resource_read" {
		t.Fatalf("invocation tool manifest = %q, want resource_read only", names)
	}
	projections := gateway.CapabilityProjections()
	if len(projections) != 1 || projections[0].Name != "resource_read" {
		t.Fatalf("capability projections = %+v, want resource_read only", projections)
	}

	prepared, err := gateway.PrepareBatch([]ModelToolCall{{
		ToolCallID:      "call_resource",
		ToolCallEventID: "evt_resource",
		Name:            "resource_read",
		Arguments:       mustMarshalToolArgs(t, map[string]interface{}{"resource_ref": "res_invocation"}),
	}})
	if err != nil {
		t.Fatalf("PrepareBatch returned error: %v", err)
	}
	result, err := gateway.Execute(context.Background(), "invocation-tool-gateway", "turn-invocation-tool-gateway", prepared[0])
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	var payload ModelResourceReadResult
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("unmarshal resource result: %v\n%s", err, result.Content)
	}
	if payload.Text != "invocation resource" {
		t.Fatalf("resource text = %q, want invocation resource", payload.Text)
	}
}

func TestInvocationToolGatewayRejectsUngrantToolsWithoutMutation(t *testing.T) {
	workspace := testTempDir(t)
	filePath := filepath.Join(workspace, "note.txt")
	writeTestFile(t, filePath, "old\n")
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  workspace,
	})
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{
		SessionID:       "invocation-tool-gateway-denied",
		Principal:       "application:test",
		CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}},
	})
	if err != nil {
		t.Fatalf("AdmitAgentInvocation returned error: %v", err)
	}
	gateway, err := k.ToolGatewayForInvocation(invocation.InvocationID)
	if err != nil {
		t.Fatalf("ToolGatewayForInvocation returned error: %v", err)
	}

	prepared, err := gateway.PrepareBatch([]ModelToolCall{{
		ToolCallID:      "call_workspace_edit",
		ToolCallEventID: "evt_workspace_edit",
		Name:            "workspace_edit",
		Arguments: mustMarshalToolArgs(t, map[string]interface{}{
			"path":       "note.txt",
			"old_string": "old",
			"new_string": "new",
		}),
	}})
	if err != nil {
		t.Fatalf("PrepareBatch returned error: %v", err)
	}
	result, err := gateway.Execute(context.Background(), "invocation-tool-gateway-denied", "turn-invocation-tool-gateway-denied", prepared[0])
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if content := readTestFile(t, filePath); content != "old\n" {
		t.Fatalf("file content = %q, want unchanged", content)
	}
	var payload ToolRequestInvalidProjection
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("unmarshal invalid payload: %v\n%s", err, result.Content)
	}
	if payload.Status != "tool_request_invalid" || payload.Executed || payload.Error.Code != "capability_grant_tool_not_allowed" {
		t.Fatalf("invalid payload = %+v, want capability_grant_tool_not_allowed", payload)
	}
}

func TestInvocationToolGatewayRejectsUnknownInvocation(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	_, err := k.ToolGatewayForInvocation("invocation_missing")
	if !errors.Is(err, ErrAgentInvocationNotFound) {
		t.Fatalf("ToolGatewayForInvocation error = %v, want ErrAgentInvocationNotFound", err)
	}
}
