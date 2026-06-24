package kernel

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"genesis/internal/testsupport"
)

func TestExecuteToolBatchesKeepsCurrentSerialProviderOrder(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testsupport.ProjectTempDir(t, "tool-execution-order"), "events.jsonl"))
	sessionID := "session_tool_execution_order"
	turnID := "turn_tool_execution_order"
	toolCallEventIDs := map[string]string{
		"call_event_a": "evt_call_a",
		"call_event_b": "evt_call_b",
	}
	executed := []string{}
	prepared := []preparedModelToolCall{
		{
			eventID:        "call_event_a",
			providerCallID: "provider_call_a",
			name:           "read_a",
			accessPlan:     pureReadAccessPlan("read_a"),
			execute: func(context.Context, string, string) (ModelToolResult, error) {
				executed = append(executed, "read_a")
				return ModelToolResult{
					ToolCallID:      "provider_call_a",
					ToolCallEventID: "call_event_a",
					Name:            "read_a",
					Content:         `{"status":"ok","order":"a"}`,
				}, nil
			},
		},
		{
			eventID:        "call_event_b",
			providerCallID: "provider_call_b",
			name:           "read_b",
			accessPlan:     pureReadAccessPlan("read_b"),
			execute: func(context.Context, string, string) (ModelToolResult, error) {
				executed = append(executed, "read_b")
				return ModelToolResult{
					ToolCallID:      "provider_call_b",
					ToolCallEventID: "call_event_b",
					Name:            "read_b",
					Content:         `{"status":"ok","order":"b"}`,
				}, nil
			},
		},
	}

	outcome, err := k.executeToolBatches(context.Background(), k.toolGateway(), sessionID, turnID, prepared, toolCallEventIDs)
	if err != nil {
		t.Fatalf("executeToolBatches returned error: %v", err)
	}
	if outcome.Completed {
		t.Fatalf("executeToolBatches completed turn unexpectedly: %+v", outcome.Response)
	}
	if strings.Join(executed, ",") != "read_a,read_b" {
		t.Fatalf("executed order = %v, want provider order", executed)
	}

	events, err := k.TurnEvents(turnID)
	if err != nil {
		t.Fatalf("TurnEvents returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want two tool.result events", events)
	}
	for i, want := range []struct {
		tool      string
		forEvent  string
		provider  string
		eventName string
	}{
		{tool: "read_a", forEvent: "evt_call_a", provider: "provider_call_a", eventName: "call_event_a"},
		{tool: "read_b", forEvent: "evt_call_b", provider: "provider_call_b", eventName: "call_event_b"},
	} {
		data, ok := events[i].Data.(EventData)
		if !ok || data.ToolResult == nil {
			t.Fatalf("events[%d] = %#v, want tool.result data", i, events[i].Data)
		}
		result := data.ToolResult
		if result.Tool != want.tool || result.ForEventID != want.forEvent || result.ProviderToolCallID != want.provider || result.ToolCallEventID != want.eventName {
			t.Fatalf("events[%d] tool.result = %+v, want %+v", i, result, want)
		}
	}
}

func TestExecuteToolBatchesCommitsOutOfOrderBatchResultsInProviderOrder(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testsupport.ProjectTempDir(t, "tool-execution-out-of-order"), "events.jsonl"))
	sessionID := "session_tool_execution_out_of_order"
	turnID := "turn_tool_execution_out_of_order"
	toolCallEventIDs := map[string]string{
		"call_event_a": "evt_call_a",
		"call_event_b": "evt_call_b",
	}
	prepared := []preparedModelToolCall{
		{
			eventID:        "call_event_a",
			providerCallID: "provider_call_a",
			name:           "read_a",
			accessPlan:     pureReadAccessPlan("read_a"),
		},
		{
			eventID:        "call_event_b",
			providerCallID: "provider_call_b",
			name:           "read_b",
			accessPlan:     pureReadAccessPlan("read_b"),
		},
	}
	completedOrder := []string{}
	runner := func(context.Context, ToolGateway, string, string, []preparedModelToolCall, ToolExecutionBatch) ([]toolCallExecutionResult, error) {
		completedOrder = append(completedOrder, "read_b", "read_a")
		return []toolCallExecutionResult{
			{
				CallIndex: 1,
				Result: ModelToolResult{
					ToolCallID:      "provider_call_b",
					ToolCallEventID: "call_event_b",
					Name:            "read_b",
					Content:         `{"status":"ok","order":"b"}`,
				},
			},
			{
				CallIndex: 0,
				Result: ModelToolResult{
					ToolCallID:      "provider_call_a",
					ToolCallEventID: "call_event_a",
					Name:            "read_a",
					Content:         `{"status":"ok","order":"a"}`,
				},
			},
		}, nil
	}

	outcome, err := k.executeToolBatchesWithRunner(context.Background(), k.toolGateway(), sessionID, turnID, prepared, toolCallEventIDs, runner)
	if err != nil {
		t.Fatalf("executeToolBatchesWithRunner returned error: %v", err)
	}
	if outcome.Completed {
		t.Fatalf("executeToolBatchesWithRunner completed turn unexpectedly: %+v", outcome.Response)
	}
	if strings.Join(completedOrder, ",") != "read_b,read_a" {
		t.Fatalf("fake runner completed order = %v, want out-of-order completion", completedOrder)
	}

	events, err := k.TurnEvents(turnID)
	if err != nil {
		t.Fatalf("TurnEvents returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want two tool.result events", events)
	}
	for i, want := range []string{"read_a", "read_b"} {
		data, ok := events[i].Data.(EventData)
		if !ok || data.ToolResult == nil {
			t.Fatalf("events[%d] = %#v, want tool.result data", i, events[i].Data)
		}
		if data.ToolResult.Tool != want {
			t.Fatalf("events[%d] tool = %q, want provider-order %q", i, data.ToolResult.Tool, want)
		}
	}
}
