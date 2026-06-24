package kernel

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"genesis/internal/testsupport"
)

func TestExecuteToolBatchesKeepsEffectfulSerialProviderOrder(t *testing.T) {
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
			name:           "write_a",
			accessPlan:     serialWorkspaceWriteAccessPlan("write_a"),
			execute: func(context.Context, string, string) (ModelToolResult, error) {
				executed = append(executed, "write_a")
				return ModelToolResult{
					ToolCallID:      "provider_call_a",
					ToolCallEventID: "call_event_a",
					Name:            "write_a",
					Content:         `{"status":"ok","order":"a"}`,
				}, nil
			},
		},
		{
			eventID:        "call_event_b",
			providerCallID: "provider_call_b",
			name:           "write_b",
			accessPlan:     serialWorkspaceWriteAccessPlan("write_b"),
			execute: func(context.Context, string, string) (ModelToolResult, error) {
				executed = append(executed, "write_b")
				return ModelToolResult{
					ToolCallID:      "provider_call_b",
					ToolCallEventID: "call_event_b",
					Name:            "write_b",
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
	if strings.Join(executed, ",") != "write_a,write_b" {
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
		{tool: "write_a", forEvent: "evt_call_a", provider: "provider_call_a", eventName: "call_event_a"},
		{tool: "write_b", forEvent: "evt_call_b", provider: "provider_call_b", eventName: "call_event_b"},
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

func serialWorkspaceWriteAccessPlan(name string) ToolAccessPlan {
	return ToolAccessPlan{
		ToolName:       name,
		EffectClass:    ToolEffectClassWorkspaceWrite,
		ParallelPolicy: ToolParallelPolicySerialFence,
		Trusted:        true,
	}
}

func TestExecuteToolBatchesCommitsCompletedSerialResultBeforeLaterError(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testsupport.ProjectTempDir(t, "tool-execution-partial-error"), "events.jsonl"))
	sessionID := "session_tool_execution_partial_error"
	turnID := "turn_tool_execution_partial_error"
	toolCallEventIDs := map[string]string{
		"call_event_a": "evt_call_a",
		"call_event_b": "evt_call_b",
	}
	prepared := []preparedModelToolCall{
		{
			eventID:        "call_event_a",
			providerCallID: "provider_call_a",
			name:           "write_a",
			accessPlan:     serialWorkspaceWriteAccessPlan("write_a"),
			execute: func(context.Context, string, string) (ModelToolResult, error) {
				return ModelToolResult{
					ToolCallID:      "provider_call_a",
					ToolCallEventID: "call_event_a",
					Name:            "write_a",
					Content:         `{"status":"ok","order":"a"}`,
				}, nil
			},
		},
		{
			eventID:        "call_event_b",
			providerCallID: "provider_call_b",
			name:           "write_b",
			accessPlan:     serialWorkspaceWriteAccessPlan("write_b"),
			execute: func(context.Context, string, string) (ModelToolResult, error) {
				return ModelToolResult{}, fmt.Errorf("%w: failed after prior call", ErrToolInfrastructureFailed)
			},
		},
	}

	_, err := k.executeToolBatches(context.Background(), k.toolGateway(), sessionID, turnID, prepared, toolCallEventIDs)
	if err == nil {
		t.Fatal("executeToolBatches returned nil error, want later call failure")
	}
	events, err := k.TurnEvents(turnID)
	if err != nil {
		t.Fatalf("TurnEvents returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want committed tool.result then turn.failed", events)
	}
	first, ok := events[0].Data.(EventData)
	if !ok || first.ToolResult == nil || first.ToolResult.Tool != "write_a" {
		t.Fatalf("events[0] = %#v, want first successful tool.result committed before later failure", events[0].Data)
	}
	second, ok := events[1].Data.(EventData)
	if !ok || second.TurnError == nil || second.TurnError.Code != "tool_infrastructure_failed" {
		t.Fatalf("events[1] = %#v, want tool infrastructure turn failure", events[1].Data)
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

func TestExecuteToolBatchesRunsPureReadBatchConcurrently(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testsupport.ProjectTempDir(t, "tool-execution-parallel-pure-read"), "events.jsonl"))
	sessionID := "session_tool_execution_parallel_pure_read"
	turnID := "turn_tool_execution_parallel_pure_read"
	toolCallEventIDs := map[string]string{
		"call_event_a": "evt_call_a",
		"call_event_b": "evt_call_b",
	}
	started := make(chan string, 2)
	releaseFirst := make(chan struct{})
	prepared := []preparedModelToolCall{
		{
			eventID:        "call_event_a",
			providerCallID: "provider_call_a",
			name:           "resource_read",
			accessPlan:     pureReadAccessPlan("resource_read"),
			execute: func(context.Context, string, string) (ModelToolResult, error) {
				started <- "resource_a"
				<-releaseFirst
				return ModelToolResult{
					ToolCallID:      "provider_call_a",
					ToolCallEventID: "call_event_a",
					Name:            "resource_read",
					Content:         `{"status":"completed","resource_ref":"res_a"}`,
				}, nil
			},
		},
		{
			eventID:        "call_event_b",
			providerCallID: "provider_call_b",
			name:           "resource_read",
			accessPlan:     pureReadAccessPlan("resource_read"),
			execute: func(context.Context, string, string) (ModelToolResult, error) {
				started <- "resource_b"
				return ModelToolResult{
					ToolCallID:      "provider_call_b",
					ToolCallEventID: "call_event_b",
					Name:            "resource_read",
					Content:         `{"status":"completed","resource_ref":"res_b"}`,
				}, nil
			},
		},
	}

	errCh := make(chan error, 1)
	go func() {
		outcome, err := k.executeToolBatches(context.Background(), k.toolGateway(), sessionID, turnID, prepared, toolCallEventIDs)
		if outcome.Completed {
			errCh <- fmt.Errorf("executeToolBatches completed turn unexpectedly: %+v", outcome.Response)
			return
		}
		errCh <- err
	}()

	waitForToolStarts(t, started, []string{"resource_a", "resource_b"})
	close(releaseFirst)
	if err := <-errCh; err != nil {
		t.Fatalf("executeToolBatches returned error: %v", err)
	}
	assertCommittedToolResultOrder(t, k, turnID, []string{"provider_call_a", "provider_call_b"})
}

func TestExecuteToolBatchesKeepsProcessIOBatchSerialByDefault(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testsupport.ProjectTempDir(t, "tool-execution-process-io-serial"), "events.jsonl"))
	sessionID := "session_tool_execution_process_io_serial"
	turnID := "turn_tool_execution_process_io_serial"
	toolCallEventIDs := map[string]string{
		"call_event_a": "evt_call_a",
		"call_event_b": "evt_call_b",
	}
	started := make(chan string, 2)
	releaseFirst := make(chan struct{})
	prepared := []preparedModelToolCall{
		{
			eventID:        "call_event_a",
			providerCallID: "provider_call_a",
			name:           "job_status",
			accessPlan:     jobControlToolAccessPlan("job_status", "job_a"),
			execute: func(context.Context, string, string) (ModelToolResult, error) {
				started <- "job_a"
				<-releaseFirst
				return ModelToolResult{
					ToolCallID:      "provider_call_a",
					ToolCallEventID: "call_event_a",
					Name:            "job_status",
					Content:         `{"status":"completed","job_id":"job_a"}`,
				}, nil
			},
		},
		{
			eventID:        "call_event_b",
			providerCallID: "provider_call_b",
			name:           "job_status",
			accessPlan:     jobControlToolAccessPlan("job_status", "job_b"),
			execute: func(context.Context, string, string) (ModelToolResult, error) {
				started <- "job_b"
				return ModelToolResult{
					ToolCallID:      "provider_call_b",
					ToolCallEventID: "call_event_b",
					Name:            "job_status",
					Content:         `{"status":"completed","job_id":"job_b"}`,
				}, nil
			},
		},
	}

	batches := planToolExecutionBatches(prepared)
	assertToolBatchShape(t, batches, [][]int{{0, 1}}, []bool{true})
	errCh := make(chan error, 1)
	go func() {
		outcome, err := k.executeToolBatches(context.Background(), k.toolGateway(), sessionID, turnID, prepared, toolCallEventIDs)
		if outcome.Completed {
			errCh <- fmt.Errorf("executeToolBatches completed turn unexpectedly: %+v", outcome.Response)
			return
		}
		errCh <- err
	}()

	waitForToolStart(t, started, "job_a")
	assertNoToolStartBeforeRelease(t, started)
	close(releaseFirst)
	waitForToolStart(t, started, "job_b")
	if err := <-errCh; err != nil {
		t.Fatalf("executeToolBatches returned error: %v", err)
	}
	assertCommittedToolResultOrder(t, k, turnID, []string{"provider_call_a", "provider_call_b"})
}

func waitForToolStart(t *testing.T, started <-chan string, want string) {
	t.Helper()
	select {
	case got := <-started:
		if got != want {
			t.Fatalf("started tool = %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s to start", want)
	}
}

func waitForToolStarts(t *testing.T, started <-chan string, want []string) {
	t.Helper()
	remaining := map[string]bool{}
	for _, name := range want {
		remaining[name] = true
	}
	deadline := time.After(2 * time.Second)
	for len(remaining) > 0 {
		select {
		case got := <-started:
			if !remaining[got] {
				t.Fatalf("unexpected or duplicate started tool = %q, remaining = %v", got, remaining)
			}
			delete(remaining, got)
		case <-deadline:
			t.Fatalf("timed out waiting for tool starts, remaining = %v", remaining)
		}
	}
}

func assertNoToolStartBeforeRelease(t *testing.T, started <-chan string) {
	t.Helper()
	select {
	case got := <-started:
		t.Fatalf("tool %q started before serial predecessor released", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func assertCommittedToolResultOrder(t *testing.T, k *Kernel, turnID string, providerCallIDs []string) {
	t.Helper()
	events, err := k.TurnEvents(turnID)
	if err != nil {
		t.Fatalf("TurnEvents returned error: %v", err)
	}
	if len(events) != len(providerCallIDs) {
		t.Fatalf("events = %+v, want %d tool.result events", events, len(providerCallIDs))
	}
	for i, want := range providerCallIDs {
		data, ok := events[i].Data.(EventData)
		if !ok || data.ToolResult == nil {
			t.Fatalf("events[%d] = %#v, want tool.result data", i, events[i].Data)
		}
		if data.ToolResult.ProviderToolCallID != want {
			t.Fatalf("events[%d] provider call id = %q, want %q", i, data.ToolResult.ProviderToolCallID, want)
		}
	}
}
