package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
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
	_, err = k.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{
		ConfigRoot: configRoot, SessionID: "worker-role-session", Principal: "application:test", RoleID: "local-small-worker", IdempotencyKey: "worker-3",
	})
	if err == nil || !strings.Contains(err.Error(), "parallel_limit_exceeded") {
		t.Fatalf("third worker error = %v, want worker role concurrency limit", err)
	}
}

func TestAdmitWorkerInvocationFromRoleDefaultsRoleConcurrencyToSix(t *testing.T) {
	configRoot := writeParentWorkerRuntimeConfigWithLimits(t, []string{"resource_read"}, 0, 0)
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModePlan,
	})

	for index := 0; index < 6; index++ {
		_, err := k.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{
			ConfigRoot:     configRoot,
			SessionID:      "worker-default-role-limit",
			Principal:      "application:test",
			RoleID:         "local-small-worker",
			IdempotencyKey: fmt.Sprintf("worker-%d", index),
		})
		if err != nil {
			t.Fatalf("worker %d admission returned error: %v", index+1, err)
		}
	}
	_, err := k.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{
		ConfigRoot:     configRoot,
		SessionID:      "worker-default-role-limit",
		Principal:      "application:test",
		RoleID:         "local-small-worker",
		IdempotencyKey: "worker-7",
	})
	if err == nil || !strings.Contains(err.Error(), "parallel_limit_exceeded") {
		t.Fatalf("seventh worker error = %v, want role parallel limit", err)
	}
}

func TestAdmitWorkerInvocationFromRoleDefaultsParentChildLimitToTwentyFour(t *testing.T) {
	configRoot := writeParentWorkerRuntimeConfigWithLimits(t, []string{"resource_read"}, 30, 0)
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModePlan,
	})

	for index := 0; index < 24; index++ {
		_, err := k.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{
			ConfigRoot:     configRoot,
			SessionID:      "worker-default-parent-limit",
			Principal:      "application:test",
			RoleID:         "local-small-worker",
			IdempotencyKey: fmt.Sprintf("worker-%d", index),
		})
		if err != nil {
			t.Fatalf("worker %d admission returned error: %v", index+1, err)
		}
	}
	_, err := k.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{
		ConfigRoot:     configRoot,
		SessionID:      "worker-default-parent-limit",
		Principal:      "application:test",
		RoleID:         "local-small-worker",
		IdempotencyKey: "worker-25",
	})
	if err == nil || !strings.Contains(err.Error(), "parallel_limit_exceeded") || !strings.Contains(err.Error(), "parent_max_children") {
		t.Fatalf("twenty-fifth child error = %v, want parent child limit", err)
	}
}

func TestAdmitWorkerInvocationFromRoleEnforcesProfileAndRouteParallelLimits(t *testing.T) {
	for _, tc := range []struct {
		name         string
		profileLimit int
		routeLimit   int
		want         string
	}{
		{name: "profile", profileLimit: 1, want: "model_profile"},
		{name: "route", profileLimit: 6, routeLimit: 1, want: "provider_route"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configRoot := writeParentWorkerRuntimeConfigWithAllLimits(t, []string{"resource_read"}, 6, 0, tc.profileLimit, tc.routeLimit)
			k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{PermissionMode: PermissionModePlan})
			for index := 1; index <= 2; index++ {
				_, err := k.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{ConfigRoot: configRoot, SessionID: "worker-" + tc.name, Principal: "application:test", RoleID: "local-small-worker", IdempotencyKey: fmt.Sprintf("worker-%d", index)})
				if index == 1 && err != nil {
					t.Fatalf("first admission returned error: %v", err)
				}
				if index == 2 && (err == nil || !strings.Contains(err.Error(), "parallel_limit_exceeded") || !strings.Contains(err.Error(), tc.want)) {
					t.Fatalf("second admission error = %v, want %s parallel limit", err, tc.want)
				}
			}
		})
	}
}

func TestAdmitWorkerInvocationFromRoleEnforcesParentChildLimitAcrossRoles(t *testing.T) {
	configRoot := writeParentWorkerRuntimeConfigWithRoles(t, 2, map[string]int{
		"reader":   6,
		"reviewer": 6,
	})
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModePlan,
	})
	for index, roleID := range []string{"reader", "reviewer"} {
		_, err := k.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{
			ConfigRoot:     configRoot,
			SessionID:      "worker-parent-limit",
			Principal:      "application:test",
			RoleID:         roleID,
			IdempotencyKey: fmt.Sprintf("worker-%d", index+1),
		})
		if err != nil {
			t.Fatalf("%s worker admission returned error: %v", roleID, err)
		}
	}
	_, err := k.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{
		ConfigRoot:     configRoot,
		SessionID:      "worker-parent-limit",
		Principal:      "application:test",
		RoleID:         "reader",
		IdempotencyKey: "worker-3",
	})
	if err == nil || !strings.Contains(err.Error(), "parallel_limit_exceeded") || !strings.Contains(err.Error(), "parent_max_children") {
		t.Fatalf("third child error = %v, want parent child limit", err)
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

func TestResolveParentWorkerRuntimeRejectsRecursiveDelegateWorkerTool(t *testing.T) {
	configRoot := writeParentWorkerRuntimeConfig(t, []string{"delegate_worker"})
	_, err := ResolveParentWorkerRuntimeFromGenesis(ParentWorkerRuntimeRequest{ConfigRoot: configRoot})
	if !errors.Is(err, ErrGenesisWorkerRoleBindingInvalid) || !strings.Contains(err.Error(), "worker_role_must_be_leaf") {
		t.Fatalf("ResolveParentWorkerRuntimeFromGenesis error = %v, want leaf-only delegate_worker rejection", err)
	}
}

func TestModelToolRoundsUsesTerminalDelegateWorkerResultAfterQueuedReceipt(t *testing.T) {
	events := []StoredEvent{
		{EventID: "evt_call", TurnID: "turn_parent", Type: "tool.call", Data: EventData{ToolCall: &ToolCallProjection{ToolCallEventID: "evt_call", ProviderToolCallID: "provider_call", Tool: "delegate_worker", Arguments: `{"role_id":"reviewer","task":"review"}`}}},
		{EventID: "evt_queued", TurnID: "turn_parent", Type: "tool.result", Data: EventData{ToolResult: &ToolResultProjection{ForEventID: "evt_call", ProviderToolCallID: "provider_call", Tool: "delegate_worker", Status: "queued", Content: `{"status":"queued"}`}}},
		{EventID: "evt_terminal", TurnID: "turn_parent", Type: "tool.result", Data: EventData{ToolResult: &ToolResultProjection{ForEventID: "evt_call", ProviderToolCallID: "provider_call", Tool: "delegate_worker", Status: "completed", Content: `{"status":"completed","final":"review accepted"}`}}},
	}
	rounds := modelToolRoundsFromStoredEvents(events, "turn_parent")
	if len(rounds) != 1 || len(rounds[0].Calls) != 1 || len(rounds[0].Results) != 1 {
		t.Fatalf("tool rounds = %+v, want one resolved delegation round", rounds)
	}
	if rounds[0].Results[0].Content != `{"status":"completed","final":"review accepted"}` {
		t.Fatalf("delegation result = %s, want terminal result", rounds[0].Results[0].Content)
	}
}

func TestNewRestartsQueuedDelegatedWorkerWithoutReplayingTools(t *testing.T) {
	configRoot := writeParentWorkerRuntimeConfig(t, []string{"resource_read"})
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	first := newAgentInvocationRunTestKernel(t, Config{
		LedgerPath:             ledgerPath,
		ToolPolicy:             ToolPolicy{PermissionMode: PermissionModePlan},
		ParentWorkerConfigRoot: configRoot,
	})
	invocation, err := first.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{
		ConfigRoot:     configRoot,
		SessionID:      "restart-delegation",
		ParentTurnID:   "turn_parent",
		Principal:      "application:kernel",
		RoleID:         "local-small-worker",
		IdempotencyKey: "evt_delegate",
	})
	if err != nil {
		t.Fatalf("AdmitWorkerInvocationFromRole returned error: %v", err)
	}
	if err := first.appendEvent(StoredEvent{EventID: "evt_delegate", SessionID: invocation.SessionID, TurnID: invocation.ParentTurnID, Type: "tool.call", Data: EventData{ToolCall: &ToolCallProjection{ToolCallEventID: "evt_delegate", ProviderToolCallID: "provider_delegate", Tool: "delegate_worker", Arguments: `{"role_id":"local-small-worker","task":"recover this task"}`}}}); err != nil {
		t.Fatalf("append delegate tool call: %v", err)
	}
	first.Close()

	child := &delegateWorkerChildProvider{completed: make(chan ModelRequest, 1)}
	_ = newAgentInvocationRunTestKernel(t, Config{
		LedgerPath:             ledgerPath,
		ToolPolicy:             ToolPolicy{PermissionMode: PermissionModePlan},
		ParentWorkerConfigRoot: configRoot,
		WorkerProviderResolver: func(profileID string) (Provider, error) { return child, nil },
	})
	select {
	case request := <-child.completed:
		if len(request.InputItems) != 1 || request.InputItems[0].Text != "recover this task" {
			t.Fatalf("recovered worker input = %+v, want focused task", request.InputItems)
		}
	case <-time.After(time.Second):
		t.Fatal("queued worker did not restart")
	}
}

func TestNewFailsStartedDelegatedWorkerInsteadOfReplayingIt(t *testing.T) {
	configRoot := writeParentWorkerRuntimeConfig(t, []string{"resource_read"})
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	first := newAgentInvocationRunTestKernel(t, Config{LedgerPath: ledgerPath, ToolPolicy: ToolPolicy{PermissionMode: PermissionModePlan}, ParentWorkerConfigRoot: configRoot})
	invocation, err := first.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{ConfigRoot: configRoot, SessionID: "restart-started", ParentTurnID: "turn_parent", Principal: "application:kernel", RoleID: "local-small-worker", IdempotencyKey: "evt_delegate"})
	if err != nil {
		t.Fatalf("AdmitWorkerInvocationFromRole returned error: %v", err)
	}
	run := AgentInvocationRunProjection{InvocationID: invocation.InvocationID, RunID: "agent_run_started", SessionID: invocation.SessionID, Principal: "application:kernel", Status: AgentInvocationRunStatusRunning, IdempotencyKey: invocation.IdempotencyKey, StartedAt: time.Now().UTC()}
	if err := first.appendAgentInvocationRunEvent("agent_invocation.run_started", run); err != nil {
		t.Fatalf("append run started: %v", err)
	}
	first.Close()

	child := &delegateWorkerChildProvider{completed: make(chan ModelRequest, 1)}
	restarted := newAgentInvocationRunTestKernel(t, Config{LedgerPath: ledgerPath, ToolPolicy: ToolPolicy{PermissionMode: PermissionModePlan}, ParentWorkerConfigRoot: configRoot, WorkerProviderResolver: func(string) (Provider, error) { return child, nil }})
	runs, err := restarted.agentInvocationRuns()
	if err != nil {
		t.Fatalf("agent invocation runs: %v", err)
	}
	last, ok := runs[run.RunID]
	if !ok || last.Status != AgentInvocationRunStatusFailed || last.Error == nil || last.Error.Code != "worker_delegation_recovery_ambiguous" {
		t.Fatalf("recovered run = %+v, want ambiguous-recovery failure", last)
	}
	select {
	case <-child.completed:
		t.Fatal("started worker must not replay")
	default:
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
	return writeParentWorkerRuntimeConfigWithLimits(t, toolSet, 2, 0)
}

func writeParentWorkerRuntimeConfigWithLimits(t *testing.T, toolSet []string, maxParallel int, maxChildren int) string {
	return writeParentWorkerRuntimeConfigWithAllLimits(t, toolSet, maxParallel, maxChildren, 0, 0)
}

func writeParentWorkerRuntimeConfigWithAllLimits(t *testing.T, toolSet []string, maxParallel int, maxChildren int, profileMaxParallel int, routeMaxParallel int) string {
	t.Helper()
	tools := make([]any, 0, len(toolSet))
	for _, tool := range toolSet {
		tools = append(tools, tool)
	}
	parent := map[string]any{
		"allowed_worker_roles": []any{"local-small-worker"},
		"default_worker_role":  "local-small-worker",
		"can_create_workers":   true,
	}
	if maxChildren > 0 {
		parent["max_children"] = maxChildren
	}
	worker := map[string]any{
		"profile_id": "worker-profile",
		"tool_set":   tools,
		"leaf_only":  true,
	}
	if maxParallel > 0 {
		worker["max_parallel"] = maxParallel
	}
	profile := map[string]any{"profile_id": "worker-profile", "model_id": "worker-model", "gateway_route": "worker-route"}
	if profileMaxParallel > 0 {
		profile["max_parallel"] = profileMaxParallel
	}
	route := map[string]any{}
	if routeMaxParallel > 0 {
		route["max_parallel"] = routeMaxParallel
	}
	return writeParentWorkerRuntimeConfigWithBindings(t, parent, map[string]any{
		"local-small-worker": worker,
	}, profile, route)
}

func writeParentWorkerRuntimeConfigWithRoles(t *testing.T, maxChildren int, roleLimits map[string]int) string {
	t.Helper()
	roles := make([]any, 0, len(roleLimits))
	workers := make(map[string]any, len(roleLimits))
	for roleID, maxParallel := range roleLimits {
		roles = append(roles, roleID)
		workers[roleID] = map[string]any{
			"profile_id":   "worker-profile",
			"tool_set":     []any{"resource_read"},
			"max_parallel": maxParallel,
			"leaf_only":    true,
		}
	}
	return writeParentWorkerRuntimeConfigWithBindings(t, map[string]any{
		"allowed_worker_roles": roles,
		"can_create_workers":   true,
		"max_children":         maxChildren,
	}, workers, map[string]any{"profile_id": "worker-profile", "model_id": "worker-model", "gateway_route": "worker-route"}, map[string]any{})
}

func writeParentWorkerRuntimeConfigWithBindings(t *testing.T, parent map[string]any, workers map[string]any, profile map[string]any, route map[string]any) string {
	t.Helper()
	return writeModelsConfig(t, map[string]any{
		"model_gateway": map[string]any{
			"protocol":       "openai-chat-completions",
			"base_url":       "https://provider.example.com/api",
			"credential_ref": "secret://models/provider/default",
			"routes":         map[string]any{"worker-route": route},
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
					"worker-profile": profile,
				},
			},
		},
		"parent_worker_runtime": map[string]any{
			"parents": map[string]any{
				DefaultModelRole: parent,
			},
			"worker_roles": workers,
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

func TestSubmitTurnDelegateWorkerPausesParentAndRunsRoleBoundLeaf(t *testing.T) {
	configRoot := writeParentWorkerRuntimeConfig(t, []string{"resource_read"})
	parent := &delegateWorkerParentProvider{finalized: make(chan struct{}, 1)}
	child := &delegateWorkerChildProvider{completed: make(chan ModelRequest, 1)}
	resolvedProfileID := ""
	k := newAgentInvocationRunTestKernel(t, Config{
		LedgerPath:             filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:               parent,
		ToolPolicy:             ToolPolicy{PermissionMode: PermissionModePlan},
		ParentWorkerConfigRoot: configRoot,
		WorkerProviderResolver: func(profileID string) (Provider, error) {
			resolvedProfileID = profileID
			return child, nil
		},
	})

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "delegate-worker-parent",
		InputItems: []InputItem{{Type: "text", Text: "inspect the repository"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Pause == nil || resp.Pause.WaitReason != WaitReasonAgentDelegation {
		receipts := []string{}
		for _, event := range resp.Events {
			if data, ok := event.Data.(EventData); ok && data.ToolResult != nil {
				receipts = append(receipts, data.ToolResult.Content)
			}
		}
		t.Fatalf("response pause = %+v receipts = %v, want agent delegation wait", resp.Pause, receipts)
	}
	if parent.CallCount() != 1 {
		t.Fatalf("parent provider calls = %d, want one before pause", parent.CallCount())
	}

	var childRequest ModelRequest
	select {
	case childRequest = <-child.completed:
	case <-time.After(time.Second):
		t.Fatal("worker did not complete")
	}
	if resolvedProfileID != "worker-profile" {
		t.Fatalf("resolved profile = %q, want worker-profile", resolvedProfileID)
	}
	if len(childRequest.InputItems) != 1 || childRequest.InputItems[0].Text != "inspect the repository" {
		t.Fatalf("worker inputs = %+v, want focused task only", childRequest.InputItems)
	}
	for _, tool := range childRequest.ToolManifest {
		if tool.Name == "delegate_worker" {
			t.Fatalf("worker manifest = %+v, must not contain delegate_worker", childRequest.ToolManifest)
		}
	}

	invocations, err := k.AgentInvocations("delegate-worker-parent")
	if err != nil {
		t.Fatalf("AgentInvocations returned error: %v", err)
	}
	if len(invocations) != 1 || invocations[0].ParentTurnID != resp.TurnID {
		t.Fatalf("invocations = %+v, want one worker bound to parent turn %q", invocations, resp.TurnID)
	}
	conversation, err := k.AgentInvocationChildConversation(invocations[0].InvocationID)
	if err != nil {
		t.Fatalf("AgentInvocationChildConversation returned error: %v", err)
	}
	if conversation.Status != AgentInvocationRunStatusCompleted || conversation.Final.Text != "worker final" {
		t.Fatalf("child conversation = %+v, want bounded completed final", conversation)
	}
	invocations, err = k.AgentInvocations("delegate-worker-parent")
	if err != nil || len(invocations) != 1 || invocations[0].Status != AgentInvocationRunStatusCompleted {
		t.Fatalf("worker list = %+v error = %v, want completed status", invocations, err)
	}
	select {
	case <-parent.finalized:
	case <-time.After(time.Second):
		t.Fatal("parent did not continue after worker terminal result")
	}
	deadline := time.After(time.Second)
	for {
		parentEvents, err := k.TurnEvents(resp.TurnID)
		if err != nil {
			t.Fatalf("TurnEvents returned error: %v", err)
		}
		if len(parentEvents) > 0 && parentEvents[len(parentEvents)-1].Type == "model.final" {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("parent events = %+v, want parent final after terminal worker result", parentEvents)
		case <-time.After(time.Millisecond):
		}
	}

	for _, event := range resp.Events {
		data, ok := event.Data.(EventData)
		if event.Type != "tool.result" || !ok || data.ToolResult == nil || data.ToolResult.Tool != "delegate_worker" {
			continue
		}
		if strings.Contains(data.ToolResult.Content, "inspect the repository") {
			t.Fatalf("delegate receipt leaked focused task: %s", data.ToolResult.Content)
		}
		return
	}
	t.Fatalf("parent events = %+v, want delegate_worker tool receipt", resp.Events)
}

type delegateWorkerParentProvider struct {
	mu        sync.Mutex
	requests  []ModelRequest
	finalized chan struct{}
}

func (p *delegateWorkerParentProvider) Name() string { return "delegate-worker-parent" }

func (p *delegateWorkerParentProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *delegateWorkerParentProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, cloneModelRequest(req))
	if len(req.ToolRounds) > 0 {
		if p.finalized != nil {
			p.finalized <- struct{}{}
		}
		return ModelResponse{Text: "parent reduced final", Model: "parent-model"}, nil
	}
	return ModelResponse{Model: "parent-model", ToolCalls: []ModelToolCall{{
		ToolCallID: "delegate_worker_call",
		Name:       "delegate_worker",
		Arguments:  json.RawMessage(`{"role_id":"local-small-worker","task":"inspect the repository"}`),
	}}}, nil
}

func (p *delegateWorkerParentProvider) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.requests)
}

type delegateWorkerChildProvider struct {
	completed chan ModelRequest
}

func (p *delegateWorkerChildProvider) Name() string { return "delegate-worker-child" }

func (p *delegateWorkerChildProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *delegateWorkerChildProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	p.completed <- cloneModelRequest(req)
	return ModelResponse{Text: "worker final", Model: "worker-model"}, nil
}
