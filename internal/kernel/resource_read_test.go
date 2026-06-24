package kernel

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"genesis/internal/testsupport"
)

func TestResourceReadReturnsBoundedText(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "resource-read")
	k := newTestKernelWithResources(t, filepath.Join(dir, "events.jsonl"), []ResourceDescriptor{
		{
			Ref:      "res_alpha",
			MimeType: "text/plain",
			Text:     "abcdef",
		},
	})
	args := mustMarshalToolArgs(t, map[string]interface{}{
		"resource_ref": "res_alpha",
		"offset_bytes": 1,
		"limit_bytes":  3,
	})

	prepared, err := k.toolGateway().PrepareBatch([]ModelToolCall{{
		ToolCallID:      "call_resource",
		ToolCallEventID: "evt_tool_resource",
		Name:            "resource_read",
		Arguments:       args,
	}})
	if err != nil {
		t.Fatalf("PrepareBatch returned error: %v", err)
	}
	result, err := k.toolGateway().Execute(context.Background(), "session_resource", "turn_resource", prepared[0])
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	var payload ModelResourceReadResult
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("unmarshal resource result: %v\n%s", err, result.Content)
	}
	if payload.Status != "completed" || !payload.Executed {
		t.Fatalf("resource result status = %+v, want completed executed", payload)
	}
	if payload.ResourceRef != "res_alpha" || payload.MimeType != "text/plain" || payload.Text != "bcd" {
		t.Fatalf("resource result = %+v, want bounded slice from res_alpha", payload)
	}
	if !payload.Truncated || payload.OriginalBytes != 6 || payload.ReturnedBytes != 3 || payload.OffsetBytes != 1 || payload.NextOffsetBytes == nil || *payload.NextOffsetBytes != 4 {
		t.Fatalf("resource truncation metadata = %+v", payload)
	}
}

func TestResourceReadUnknownRefReturnsRepairFeedback(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "resource-read-unknown")
	k := newTestKernelWithResources(t, filepath.Join(dir, "events.jsonl"), []ResourceDescriptor{{
		Ref:      "res_known",
		MimeType: "text/plain",
		Text:     "known",
	}})
	args := mustMarshalToolArgs(t, map[string]interface{}{
		"resource_ref": "res_missing",
	})

	prepared, err := k.toolGateway().PrepareBatch([]ModelToolCall{{
		ToolCallID:      "call_resource_unknown",
		ToolCallEventID: "evt_tool_resource_unknown",
		Name:            "resource_read",
		Arguments:       args,
	}})
	if err != nil {
		t.Fatalf("PrepareBatch returned error: %v", err)
	}
	result, err := k.toolGateway().Execute(context.Background(), "session_resource", "turn_resource", prepared[0])
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	var payload ToolRequestInvalidProjection
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("unmarshal invalid result: %v\n%s", err, result.Content)
	}
	if payload.Status != "tool_request_invalid" || payload.Executed {
		t.Fatalf("invalid resource result = %+v, want repair feedback without execution", payload)
	}
	if payload.Error.Code != "unknown_resource_ref" {
		t.Fatalf("invalid resource error = %+v, want unknown_resource_ref", payload.Error)
	}
}

func TestResourceReadPreparesPureReadAccessPlan(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "resource-read-scheduling")
	k := newTestKernelWithResources(t, filepath.Join(dir, "events.jsonl"), []ResourceDescriptor{
		{Ref: "res_a", MimeType: "text/plain", Text: "a"},
		{Ref: "res_b", MimeType: "text/plain", Text: "b"},
	})
	prepared, err := k.toolGateway().PrepareBatch([]ModelToolCall{
		{
			ToolCallID:      "call_resource_a",
			ToolCallEventID: "evt_tool_resource_a",
			Name:            "resource_read",
			Arguments:       mustMarshalToolArgs(t, map[string]interface{}{"resource_ref": "res_a"}),
		},
		{
			ToolCallID:      "call_resource_b",
			ToolCallEventID: "evt_tool_resource_b",
			Name:            "resource_read",
			Arguments:       mustMarshalToolArgs(t, map[string]interface{}{"resource_ref": "res_b"}),
		},
	})
	if err != nil {
		t.Fatalf("PrepareBatch returned error: %v", err)
	}
	for i, call := range prepared {
		if call.accessPlan.EffectClass != ToolEffectClassPureRead || call.accessPlan.ParallelPolicy != ToolParallelPolicyCompatibleLocks || call.accessPlan.parallelClass() != ToolEffectClassPureRead {
			t.Fatalf("prepared[%d] access plan = %+v, want trusted pure read", i, call.accessPlan)
		}
		if len(call.accessPlan.ResourceFootprint.ReadScopes) != 1 || call.accessPlan.ResourceFootprint.ReadScopes[0] == "" {
			t.Fatalf("prepared[%d] read scopes = %+v", i, call.accessPlan.ResourceFootprint.ReadScopes)
		}
	}
	batches := planToolExecutionBatches(prepared)
	assertToolBatchShape(t, batches, [][]int{{0, 1}}, []bool{true})
}

func newTestKernelWithResources(t *testing.T, ledgerPath string, resources []ResourceDescriptor) *Kernel {
	t.Helper()
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     FakeProvider{},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModePlan,
		},
		Resources: resources,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return k
}

func mustMarshalToolArgs(t *testing.T, value interface{}) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}
	return payload
}
