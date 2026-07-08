package kernel

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentInvocationAdmissionReplaysPolicyAllowedGrant(t *testing.T) {
	workspace := testTempDir(t)
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  workspace,
	})

	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{
		SessionID:       "agent-invocation-session",
		Principal:       "application:test",
		AgentProfileRef: "agent_profile:reviewer",
		ContextScope:    "diff",
		CapabilityGrant: CapabilityGrant{ToolNames: []string{"workspace_edit", "resource_read", "resource_read"}},
		IdempotencyKey:  "delegate-reviewer",
	})
	if err != nil {
		t.Fatalf("AdmitAgentInvocation returned error: %v", err)
	}
	if invocation.InvocationID == "" || invocation.Status != AgentInvocationStatusAdmitted {
		t.Fatalf("invocation = %+v, want admitted id", invocation)
	}
	if strings.Join(invocation.CapabilityGrant.ToolNames, ",") != "resource_read,workspace_edit" {
		t.Fatalf("grant tools = %v, want normalized sorted unique", invocation.CapabilityGrant.ToolNames)
	}
	if invocation.AgentProfileRef != "agent_profile:reviewer" || invocation.ContextScope != "diff" {
		t.Fatalf("invocation refs = %+v", invocation)
	}

	replayed, err := k.AgentInvocation(invocation.InvocationID)
	if err != nil {
		t.Fatalf("AgentInvocation returned error: %v", err)
	}
	if replayed.InvocationID != invocation.InvocationID || replayed.IdempotencyKey != "delegate-reviewer" {
		t.Fatalf("replayed invocation = %+v, want original", replayed)
	}
	sessionInvocations, err := k.AgentInvocations("agent-invocation-session")
	if err != nil {
		t.Fatalf("AgentInvocations returned error: %v", err)
	}
	if len(sessionInvocations) != 1 || sessionInvocations[0].InvocationID != invocation.InvocationID {
		t.Fatalf("session invocations = %+v, want admitted invocation", sessionInvocations)
	}

	payload, err := json.Marshal(invocation)
	if err != nil {
		t.Fatalf("marshal invocation: %v", err)
	}
	for _, forbidden := range []string{workspace, "sandbox_profile", "approval", "provider_route", "credential"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("invocation projection leaked %q: %s", forbidden, string(payload))
		}
	}
}

func TestAgentInvocationAdmissionIdempotencyReturnsExistingFact(t *testing.T) {
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModePlan,
	})
	req := AgentInvocationAdmissionRequest{
		SessionID:       "agent-invocation-idempotent",
		Principal:       "application:test",
		CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}},
		IdempotencyKey:  "same-child",
	}
	first, err := k.AdmitAgentInvocation(req)
	if err != nil {
		t.Fatalf("first AdmitAgentInvocation returned error: %v", err)
	}
	second, err := k.AdmitAgentInvocation(req)
	if err != nil {
		t.Fatalf("second AdmitAgentInvocation returned error: %v", err)
	}
	if second.InvocationID != first.InvocationID {
		t.Fatalf("idempotent invocation id = %q, want %q", second.InvocationID, first.InvocationID)
	}
	items, err := k.AgentInvocations("agent-invocation-idempotent")
	if err != nil {
		t.Fatalf("AgentInvocations returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("invocations = %+v, want one ledger fact", items)
	}
}

func TestAgentInvocationAdmissionRoleDoesNotGrantWriteToolInPlanMode(t *testing.T) {
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModePlan,
	})

	_, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{
		SessionID:       "agent-invocation-role-no-authority",
		Principal:       "application:test",
		AgentProfileRef: "agent_profile:writer",
		CapabilityGrant: CapabilityGrant{ToolNames: []string{"workspace_edit"}},
	})
	if err == nil || !strings.Contains(err.Error(), "capability_grant_tool_not_allowed") {
		t.Fatalf("AdmitAgentInvocation error = %v, want tool-not-allowed", err)
	}
	items, err := k.AgentInvocations("agent-invocation-role-no-authority")
	if err != nil {
		t.Fatalf("AgentInvocations returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("invocations = %+v, want no rejected fact", items)
	}
}

func TestAgentInvocationChildGrantMustBeSubsetOfParent(t *testing.T) {
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  testTempDir(t),
	})
	parent, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{
		SessionID:       "agent-invocation-child",
		Principal:       "application:test",
		CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}},
	})
	if err != nil {
		t.Fatalf("parent AdmitAgentInvocation returned error: %v", err)
	}

	child, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{
		SessionID:           "agent-invocation-child",
		ParentInvocationID:  parent.InvocationID,
		Principal:           "application:test",
		AgentProfileRef:     "agent_profile:reader",
		CapabilityGrant:     CapabilityGrant{ToolNames: []string{"resource_read"}},
		ParentResultChannel: "parent_result:direct",
	})
	if err != nil {
		t.Fatalf("child AdmitAgentInvocation returned error: %v", err)
	}
	if child.ParentInvocationID != parent.InvocationID || child.ParentResultChannel != "parent_result:direct" {
		t.Fatalf("child invocation = %+v, want parent linkage", child)
	}

	_, err = k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{
		SessionID:          "agent-invocation-child",
		ParentInvocationID: parent.InvocationID,
		Principal:          "application:test",
		CapabilityGrant:    CapabilityGrant{ToolNames: []string{"workspace_edit"}},
	})
	if err == nil || !strings.Contains(err.Error(), "capability_grant_exceeds_parent") {
		t.Fatalf("exceeding child error = %v, want capability_grant_exceeds_parent", err)
	}
}

func TestAgentInvocationAdmissionRejectsUnknownParentAndTool(t *testing.T) {
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  testTempDir(t),
	})
	for _, tc := range []struct {
		name string
		req  AgentInvocationAdmissionRequest
		want string
	}{
		{
			name: "unknown parent",
			req: AgentInvocationAdmissionRequest{
				SessionID:          "agent-invocation-invalid",
				ParentInvocationID: "invocation_missing",
				Principal:          "application:test",
				CapabilityGrant:    CapabilityGrant{ToolNames: []string{"resource_read"}},
			},
			want: "parent_invocation_not_found",
		},
		{
			name: "unknown tool",
			req: AgentInvocationAdmissionRequest{
				SessionID:       "agent-invocation-invalid",
				Principal:       "application:test",
				CapabilityGrant: CapabilityGrant{ToolNames: []string{"unknown_tool"}},
			},
			want: "capability_grant_unknown_tool",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := k.AdmitAgentInvocation(tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("AdmitAgentInvocation error = %v, want %s", err, tc.want)
			}
		})
	}
}
