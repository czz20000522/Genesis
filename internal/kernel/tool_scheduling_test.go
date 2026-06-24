package kernel

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"genesis/internal/testsupport"
)

func TestPlanToolExecutionBatchesGroupsCompatiblePureReads(t *testing.T) {
	calls := []preparedModelToolCall{
		scheduledTestCall("read_a", pureReadAccessPlan("read_a")),
		scheduledTestCall("read_b", pureReadAccessPlan("read_b")),
		scheduledTestCall("read_c", pureReadAccessPlan("read_c")),
	}
	batches := planToolExecutionBatches(calls)
	assertToolBatchShape(t, batches, [][]int{{0, 1, 2}}, []bool{true})
}

func TestPlanToolExecutionBatchesKeepsWriteFences(t *testing.T) {
	calls := []preparedModelToolCall{
		scheduledTestCall("read_before", pureReadAccessPlan("read_before")),
		scheduledTestCall("write", ToolAccessPlan{
			ToolName:       "write",
			EffectClass:    ToolEffectClassWorkspaceWrite,
			ParallelPolicy: ToolParallelPolicySerialFence,
			Trusted:        true,
		}),
		scheduledTestCall("read_after", pureReadAccessPlan("read_after")),
	}
	batches := planToolExecutionBatches(calls)
	assertToolBatchShape(t, batches, [][]int{{0}, {1}, {2}}, []bool{false, false, false})
}

func TestPlanToolExecutionBatchesSerializesUnknownAndStateReads(t *testing.T) {
	calls := []preparedModelToolCall{
		scheduledTestCall("unknown", ToolAccessPlan{}),
		scheduledTestCall("state_read", ToolAccessPlan{
			ToolName:       "state_read",
			EffectClass:    ToolEffectClassStateRead,
			ParallelPolicy: ToolParallelPolicyCompatibleLocks,
			Trusted:        true,
		}),
		scheduledTestCall("known_read", pureReadAccessPlan("known_read")),
	}
	batches := planToolExecutionBatches(calls)
	assertToolBatchShape(t, batches, [][]int{{0}, {1}, {2}}, []bool{false, false, false})
	if !strings.Contains(batches[0].Reason, "missing_or_untrusted") {
		t.Fatalf("unknown batch reason = %q, want untrusted access plan", batches[0].Reason)
	}
	if batches[1].Reason != "state_read_waits_for_prior_committed_facts" {
		t.Fatalf("state read reason = %q", batches[1].Reason)
	}
}

func TestPlanToolExecutionBatchesSerializesUntrustedSchedulingMetadata(t *testing.T) {
	calls := []preparedModelToolCall{
		scheduledTestCall("untrusted_read", ToolAccessPlan{
			ToolName:       "untrusted_read",
			EffectClass:    ToolEffectClassPureRead,
			ParallelPolicy: ToolParallelPolicyCompatibleLocks,
			Trusted:        false,
		}),
		scheduledTestCall("trusted_read", pureReadAccessPlan("trusted_read")),
	}
	batches := planToolExecutionBatches(calls)
	assertToolBatchShape(t, batches, [][]int{{0}, {1}}, []bool{false, false})
	if batches[0].Reason != "missing_or_untrusted_tool_access_plan" {
		t.Fatalf("untrusted batch reason = %q", batches[0].Reason)
	}
}

func TestPlanToolExecutionBatchesSerializesKernelStateWrites(t *testing.T) {
	calls := []preparedModelToolCall{
		scheduledTestCall("read_before", pureReadAccessPlan("read_before")),
		scheduledTestCall("memory_approve", ToolAccessPlan{
			ToolName:       "memory_approve",
			EffectClass:    ToolEffectClassKernelStateWrite,
			ParallelPolicy: ToolParallelPolicySerialFence,
			Trusted:        true,
		}),
		scheduledTestCall("read_after", pureReadAccessPlan("read_after")),
	}
	batches := planToolExecutionBatches(calls)
	assertToolBatchShape(t, batches, [][]int{{0}, {1}, {2}}, []bool{false, false, false})
	if batches[1].Reason != "write_effect_serial_fence" {
		t.Fatalf("kernel state write reason = %q", batches[1].Reason)
	}
}

func TestPlanToolExecutionBatchesSerializesExternalSideEffectsThroughOwner(t *testing.T) {
	calls := []preparedModelToolCall{
		scheduledTestCall("read_before", pureReadAccessPlan("read_before")),
		scheduledTestCall("send_message", ToolAccessPlan{
			ToolName:       "send_message",
			EffectClass:    ToolEffectClassExternalSideEffect,
			ParallelPolicy: ToolParallelPolicySerialFence,
			Trusted:        true,
			ResourceFootprint: ToolResourceFootprint{
				ExternalTargets: []string{"connector:feishu:chat:oc_123"},
			},
		}),
		scheduledTestCall("read_after", pureReadAccessPlan("read_after")),
	}
	batches := planToolExecutionBatches(calls)
	assertToolBatchShape(t, batches, [][]int{{0}, {1}, {2}}, []bool{false, false, false})
	if batches[1].Reason != "external_side_effect_routes_through_owner" {
		t.Fatalf("external side effect reason = %q", batches[1].Reason)
	}
}

func TestPlanToolExecutionBatchesSerializesSameHandleProcessIO(t *testing.T) {
	calls := []preparedModelToolCall{
		scheduledTestCall("job_status_a", jobControlToolAccessPlan("job_status", "job_a")),
		scheduledTestCall("job_status_b", jobControlToolAccessPlan("job_status", "job_b")),
		scheduledTestCall("job_cancel_a", jobControlToolAccessPlan("job_cancel", "job_a")),
	}
	batches := planToolExecutionBatches(calls)
	assertToolBatchShape(t, batches, [][]int{{0, 1}, {2}}, []bool{true, false})
}

func TestPlanToolExecutionBatchesKeepsProcessStartAdmissionSerial(t *testing.T) {
	calls := []preparedModelToolCall{
		scheduledTestCall("managed_shell", shellExecToolAccessPlan("shell_exec", ".", maxForegroundShellTimeoutSec+1)),
		scheduledTestCall("read_after", pureReadAccessPlan("read_after")),
	}
	batches := planToolExecutionBatches(calls)
	assertToolBatchShape(t, batches, [][]int{{0}, {1}}, []bool{false, false})
	if batches[0].Reason != "process_start_serial_admission" {
		t.Fatalf("process start reason = %q", batches[0].Reason)
	}
}

func TestPrepareBatchAssignsDefaultToolAccessPlans(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "tool-scheduling-prepare")
	k := newTestKernel(t, filepath.Join(dir, "events.jsonl"))
	shellArgs, err := json.Marshal(map[string]interface{}{
		"command": "echo schedule",
		"cwd":     dir,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	statusArgs, err := json.Marshal(map[string]interface{}{
		"job_id": "job_schedule",
	})
	if err != nil {
		t.Fatalf("marshal status args: %v", err)
	}
	cancelArgs, err := json.Marshal(map[string]interface{}{
		"job_id": "job_schedule",
		"reason": "stop",
	})
	if err != nil {
		t.Fatalf("marshal cancel args: %v", err)
	}
	resourceArgs, err := json.Marshal(map[string]interface{}{
		"resource_ref": "res_schedule",
	})
	if err != nil {
		t.Fatalf("marshal resource args: %v", err)
	}
	k = newTestKernelWithResources(t, filepath.Join(dir, "resource-events.jsonl"), []ResourceDescriptor{{
		Ref:      "res_schedule",
		MimeType: "text/plain",
		Text:     "schedule",
	}})
	prepared, err := k.toolGateway().PrepareBatch([]ModelToolCall{
		{ToolCallID: "call_shell", ToolCallEventID: "evt_tool_shell", Name: "shell_exec", Arguments: shellArgs},
		{ToolCallID: "call_status", ToolCallEventID: "evt_tool_status", Name: "job_status", Arguments: statusArgs},
		{ToolCallID: "call_cancel", ToolCallEventID: "evt_tool_cancel", Name: "job_cancel", Arguments: cancelArgs},
		{ToolCallID: "call_resource", ToolCallEventID: "evt_tool_resource", Name: "resource_read", Arguments: resourceArgs},
	})
	if err != nil {
		t.Fatalf("PrepareBatch returned error: %v", err)
	}
	if prepared[0].accessPlan.EffectClass != ToolEffectClassWorkspaceWrite || prepared[0].accessPlan.parallelClass() != "" {
		t.Fatalf("shell access plan = %+v, want effectful serial plan", prepared[0].accessPlan)
	}
	if prepared[1].accessPlan.EffectClass != ToolEffectClassProcessIO || prepared[1].accessPlan.ResourceFootprint.Handles[0] != "job:job_schedule" {
		t.Fatalf("job_status access plan = %+v", prepared[1].accessPlan)
	}
	if prepared[3].accessPlan.EffectClass != ToolEffectClassPureRead || prepared[3].accessPlan.parallelClass() != ToolEffectClassPureRead {
		t.Fatalf("resource_read access plan = %+v, want trusted pure-read plan", prepared[3].accessPlan)
	}
	batches := planToolExecutionBatches(prepared)
	assertToolBatchShape(t, batches, [][]int{{0}, {1}, {2}, {3}}, []bool{false, false, false, false})
}

func TestToolSchedulingMetadataStaysOutOfModelVisibleManifest(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testsupport.ProjectTempDir(t, "tool-scheduling-manifest"), "events.jsonl"))
	payload, err := json.Marshal(k.toolGateway().ToolManifest())
	if err != nil {
		t.Fatalf("marshal tool manifest: %v", err)
	}
	for _, forbidden := range []string{"effect_class", "resource_footprint", "parallel_policy", "workspace_write", "process_io"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("model-visible manifest leaked scheduling metadata %q: %s", forbidden, string(payload))
		}
	}
}

func TestDefaultKernelPureReadCandidateIsResourceReadOnly(t *testing.T) {
	pureReadTools := []string{}
	for _, tool := range defaultKernelTools() {
		spec := tool.Spec.Scheduling
		if tool.Spec.Name == "shell_exec" && spec.EffectClass == ToolEffectClassPureRead {
			t.Fatalf("shell_exec scheduling = %+v, must not become pure-read by command text", spec)
		}
		if spec.EffectClass == ToolEffectClassPureRead && spec.ParallelPolicy == ToolParallelPolicyCompatibleLocks {
			pureReadTools = append(pureReadTools, tool.Spec.Name)
		}
	}
	if strings.Join(pureReadTools, ",") != "resource_read" {
		t.Fatalf("default pure-read candidates = %v, want only resource_read", pureReadTools)
	}
}

func TestNonIdempotentEffectClassesDoNotEnterParallelClass(t *testing.T) {
	for _, effectClass := range []string{
		ToolEffectClassWorkspaceWrite,
		ToolEffectClassKernelStateWrite,
		ToolEffectClassProcessStart,
		ToolEffectClassExternalSideEffect,
	} {
		plan := ToolAccessPlan{
			ToolName:       "future_effectful_tool",
			EffectClass:    effectClass,
			ParallelPolicy: ToolParallelPolicyCompatibleLocks,
			Trusted:        true,
		}
		if got := plan.parallelClass(); got != "" {
			t.Fatalf("%s parallel class = %q, want serial until replay/idempotency contract is proven", effectClass, got)
		}
	}
}

func scheduledTestCall(name string, plan ToolAccessPlan) preparedModelToolCall {
	return preparedModelToolCall{name: name, accessPlan: plan}
}

func pureReadAccessPlan(name string) ToolAccessPlan {
	return ToolAccessPlan{
		ToolName:       name,
		EffectClass:    ToolEffectClassPureRead,
		ParallelPolicy: ToolParallelPolicyCompatibleLocks,
		Trusted:        true,
	}
}

func assertToolBatchShape(t *testing.T, batches []ToolExecutionBatch, wantIndexes [][]int, wantParallel []bool) {
	t.Helper()
	gotIndexes := make([][]int, 0, len(batches))
	gotParallel := make([]bool, 0, len(batches))
	for _, batch := range batches {
		gotIndexes = append(gotIndexes, append([]int(nil), batch.CallIndexes...))
		gotParallel = append(gotParallel, batch.Parallel)
	}
	if !reflect.DeepEqual(gotIndexes, wantIndexes) {
		t.Fatalf("batch indexes = %+v, want %+v", gotIndexes, wantIndexes)
	}
	if !reflect.DeepEqual(gotParallel, wantParallel) {
		t.Fatalf("batch parallel flags = %+v, want %+v", gotParallel, wantParallel)
	}
}
