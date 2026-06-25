package toolruntime

import "testing"

func TestPlanExecutionBatchesParallelizesOnlyTrustedCompatiblePureReads(t *testing.T) {
	calls := []PlannedCall{
		{Name: "resource_read", AccessPlan: AccessPlan{
			EffectClass:    EffectClassPureRead,
			ParallelPolicy: ParallelPolicyCompatibleLocks,
			ResourceFootprint: ResourceFootprint{
				ReadScopes: []string{"resource:a"},
			},
			Trusted: true,
		}},
		{Name: "resource_read", AccessPlan: AccessPlan{
			EffectClass:    EffectClassPureRead,
			ParallelPolicy: ParallelPolicyCompatibleLocks,
			ResourceFootprint: ResourceFootprint{
				ReadScopes: []string{"resource:b"},
			},
			Trusted: true,
		}},
	}

	batches := PlanExecutionBatches(calls)

	if len(batches) != 1 || !batches[0].Parallel || batches[0].Reason != EffectClassPureRead {
		t.Fatalf("batches = %+v, want one parallel pure-read batch", batches)
	}
}

func TestPlanExecutionBatchesSerializesWritesAndExternalSideEffects(t *testing.T) {
	calls := []PlannedCall{
		{Name: "shell_exec", AccessPlan: AccessPlan{
			EffectClass:    EffectClassWorkspaceWrite,
			ParallelPolicy: ParallelPolicySerialFence,
			Trusted:        true,
		}},
		{Name: "connector_send", AccessPlan: AccessPlan{
			EffectClass:    EffectClassExternalSideEffect,
			ParallelPolicy: ParallelPolicySerialFence,
			Trusted:        true,
		}},
	}

	batches := PlanExecutionBatches(calls)

	if len(batches) != 2 {
		t.Fatalf("batches = %+v, want two serial batches", batches)
	}
	if batches[0].Parallel || batches[0].Reason != "write_effect_serial_fence" {
		t.Fatalf("write batch = %+v, want serial write fence", batches[0])
	}
	if batches[1].Parallel || batches[1].Reason != "external_side_effect_routes_through_owner" {
		t.Fatalf("external batch = %+v, want serial external side-effect owner route", batches[1])
	}
}

func TestPlanExecutionBatchesKeepsSameProcessHandleSerial(t *testing.T) {
	calls := []PlannedCall{
		{Name: "job_status", AccessPlan: AccessPlan{
			EffectClass:    EffectClassProcessIO,
			ParallelPolicy: ParallelPolicyPerHandleSerial,
			ResourceFootprint: ResourceFootprint{
				Handles: []string{"job:one"},
			},
			Trusted: true,
		}},
		{Name: "job_status", AccessPlan: AccessPlan{
			EffectClass:    EffectClassProcessIO,
			ParallelPolicy: ParallelPolicyPerHandleSerial,
			ResourceFootprint: ResourceFootprint{
				Handles: []string{"job:one"},
			},
			Trusted: true,
		}},
	}

	batches := PlanExecutionBatches(calls)

	if len(batches) != 2 {
		t.Fatalf("batches = %+v, want same handle in separate batches", batches)
	}
	for _, batch := range batches {
		if batch.Parallel || batch.Reason != EffectClassProcessIO {
			t.Fatalf("process IO batch = %+v, want serial process IO batch", batch)
		}
	}
}

func TestAccessPlanRejectsUntrustedParallelMetadata(t *testing.T) {
	plan := AccessPlan{
		EffectClass:    EffectClassPureRead,
		ParallelPolicy: ParallelPolicyCompatibleLocks,
		Trusted:        false,
	}

	if class := plan.ParallelClass(); class != "" {
		t.Fatalf("ParallelClass = %q, want empty for untrusted metadata", class)
	}
	if reason := plan.SerialReason("resource_read"); reason != "missing_or_untrusted_tool_access_plan" {
		t.Fatalf("SerialReason = %q, want untrusted metadata reason", reason)
	}
}
