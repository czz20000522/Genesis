package kernel

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestAdmitWorkerInvocationFromRoleUsesPresetToolsAndAllowsSameRoleInstances(t *testing.T) {
	configRoot := writeParentWorkerRuntimeConfig(t, []string{"resource_read", "source_read"})
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModePlan,
	})

	first, err := k.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{
		ConfigRoot:     configRoot,
		SessionID:      "worker-role-session",
		Principal:      "application:test",
		RoleID:         "local-small-worker",
		IdempotencyKey: "worker-1",
	})
	if err != nil {
		t.Fatalf("first AdmitWorkerInvocationFromRole returned error: %v", err)
	}
	second, err := k.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{
		ConfigRoot:     configRoot,
		SessionID:      "worker-role-session",
		Principal:      "application:test",
		RoleID:         "local-small-worker",
		IdempotencyKey: "worker-2",
	})
	if err != nil {
		t.Fatalf("second AdmitWorkerInvocationFromRole returned error: %v", err)
	}
	if first.InvocationID == second.InvocationID {
		t.Fatalf("same role produced same invocation id %q", first.InvocationID)
	}
	if first.AgentProfileRef != "agent_profile:local-small-worker" || second.AgentProfileRef != first.AgentProfileRef {
		t.Fatalf("agent profile refs = %q, %q", first.AgentProfileRef, second.AgentProfileRef)
	}
	if strings.Join(first.CapabilityGrant.ToolNames, ",") != "resource_read,source_read" {
		t.Fatalf("first tool grant = %v, want role preset tools", first.CapabilityGrant.ToolNames)
	}
	if strings.Join(second.CapabilityGrant.ToolNames, ",") != "resource_read,source_read" {
		t.Fatalf("second tool grant = %v, want role preset tools", second.CapabilityGrant.ToolNames)
	}
	items, err := k.AgentInvocations("worker-role-session")
	if err != nil {
		t.Fatalf("AgentInvocations returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("invocations = %+v, want two same-role worker identities", items)
	}
}

func TestAdmitWorkerInvocationFromRoleRejectsExtraParentToolsBeforeAppend(t *testing.T) {
	configRoot := writeParentWorkerRuntimeConfig(t, []string{"resource_read"})
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  testTempDir(t),
	})

	_, err := k.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{
		ConfigRoot:         configRoot,
		SessionID:          "worker-extra-tool-session",
		Principal:          "application:test",
		RoleID:             "local-small-worker",
		RequestedToolNames: []string{"resource_read", "workspace_edit"},
	})
	if err == nil || !strings.Contains(err.Error(), "capability_grant_exceeds_role") {
		t.Fatalf("AdmitWorkerInvocationFromRole error = %v, want capability_grant_exceeds_role", err)
	}
	items, err := k.AgentInvocations("worker-extra-tool-session")
	if err != nil {
		t.Fatalf("AgentInvocations returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("invocations = %+v, want no rejected worker fact", items)
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

func writeParentWorkerRuntimeConfig(t *testing.T, toolSet []string) string {
	t.Helper()
	tools := make([]any, 0, len(toolSet))
	for _, tool := range toolSet {
		tools = append(tools, tool)
	}
	return writeModelsConfig(t, map[string]any{
		"model_gateway": map[string]any{
			"protocol":       "openai-chat-completions",
			"base_url":       "https://provider.example.com/api",
			"credential_ref": "secret://models/provider/default",
		},
		"active_model_profile_bindings": map[string]any{
			DefaultModelRole: "parent-profile",
		},
		"model_profiles": map[string]any{
			"cloud": map[string]any{
				"gateway": map[string]any{
					"parent-profile": map[string]any{
						"profile_id": "parent-profile",
						"model_id":   "frontier-parent",
					},
					"worker-profile": map[string]any{
						"profile_id": "worker-profile",
						"model_id":   "worker-model",
					},
				},
			},
		},
		"parent_worker_runtime": map[string]any{
			"parents": map[string]any{
				DefaultModelRole: map[string]any{
					"allowed_worker_roles": []any{"local-small-worker"},
					"default_worker_role":  "local-small-worker",
					"can_create_workers":   true,
				},
			},
			"worker_roles": map[string]any{
				"local-small-worker": map[string]any{
					"profile_id": "worker-profile",
					"tool_set":   tools,
					"leaf_only":  true,
				},
			},
		},
	})
}

func TestAgentInvocationRunReturnsBoundedFinalWithoutParentTurn(t *testing.T) {
	provider := &recordingTextProvider{text: "child final"}
	k := newAgentInvocationRunTestKernel(t, Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:   provider,
		ToolPolicy: ToolPolicy{PermissionMode: PermissionModePlan},
	})
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{
		SessionID:       "agent-invocation-run",
		Principal:       "application:test",
		ContextScope:    "focused",
		CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}},
	})
	if err != nil {
		t.Fatalf("AdmitAgentInvocation returned error: %v", err)
	}

	run, err := k.RunAgentInvocation(context.Background(), AgentInvocationRunRequest{
		InvocationID:   invocation.InvocationID,
		Principal:      "application:test",
		InputItems:     []InputItem{{Type: "text", Text: "summarize this file"}},
		IdempotencyKey: "run-once",
	})
	if err != nil {
		t.Fatalf("RunAgentInvocation returned error: %v", err)
	}
	if run.Status != AgentInvocationRunStatusCompleted || run.Final.Text != "child final" {
		t.Fatalf("run = %+v, want completed child final", run)
	}
	payload, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}
	if strings.Contains(string(payload), "summarize this file") {
		t.Fatalf("run projection leaked focused prompt: %s", string(payload))
	}
	requests := provider.Requests()
	if len(requests) != 1 {
		t.Fatalf("provider requests = %d, want one", len(requests))
	}
	if requests[0].SessionID != "agent-invocation-run" || requests[0].TurnID != run.RunID {
		t.Fatalf("provider request ids = %+v, want session and child run id", requests[0])
	}
	if _, ok := modelInputTextByKind(requests[0].InputItems, ModelInputKindConversationHistoryContext); ok {
		t.Fatalf("child request inherited parent conversation history: %+v", requests[0].InputItems)
	}
	if text, ok := modelInputTextByKind(requests[0].InputItems, ModelInputKindUserText); !ok || text != "summarize this file" {
		t.Fatalf("child input = %+v, want focused prompt", requests[0].InputItems)
	}
	if len(requests[0].ToolManifest) != 1 || requests[0].ToolManifest[0].Name != "resource_read" {
		t.Fatalf("tool manifest = %+v, want invocation-scoped resource_read only", requests[0].ToolManifest)
	}
	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	if countEvents(events, "turn.submitted") != 0 || countEvents(events, "model.final") != 0 {
		t.Fatalf("events include parent turn transcript events: %+v", events)
	}
}

func TestAgentInvocationChildConversationProjectsBoundedRun(t *testing.T) {
	provider := &recordingTextProvider{
		text:  "child final",
		usage: &TokenUsage{InputTokens: 11, OutputTokens: 3, TotalTokens: 14},
	}
	k := newAgentInvocationRunTestKernel(t, Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:   provider,
		ToolPolicy: ToolPolicy{PermissionMode: PermissionModePlan},
	})
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{
		SessionID:           "agent-invocation-child-conversation",
		Principal:           "application:test",
		AgentProfileRef:     "agent_profile:local-small-worker",
		ContextScope:        "context:diff-plus-issue",
		ParentResultChannel: "parent_result:direct",
		CapabilityGrant:     CapabilityGrant{ToolNames: []string{"resource_read"}},
	})
	if err != nil {
		t.Fatalf("AdmitAgentInvocation returned error: %v", err)
	}

	run, err := k.RunAgentInvocation(context.Background(), AgentInvocationRunRequest{
		InvocationID: invocation.InvocationID,
		Principal:    "application:test",
		InputItems:   []InputItem{{Type: "text", Text: "secret focused prompt should not project"}},
	})
	if err != nil {
		t.Fatalf("RunAgentInvocation returned error: %v", err)
	}
	if _, _, err := k.appendToolCallEvents(invocation.SessionID, run.RunID, []ModelToolCall{{
		ToolCallID: "provider_raw_tool_call",
		Name:       "resource_read",
		Arguments:  json.RawMessage(`{"ref":"raw tool trace should not project"}`),
	}}); err != nil {
		t.Fatalf("appendToolCallEvents returned error: %v", err)
	}

	conversation, err := k.AgentInvocationChildConversation(invocation.InvocationID)
	if err != nil {
		t.Fatalf("AgentInvocationChildConversation returned error: %v", err)
	}
	if conversation.InvocationID != invocation.InvocationID || conversation.RunID != run.RunID {
		t.Fatalf("conversation ids = %+v, want invocation/run ids", conversation)
	}
	if conversation.RoleID != "local-small-worker" || conversation.AgentProfileRef != invocation.AgentProfileRef {
		t.Fatalf("conversation role/profile = %+v, want local-small-worker profile", conversation)
	}
	if conversation.Status != AgentInvocationRunStatusCompleted || conversation.Final.Text != "child final" {
		t.Fatalf("conversation final = %+v, want completed child final", conversation)
	}
	if conversation.ContextScope != "context:diff-plus-issue" {
		t.Fatalf("conversation context scope = %q, want preserved scope", conversation.ContextScope)
	}
	if conversation.Usage == nil || conversation.Usage.TotalTokens != 14 {
		t.Fatalf("conversation usage = %+v, want total tokens", conversation.Usage)
	}
	if !containsString(conversation.ToolSet, "resource_read") || len(conversation.ToolSet) != 1 {
		t.Fatalf("conversation tool set = %+v, want resource_read only", conversation.ToolSet)
	}
	if !containsString(conversation.ModelInputKinds, ModelInputKindUserText) {
		t.Fatalf("conversation model input kinds = %+v, want user text", conversation.ModelInputKinds)
	}
	if len(conversation.EvidenceRefs) < 2 {
		t.Fatalf("conversation evidence refs = %+v, want admission and run refs", conversation.EvidenceRefs)
	}
	for _, ref := range conversation.EvidenceRefs {
		if !strings.HasPrefix(ref, "event:") {
			t.Fatalf("conversation evidence ref = %q, want event ref", ref)
		}
	}
	payload, err := json.Marshal(conversation)
	if err != nil {
		t.Fatalf("marshal conversation: %v", err)
	}
	for _, forbidden := range []string{
		"secret focused prompt should not project",
		"provider_raw_tool_call",
		"raw tool trace should not project",
		"tool_call_id",
		"credential",
		"sandbox",
		"permission",
	} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("conversation projection leaked %q: %s", forbidden, string(payload))
		}
	}
}

func TestAgentInvocationRunRejectsToolOutsideGrant(t *testing.T) {
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{{
			ToolCallID: "edit-outside-grant",
			Name:       "workspace_edit",
			Arguments:  json.RawMessage(`{}`),
		}},
	}
	k := newAgentInvocationRunTestKernel(t, Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:   provider,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  testTempDir(t),
		},
	})
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{
		SessionID:       "agent-invocation-run-denied",
		Principal:       "application:test",
		CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}},
	})
	if err != nil {
		t.Fatalf("AdmitAgentInvocation returned error: %v", err)
	}

	run, err := k.RunAgentInvocation(context.Background(), AgentInvocationRunRequest{
		InvocationID: invocation.InvocationID,
		Principal:    "application:test",
		InputItems:   []InputItem{{Type: "text", Text: "edit the workspace"}},
	})
	if err == nil || !strings.Contains(err.Error(), "capability_grant_tool_not_allowed") {
		t.Fatalf("RunAgentInvocation error = %v, want capability_grant_tool_not_allowed", err)
	}
	if run.Status != AgentInvocationRunStatusFailed || run.Error == nil || run.Error.Code != "tool_call_rejected" {
		t.Fatalf("run = %+v, want failed tool_call_rejected", run)
	}
	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	if countEvents(events, "agent_invocation.run_started") != 1 || countEvents(events, "agent_invocation.run_failed") != 1 {
		t.Fatalf("events did not record started+failed: %+v", events)
	}
}

func TestAgentInvocationRunIdempotencyReturnsTerminalResult(t *testing.T) {
	provider := &countingTextProvider{text: "only once"}
	k := newAgentInvocationRunTestKernel(t, Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:   provider,
		ToolPolicy: ToolPolicy{PermissionMode: PermissionModePlan},
	})
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{
		SessionID:       "agent-invocation-run-idempotent",
		Principal:       "application:test",
		CapabilityGrant: CapabilityGrant{},
	})
	if err != nil {
		t.Fatalf("AdmitAgentInvocation returned error: %v", err)
	}
	req := AgentInvocationRunRequest{
		InvocationID:   invocation.InvocationID,
		Principal:      "application:test",
		InputItems:     []InputItem{{Type: "text", Text: "do it once"}},
		IdempotencyKey: "same-run",
	}
	first, err := k.RunAgentInvocation(context.Background(), req)
	if err != nil {
		t.Fatalf("first RunAgentInvocation returned error: %v", err)
	}
	second, err := k.RunAgentInvocation(context.Background(), req)
	if err != nil {
		t.Fatalf("second RunAgentInvocation returned error: %v", err)
	}
	if second.RunID != first.RunID || second.Final.Text != first.Final.Text {
		t.Fatalf("idempotent run = %+v, want original %+v", second, first)
	}
	if provider.Calls() != 1 {
		t.Fatalf("provider calls = %d, want one", provider.Calls())
	}
	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	if countEvents(events, "agent_invocation.run_started") != 1 || countEvents(events, "agent_invocation.run_completed") != 1 {
		t.Fatalf("events did not record exactly one terminal run: %+v", events)
	}
}

func countEvents(events []StoredEvent, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func newAgentInvocationRunTestKernel(t *testing.T, config Config) *Kernel {
	t.Helper()
	config.RuntimeToken = testRuntimeToken
	config.Clock = func() time.Time {
		return time.Date(2026, 6, 22, 1, 2, 3, 0, time.UTC)
	}
	k, err := New(config)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	t.Cleanup(k.Close)
	return k
}
