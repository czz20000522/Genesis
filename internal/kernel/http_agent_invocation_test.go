package kernel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestHTTPAgentInvocationAdmitReadAndList(t *testing.T) {
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  testTempDir(t),
	})
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	createResp, err := postJSONWithAuth(server.URL+"/agent-invocations", []byte(`{
		"session_id": "http-agent-invocation",
		"principal": "application:test",
		"agent_profile_ref": "agent_profile:reviewer",
		"capability_grant": {"tool_names": ["workspace_edit", "resource_read", "resource_read"]},
		"context_scope": "diff",
		"parent_result_channel": "parent_result:direct",
		"idempotency_key": "http-child-1"
	}`))
	if err != nil {
		t.Fatalf("POST /agent-invocations failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("create invocation status = %d, want 200", createResp.StatusCode)
	}
	var created AgentInvocationProjection
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created invocation: %v", err)
	}
	if created.InvocationID == "" || created.Status != AgentInvocationStatusAdmitted {
		t.Fatalf("created invocation = %+v, want admitted id", created)
	}
	if len(created.CapabilityGrant.ToolNames) != 2 || created.CapabilityGrant.ToolNames[0] != "resource_read" || created.CapabilityGrant.ToolNames[1] != "workspace_edit" {
		t.Fatalf("created grant = %+v, want normalized grant", created.CapabilityGrant)
	}

	readResp, err := getWithAuth(server.URL + "/agent-invocations/" + created.InvocationID)
	if err != nil {
		t.Fatalf("GET /agent-invocations/{id} failed: %v", err)
	}
	defer readResp.Body.Close()
	if readResp.StatusCode != http.StatusOK {
		t.Fatalf("read invocation status = %d, want 200", readResp.StatusCode)
	}
	var readBack AgentInvocationProjection
	if err := json.NewDecoder(readResp.Body).Decode(&readBack); err != nil {
		t.Fatalf("decode read invocation: %v", err)
	}
	if readBack.InvocationID != created.InvocationID || readBack.IdempotencyKey != "http-child-1" {
		t.Fatalf("read invocation = %+v, want original", readBack)
	}

	listResp, err := getWithAuth(server.URL + "/sessions/http-agent-invocation/agent-invocations")
	if err != nil {
		t.Fatalf("GET /sessions/{id}/agent-invocations failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list invocation status = %d, want 200", listResp.StatusCode)
	}
	var list []AgentInvocationProjection
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode invocation list: %v", err)
	}
	if len(list) != 1 || list[0].InvocationID != created.InvocationID {
		t.Fatalf("listed invocations = %+v, want created invocation", list)
	}
}

func TestHTTPAgentInvocationReadUnknownReturnsNotFound(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := getWithAuth(server.URL + "/agent-invocations/invocation_missing")
	if err != nil {
		t.Fatalf("GET /agent-invocations/{id} failed: %v", err)
	}
	defer resp.Body.Close()
	assertErrorCode(t, resp, http.StatusNotFound, "not_found")
}
