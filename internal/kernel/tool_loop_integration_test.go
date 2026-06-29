package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal(t *testing.T) {
	workspace := testTempDir(t)
	toolCommand := writeFileCommand("tool-result.txt", "toolvalue")
	toolArgs, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": toolCommand,
	})
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		messages, ok := req["messages"].([]interface{})
		if !ok || len(messages) == 0 {
			t.Fatalf("messages = %#v, want non-empty array", req["messages"])
		}
		w.Header().Set("Content-Type", "application/json")
		switch callCount {
		case 1:
			tools, ok := req["tools"].([]interface{})
			if !ok || len(tools) == 0 {
				http.Error(w, "missing shell_exec tool descriptor", http.StatusBadRequest)
				return
			}
			toolNames := providerToolNamesFromRequest(t, tools)
			if !containsString(toolNames, "shell_exec") {
				t.Fatalf("provider tool names = %v, want canonical shell_exec", toolNames)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"model": "served-model",
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": nil,
							"tool_calls": []interface{}{
								map[string]interface{}{
									"id":   "call_write_file",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "shell_exec",
										"arguments": string(toolArgs),
									},
								},
							},
						},
					},
				},
			})
		case 2:
			if len(messages) != 3 {
				t.Fatalf("second request messages = %#v, want user, assistant tool call, tool result", messages)
			}
			assistantMessage, ok := messages[1].(map[string]interface{})
			if !ok {
				t.Fatalf("assistant message = %#v", messages[1])
			}
			assistantToolCalls, ok := assistantMessage["tool_calls"].([]interface{})
			if !ok || len(assistantToolCalls) != 1 {
				t.Fatalf("assistant tool calls = %#v, want replayed provider tool call", assistantMessage["tool_calls"])
			}
			assistantToolCall, ok := assistantToolCalls[0].(map[string]interface{})
			if !ok {
				t.Fatalf("assistant tool call = %#v", assistantToolCalls[0])
			}
			assistantFunction, ok := assistantToolCall["function"].(map[string]interface{})
			if !ok || assistantFunction["name"] != "shell_exec" {
				t.Fatalf("assistant tool call function = %#v, want provider-safe shell_exec", assistantToolCall["function"])
			}
			if assistantFunction["arguments"] != string(toolArgs) {
				t.Fatalf("assistant tool call arguments = %#v, want replayed arguments from tool.call event", assistantFunction["arguments"])
			}
			toolMessage, ok := messages[2].(map[string]interface{})
			if !ok {
				t.Fatalf("tool message = %#v", messages[2])
			}
			if toolMessage["role"] != "tool" || toolMessage["tool_call_id"] != "call_write_file" {
				t.Fatalf("tool message = %#v, want shell tool result for call_write_file", toolMessage)
			}
			content, _ := toolMessage["content"].(string)
			payload := decodeJSONMap(t, content)
			if payload["status"] != "completed" || payload["executed"] != true {
				t.Fatalf("tool evidence content = %q, want completed minimal shell result", content)
			}
			for _, forbidden := range []string{"tool", "permission_mode", "cwd", "command", "operation_id", "blocked_reason", "infrastructure_reason"} {
				if _, ok := payload[forbidden]; ok {
					t.Fatalf("tool evidence payload = %+v, must not expose %q to model", payload, forbidden)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"model": "served-model",
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "tool evidence received",
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected provider call %d", callCount)
		}
	}))
	defer server.Close()

	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k, err := New(Config{
		LedgerPath: ledgerPath,
		Provider: NewOpenAICompatibleProvider(OpenAICompatibleConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "test-model",
		}),
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
		Clock: func() time.Time {
			return time.Date(2026, 6, 22, 1, 2, 3, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "provider-tool-loop",
		InputItems: []InputItem{{Type: "text", Text: "write the file"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "tool evidence received" {
		t.Fatalf("final text = %q, want tool evidence received", resp.Final.Text)
	}
	if callCount != 2 {
		t.Fatalf("provider call count = %d, want 2", callCount)
	}
	fileContent, err := os.ReadFile(filepath.Join(workspace, "tool-result.txt"))
	if err != nil {
		t.Fatalf("read tool output file: %v", err)
	}
	if string(fileContent) != "toolvalue" {
		t.Fatalf("tool output file = %q, want toolvalue", string(fileContent))
	}

	restarted := newTestKernelWithRuntimeTokenAndPolicy(t, ledgerPath, testRuntimeToken, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})
	events, err := restarted.TurnEvents(resp.TurnID)
	if err != nil {
		t.Fatalf("TurnEvents returned error: %v", err)
	}
	eventTypes := make([]string, 0, len(events))
	for _, event := range events {
		eventTypes = append(eventTypes, event.Type)
	}
	wantTypes := []string{"turn.submitted", "tool.call", "operation.running", "operation.completed", "tool.result", "model.final"}
	if strings.Join(eventTypes, ",") != strings.Join(wantTypes, ",") {
		t.Fatalf("turn event types = %v, want %v", eventTypes, wantTypes)
	}
	toolCallData, ok := events[1].Data.(EventData)
	if !ok {
		t.Fatalf("tool call data = %#v, want EventData", events[1].Data)
	}
	if toolCallData.ToolCall == nil || toolCallData.ToolCall.Tool != "shell_exec" || toolCallData.ToolCall.ToolCallEventID == "" {
		t.Fatalf("tool call event = %+v, want canonical shell_exec", toolCallData.ToolCall)
	}
	if toolCallData.ToolCall.ToolCallEventID != events[1].EventID || toolCallData.ToolCall.ProviderToolCallID != "call_write_file" {
		t.Fatalf("tool call event = %+v, want event id identity and provider correlation", toolCallData.ToolCall)
	}
	if !strings.Contains(toolCallData.ToolCall.Arguments, "tool-result.txt") {
		t.Fatalf("tool call arguments = %s, want provider replay arguments", toolCallData.ToolCall.Arguments)
	}
	toolResultData, ok := events[4].Data.(EventData)
	if !ok {
		t.Fatalf("tool result data = %#v, want EventData", events[4].Data)
	}
	if toolResultData.ToolResult == nil || toolResultData.ToolResult.ForEventID != events[1].EventID || toolResultData.ToolResult.Status != "completed" {
		t.Fatalf("tool result event = %+v, want result linked to %s", toolResultData.ToolResult, events[1].EventID)
	}
	if toolResultData.ToolResult.ToolCallEventID != events[1].EventID || toolResultData.ToolResult.ProviderToolCallID != "call_write_file" {
		t.Fatalf("tool result event = %+v, want event id identity and provider correlation", toolResultData.ToolResult)
	}
	if toolCallData.ToolCall.Arguments != string(toolArgs) {
		t.Fatalf("tool call event arguments = %s, want original provider arguments %s", toolCallData.ToolCall.Arguments, string(toolArgs))
	}
	session, err := restarted.Session("provider-tool-loop")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(session.Events) != len(events) {
		t.Fatalf("session events = %d, want %d", len(session.Events), len(events))
	}
	if session.Events[1].Data.ToolCall == nil || session.Events[1].Data.ToolCall.Tool != "shell_exec" {
		t.Fatalf("session tool.call event = %+v, want payload", session.Events[1].Data.ToolCall)
	}
	if session.Events[4].Data.ToolResult == nil || session.Events[4].Data.ToolResult.ForEventID != session.Events[1].EventID {
		t.Fatalf("session tool.result event = %+v, want for_event_id=%s", session.Events[4].Data.ToolResult, session.Events[1].EventID)
	}
}

func TestSubmitTurnUsesToolCallEventIDWhenProviderIDMissing(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": writeFileCommand("missing-provider-id.txt", "event-id"),
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "event id tool slot observed",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "missing-provider-tool-id",
		InputItems: []InputItem{{Type: "text", Text: "write file without provider tool id"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "event id tool slot observed" {
		t.Fatalf("final text = %q, want event id tool slot observed", resp.Final.Text)
	}
	requests := provider.Requests()
	if len(requests) != 2 || len(requests[1].ToolRounds) != 1 || len(requests[1].ToolRounds[0].Results) != 1 {
		t.Fatalf("provider requests = %+v, want tool result round", requests)
	}
	result := requests[1].ToolRounds[0].Results[0]
	if result.ToolCallID != "" || result.ToolCallEventID != "" {
		t.Fatalf("tool result = %+v, want no provider id and no kernel event id in provider context", result)
	}
	projection, err := k.Session("missing-provider-tool-id")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Events) < 5 || projection.Events[1].Data.ToolCall == nil || projection.Events[4].Data.ToolResult == nil {
		t.Fatalf("events = %+v, want tool.call and tool.result payloads", projection.Events)
	}
	if projection.Events[1].Data.ToolCall.ToolCallEventID != projection.Events[1].EventID || projection.Events[1].Data.ToolCall.ProviderToolCallID != "" {
		t.Fatalf("tool.call = %+v, want event id identity without provider id", projection.Events[1].Data.ToolCall)
	}
	if projection.Events[4].Data.ToolResult.ToolCallEventID != projection.Events[1].EventID || projection.Events[4].Data.ToolResult.ForEventID != projection.Events[1].EventID {
		t.Fatalf("tool.result = %+v, want event id identity and for_event_id link", projection.Events[4].Data.ToolResult)
	}
}

func TestSubmitTurnReturnsRepairFeedbackForInvalidShellArguments(t *testing.T) {
	workspace := testTempDir(t)
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{
				ToolCallID: "call_missing_command",
				Name:       "shell_exec",
				Arguments:  json.RawMessage(`{"cwd":"` + filepath.ToSlash(workspace) + `"}`),
			},
		},
		final: "repair feedback received",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "invalid-shell-arguments",
		InputItems: []InputItem{{Type: "text", Text: "try malformed shell call"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "repair feedback received" {
		t.Fatalf("final text = %q, want repair feedback received", resp.Final.Text)
	}
	requests := provider.Requests()
	if len(requests) != 2 {
		t.Fatalf("provider requests = %d, want tool repair round", len(requests))
	}
	rounds := requests[1].ToolRounds
	if len(rounds) != 1 || len(rounds[0].Results) != 1 {
		t.Fatalf("tool rounds = %+v, want one repair result", rounds)
	}
	result := rounds[0].Results[0]
	if result.ToolCallID != "call_missing_command" || result.ToolCallEventID != "" || result.Name != "shell_exec" {
		t.Fatalf("tool result = %+v, want provider echo id without kernel event id for call_missing_command", result)
	}
	payload := decodeJSONMap(t, result.Content)
	if payload["status"] != "tool_request_invalid" || payload["tool"] != "shell_exec" || payload["executed"] != false {
		t.Fatalf("repair payload = %+v, want non-executed tool_request_invalid", payload)
	}
	if _, ok := payload["tool_call_id"]; ok {
		t.Fatalf("repair payload = %+v, must not duplicate tool_call_id inside model-visible content", payload)
	}
	errorPayload, ok := payload["error"].(map[string]interface{})
	if !ok || errorPayload["code"] != "invalid_shell_exec_request" {
		t.Fatalf("repair error = %+v, want invalid_shell_exec_request", payload["error"])
	}
	projection, err := k.Session("invalid-shell-arguments")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 0 {
		t.Fatalf("operations = %+v, want no shell effect for invalid request", projection.Operations)
	}
}

func TestSubmitTurnUsesKernelEventIDForUnsafeProviderToolCallID(t *testing.T) {
	workspace := testTempDir(t)
	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider: &toolFeedbackProvider{
			calls: []ModelToolCall{{
				ToolCallID: "bad tool call id",
				Name:       "shell_exec",
				Arguments:  json.RawMessage(`{"command":"` + echoCommand("hello") + `"}`),
			}},
			final: "unsafe provider id did not become kernel identity",
		},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "unsafe-provider-tool-call-id",
		InputItems: []InputItem{{Type: "text", Text: "try unsafe provider tool call id"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "unsafe provider id did not become kernel identity" {
		t.Fatalf("final text = %q, want unsafe provider id did not become kernel identity", resp.Final.Text)
	}
	projection, err := k.Session("unsafe-provider-tool-call-id")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Events) < 5 || projection.Events[1].Data.ToolCall == nil || projection.Events[4].Data.ToolResult == nil {
		t.Fatalf("events = %+v, want tool call/result payloads", projection.Events)
	}
	if projection.Events[1].Data.ToolCall.ToolCallEventID != projection.Events[1].EventID || projection.Events[1].Data.ToolCall.ProviderToolCallID != "provider_tool_call_id_unavailable" {
		t.Fatalf("tool.call = %+v, want event id identity and redacted provider correlation", projection.Events[1].Data.ToolCall)
	}
	if projection.Events[4].Data.ToolResult.ToolCallEventID != projection.Events[1].EventID || projection.Events[4].Data.ToolResult.ProviderToolCallID != "provider_tool_call_id_unavailable" {
		t.Fatalf("tool.result = %+v, want event id identity and redacted provider correlation", projection.Events[4].Data.ToolResult)
	}
}

func TestSubmitTurnRejectsProviderSuppliedKernelToolEventID(t *testing.T) {
	workspace := testTempDir(t)
	outputPath := filepath.Join(workspace, "forged-event-id.txt")
	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider: &toolFeedbackProvider{
			calls: []ModelToolCall{{
				ToolCallID:      "call_provider_visible",
				ToolCallEventID: "evt_forged_by_provider",
				Name:            "shell_exec",
				Arguments:       json.RawMessage(`{"command":"` + writeFileCommand("forged-event-id.txt", "effect") + `"}`),
			}},
			final: "must not reach final",
		},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "provider-forged-event-id",
		InputItems: []InputItem{{Type: "text", Text: "try forged event id"}},
	})
	if !errors.Is(err, ErrModelToolCallRejected) {
		t.Fatalf("SubmitTurn error = %v, want ErrModelToolCallRejected", err)
	}
	if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("forged provider event id executed effect; stat err=%v", statErr)
	}
	projection, sessionErr := k.Session("provider-forged-event-id")
	if sessionErr != nil {
		t.Fatalf("Session returned error: %v", sessionErr)
	}
	for _, event := range projection.Events {
		if event.Type == "tool.call" || event.Type == "operation.running" || event.Type == "operation.completed" {
			t.Fatalf("event %s was recorded before rejecting forged provider event id: %+v", event.Type, event)
		}
	}
}

func TestSubmitTurnFeedsNonZeroShellExitToModel(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": failingShellCommand(),
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_failing_command", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "command failure observed",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "nonzero-shell-exit",
		InputItems: []InputItem{{Type: "text", Text: "run a failing command"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "command failure observed" {
		t.Fatalf("final text = %q, want command failure observed", resp.Final.Text)
	}
	requests := provider.Requests()
	if len(requests) != 2 {
		t.Fatalf("provider requests = %d, want tool result round", len(requests))
	}
	result := requests[1].ToolRounds[0].Results[0]
	payload := decodeJSONMap(t, result.Content)
	if payload["status"] != "failed" || payload["executed"] != true {
		t.Fatalf("tool result payload = %+v, want failed executed command", payload)
	}
	assertJSONNumber(t, payload, "exit_code", 7)
	stderr, _ := payload["stderr"].(string)
	if !strings.Contains(stderr, "GENESIS_TOOL_COMMAND_FAILURE") {
		t.Fatalf("stderr = %q, want command failure marker", stderr)
	}
	for _, forbidden := range []string{"tool", "operation_id", "session_id", "turn_id", "idempotency_key", "started_at", "ended_at", "permission_mode", "authority_policy", "sandbox_profile", "approval_policy", "cwd", "command", "blocked_reason", "infrastructure_reason"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("tool result payload = %+v, must not expose control-plane field %q", payload, forbidden)
		}
	}
}

func TestSubmitTurnReturnsMinimalPermissionDeniedToolResult(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": echoCommand("blocked"),
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_plan_blocked", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "permission feedback received",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModePlan,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "model-visible-permission-denied",
		InputItems: []InputItem{{Type: "text", Text: "try blocked shell"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "permission feedback received" {
		t.Fatalf("final text = %q, want permission feedback received", resp.Final.Text)
	}
	result := provider.Requests()[1].ToolRounds[0].Results[0]
	payload := decodeJSONMap(t, result.Content)
	if payload["status"] != "permission_denied" || payload["executed"] != false {
		t.Fatalf("tool result payload = %+v, want minimal permission_denied", payload)
	}
	errorPayload, ok := payload["error"].(map[string]interface{})
	if !ok || errorPayload["code"] != "permission_denied" {
		t.Fatalf("tool result error = %+v, want permission_denied", payload["error"])
	}
	for _, forbidden := range []string{"permission_mode", "authority_policy", "sandbox_profile", "approval_policy", "blocked_reason", "operation_id", "cwd", "command", "started_at", "ended_at", "infrastructure_reason"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("tool result payload = %+v, must not expose %q to model", payload, forbidden)
		}
	}
	projection, err := k.Session("model-visible-permission-denied")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 1 || projection.Operations[0].Status != "blocked" || projection.Operations[0].PermissionMode != PermissionModePlan || projection.Operations[0].BlockedReason == "" {
		t.Fatalf("operations = %+v, want full blocked operation evidence in inspection projection", projection.Operations)
	}
}

func TestSubmitTurnBlocksUnavailableSandboxProfileBeforeExecution(t *testing.T) {
	workspace := testTempDir(t)
	target := filepath.Join(workspace, "sandbox-profile-should-not-run.txt")
	arguments, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": writeFileCommand(filepath.Base(target), "blocked"),
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_unavailable_sandbox", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "sandbox feedback received",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
			SandboxProfile: SandboxProfileOSWorkspace,
			ApprovalPolicy: ApprovalPolicyNever,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "unavailable-sandbox-profile",
		InputItems: []InputItem{{Type: "text", Text: "try unavailable sandbox shell"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "sandbox feedback received" {
		t.Fatalf("final text = %q, want sandbox feedback received", resp.Final.Text)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("blocked sandbox command created %q; stat err=%v", target, err)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "sandbox_profile_unavailable" || payload["executed"] != false {
		t.Fatalf("tool result payload = %+v, want sandbox_profile_unavailable without execution", payload)
	}
	errorPayload, ok := payload["error"].(map[string]interface{})
	if !ok || errorPayload["code"] != "sandbox_profile_unavailable" {
		t.Fatalf("tool result error = %+v, want sandbox_profile_unavailable", payload["error"])
	}
	for _, forbidden := range []string{"permission_mode", "authority_policy", "sandbox_profile", "approval_policy", "blocked_reason", "operation_id", "cwd", "command", "started_at", "ended_at", "infrastructure_reason"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("tool result payload = %+v, must not expose %q to model", payload, forbidden)
		}
	}
	projection, err := k.Session("unavailable-sandbox-profile")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 1 || projection.Operations[0].Status != "blocked" || projection.Operations[0].BlockedReason != "sandbox_profile_unavailable=os_workspace" {
		t.Fatalf("operations = %+v, want blocked unavailable sandbox evidence", projection.Operations)
	}
}

func TestSubmitTurnBlocksReadOnlySandboxOverrideBeforeExecution(t *testing.T) {
	for _, permissionMode := range []string{PermissionModeDefault, PermissionModeYolo} {
		t.Run(permissionMode, func(t *testing.T) {
			workspace := testTempDir(t)
			target := filepath.Join(workspace, "read-only-sandbox-should-not-run.txt")
			arguments, err := json.Marshal(map[string]string{
				"cwd":     workspace,
				"command": writeFileCommand(filepath.Base(target), "blocked"),
			})
			if err != nil {
				t.Fatalf("marshal shell args: %v", err)
			}
			provider := &toolFeedbackProvider{
				calls: []ModelToolCall{
					{ToolCallID: "call_read_only_sandbox_" + permissionMode, Name: "shell_exec", Arguments: json.RawMessage(arguments)},
				},
				final: "read-only sandbox feedback received",
			}
			k, err := New(Config{
				LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
				Provider:     provider,
				RuntimeToken: testRuntimeToken,
				ToolPolicy: ToolPolicy{
					PermissionMode: permissionMode,
					WorkspaceRoot:  workspace,
					SandboxProfile: SandboxProfileReadOnly,
					ApprovalPolicy: ApprovalPolicyNever,
				},
			})
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}

			sessionID := "read-only-sandbox-override-" + permissionMode
			resp, err := k.SubmitTurn(context.Background(), TurnRequest{
				SessionID:  sessionID,
				InputItems: []InputItem{{Type: "text", Text: "try read-only sandbox shell"}},
			})
			if err != nil {
				t.Fatalf("SubmitTurn returned error: %v", err)
			}
			if resp.Final.Text != "read-only sandbox feedback received" {
				t.Fatalf("final text = %q, want read-only sandbox feedback received", resp.Final.Text)
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("blocked read-only sandbox command created %q; stat err=%v", target, err)
			}
			payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
			if payload["status"] != "permission_denied" || payload["executed"] != false {
				t.Fatalf("tool result payload = %+v, want permission_denied without execution", payload)
			}
			projection, err := k.Session(sessionID)
			if err != nil {
				t.Fatalf("Session returned error: %v", err)
			}
			if len(projection.Operations) != 1 || projection.Operations[0].Status != "blocked" || projection.Operations[0].BlockedReason != "sandbox_profile_not_allowed_for_permission_mode" {
				t.Fatalf("operations = %+v, want blocked read-only sandbox override evidence", projection.Operations)
			}
		})
	}
}

func TestSubmitTurnBlocksApprovalRequiredBeforeExecution(t *testing.T) {
	workspace := testTempDir(t)
	target := filepath.Join(workspace, "approval-should-not-run.txt")
	arguments, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": writeFileCommand(filepath.Base(target), "blocked"),
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_approval_required", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "approval feedback received",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
			ApprovalPolicy: ApprovalPolicyOnRequest,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "approval-required-shell",
		InputItems: []InputItem{{Type: "text", Text: "try approval shell"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "approval feedback received" {
		t.Fatalf("final text = %q, want approval feedback received", resp.Final.Text)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("approval-blocked command created %q; stat err=%v", target, err)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "approval_required" || payload["executed"] != false {
		t.Fatalf("tool result payload = %+v, want approval_required without execution", payload)
	}
	errorPayload, ok := payload["error"].(map[string]interface{})
	if !ok || errorPayload["code"] != "approval_required" {
		t.Fatalf("tool result error = %+v, want approval_required", payload["error"])
	}
	for _, forbidden := range []string{"permission_mode", "authority_policy", "sandbox_profile", "approval_policy", "blocked_reason", "operation_id", "cwd", "command", "started_at", "ended_at", "infrastructure_reason"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("tool result payload = %+v, must not expose %q to model", payload, forbidden)
		}
	}
	projection, err := k.Session("approval-required-shell")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 1 || projection.Operations[0].Status != "blocked" || projection.Operations[0].BlockedReason != "approval_required" {
		t.Fatalf("operations = %+v, want blocked approval evidence", projection.Operations)
	}
}

func TestSubmitTurnPlanOnRequestKeepsReadOnlyDenialBeforeApproval(t *testing.T) {
	workspace := testTempDir(t)
	target := filepath.Join(workspace, "plan-approval-should-not-run.txt")
	arguments, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": writeFileCommand(filepath.Base(target), "blocked"),
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_plan_on_request", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "plan denial feedback received",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModePlan,
			WorkspaceRoot:  workspace,
			ApprovalPolicy: ApprovalPolicyOnRequest,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "plan-on-request-read-only-denial",
		InputItems: []InputItem{{Type: "text", Text: "try plan shell with approval policy"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "plan denial feedback received" {
		t.Fatalf("final text = %q, want plan denial feedback received", resp.Final.Text)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("plan-blocked command created %q; stat err=%v", target, err)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "permission_denied" || payload["executed"] != false {
		t.Fatalf("tool result payload = %+v, want permission_denied without execution", payload)
	}
	errorPayload, ok := payload["error"].(map[string]interface{})
	if !ok || errorPayload["code"] != "permission_denied" {
		t.Fatalf("tool result error = %+v, want permission_denied", payload["error"])
	}
	projection, err := k.Session("plan-on-request-read-only-denial")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 1 || projection.Operations[0].Status != "blocked" || projection.Operations[0].BlockedReason != "blocked_by_permission_mode=plan" {
		t.Fatalf("operations = %+v, want hard read-only denial evidence", projection.Operations)
	}
}

func TestSubmitTurnAcceptsForegroundShellTimeoutSeconds(t *testing.T) {
	for _, timeoutSec := range []int{1, 180} {
		t.Run(fmt.Sprintf("timeout_%d", timeoutSec), func(t *testing.T) {
			workspace := testTempDir(t)
			arguments, err := json.Marshal(map[string]interface{}{
				"cwd":         workspace,
				"command":     echoCommand("foreground-timeout"),
				"timeout_sec": timeoutSec,
			})
			if err != nil {
				t.Fatalf("marshal shell args: %v", err)
			}
			provider := &toolFeedbackProvider{
				calls: []ModelToolCall{
					{ToolCallID: fmt.Sprintf("call_timeout_%d", timeoutSec), Name: "shell_exec", Arguments: json.RawMessage(arguments)},
				},
				final: "foreground timeout accepted",
			}
			k, err := New(Config{
				LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
				Provider:     provider,
				RuntimeToken: testRuntimeToken,
				ToolPolicy: ToolPolicy{
					PermissionMode: PermissionModeYolo,
					WorkspaceRoot:  workspace,
				},
			})
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}

			resp, err := k.SubmitTurn(context.Background(), TurnRequest{
				SessionID:  fmt.Sprintf("foreground-timeout-%d", timeoutSec),
				InputItems: []InputItem{{Type: "text", Text: "run foreground timeout shell"}},
			})
			if err != nil {
				t.Fatalf("SubmitTurn returned error: %v", err)
			}
			if resp.Final.Text != "foreground timeout accepted" {
				t.Fatalf("final text = %q, want foreground timeout accepted", resp.Final.Text)
			}
			payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
			if payload["status"] != "completed" || payload["executed"] != true {
				t.Fatalf("tool result payload = %+v, want completed foreground execution", payload)
			}
			projection, err := k.Session(fmt.Sprintf("foreground-timeout-%d", timeoutSec))
			if err != nil {
				t.Fatalf("Session returned error: %v", err)
			}
			if len(projection.Operations) != 1 {
				t.Fatalf("operations = %+v, want one foreground shell operation", projection.Operations)
			}
			operationPayload := operationJSONMap(t, projection.Operations[0])
			assertJSONNumber(t, operationPayload, "timeout_sec", timeoutSec)
		})
	}
}

func TestSubmitTurnForegroundShellTimeoutRecordsTerminalOutcome(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     timeoutAfterOutputCommand(),
		"timeout_sec": 1,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_foreground_timeout_outcome", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "timeout outcome observed",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "foreground-timeout-outcome",
		InputItems: []InputItem{{Type: "text", Text: "run foreground timeout shell"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "timeout outcome observed" {
		t.Fatalf("final text = %q, want timeout outcome observed", resp.Final.Text)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "failed" || payload["executed"] != true {
		t.Fatalf("tool result payload = %+v, want failed executed command result", payload)
	}
	if payload["timed_out"] != true || payload["timeout_reason"] != "foreground_timeout" {
		t.Fatalf("tool result payload = %+v, want foreground timeout metadata", payload)
	}
	if !strings.Contains(fmt.Sprint(payload["stdout"]), "before-timeout") {
		t.Fatalf("tool result stdout = %+v, want captured pre-timeout output", payload["stdout"])
	}
	elapsed, ok := payload["elapsed_ms"].(float64)
	if !ok || elapsed <= 0 {
		t.Fatalf("tool result elapsed_ms = %+v, want positive elapsed time", payload["elapsed_ms"])
	}

	projection, err := k.Session("foreground-timeout-outcome")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 1 {
		t.Fatalf("operations = %+v, want one timeout operation", projection.Operations)
	}
	operation := projection.Operations[0]
	if operation.Status != "failed" || !operation.TimedOut || operation.TimeoutReason != "foreground_timeout" {
		t.Fatalf("operation = %+v, want failed foreground timeout operation", operation)
	}
	if operation.InfrastructureReason != "" {
		t.Fatalf("operation infrastructure reason = %q, want ordinary timeout outcome", operation.InfrastructureReason)
	}
	if operation.ElapsedMs <= 0 {
		t.Fatalf("operation elapsed_ms = %d, want positive elapsed time", operation.ElapsedMs)
	}
	if !strings.Contains(operation.Stdout, "before-timeout") {
		t.Fatalf("operation stdout = %q, want captured pre-timeout output", operation.Stdout)
	}
}

func TestSubmitTurnDefaultsShellTimeoutToThirtySeconds(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": echoCommand("default-timeout"),
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_default_timeout", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "default timeout accepted",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "default-shell-timeout",
		InputItems: []InputItem{{Type: "text", Text: "run default timeout shell"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	projection, err := k.Session("default-shell-timeout")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 1 {
		t.Fatalf("operations = %+v, want one foreground shell operation", projection.Operations)
	}
	operationPayload := operationJSONMap(t, projection.Operations[0])
	assertJSONNumber(t, operationPayload, "timeout_sec", 30)
}

func TestSubmitTurnReturnsRepairFeedbackForInvalidShellTimeoutSeconds(t *testing.T) {
	cases := []struct {
		name      string
		arguments string
	}{
		{
			name:      "zero",
			arguments: `{"command":"` + echoCommand("invalid-timeout") + `","timeout_sec":0}`,
		},
		{
			name:      "negative",
			arguments: `{"command":"` + echoCommand("invalid-timeout") + `","timeout_sec":-1}`,
		},
		{
			name:      "string",
			arguments: `{"command":"` + echoCommand("invalid-timeout") + `","timeout_sec":"30"}`,
		},
		{
			name:      "fractional",
			arguments: `{"command":"` + echoCommand("invalid-timeout") + `","timeout_sec":1.5}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workspace := testTempDir(t)
			provider := &toolFeedbackProvider{
				calls: []ModelToolCall{
					{ToolCallID: "call_invalid_timeout_" + tc.name, Name: "shell_exec", Arguments: json.RawMessage(tc.arguments)},
				},
				final: "invalid timeout repair received",
			}
			k, err := New(Config{
				LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
				Provider:     provider,
				RuntimeToken: testRuntimeToken,
				ToolPolicy: ToolPolicy{
					PermissionMode: PermissionModeDefault,
					WorkspaceRoot:  workspace,
				},
			})
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}

			_, err = k.SubmitTurn(context.Background(), TurnRequest{
				SessionID:  "invalid-shell-timeout-" + tc.name,
				InputItems: []InputItem{{Type: "text", Text: "try invalid timeout"}},
			})
			if err != nil {
				t.Fatalf("SubmitTurn returned error: %v", err)
			}
			payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
			if payload["status"] != "tool_request_invalid" || payload["executed"] != false {
				t.Fatalf("tool result payload = %+v, want repairable invalid timeout", payload)
			}
			projection, err := k.Session("invalid-shell-timeout-" + tc.name)
			if err != nil {
				t.Fatalf("Session returned error: %v", err)
			}
			if len(projection.Operations) != 0 {
				t.Fatalf("operations = %+v, want no effect for invalid timeout", projection.Operations)
			}
		})
	}
}

func TestSubmitTurnReportsToolInfrastructureFailureSeparately(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": echoCommand("hello"),
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider: &toolFeedbackProvider{
			calls: []ModelToolCall{
				{ToolCallID: "call_infra_failure", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
			},
			final: "should not be reached",
		},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	k.ledger = &failOnOperationLedger{}

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "tool-infrastructure-failure",
		InputItems: []InputItem{{Type: "text", Text: "run shell through failing ledger"}},
	})
	if err == nil {
		t.Fatal("SubmitTurn returned nil error for tool infrastructure failure")
	}
	projection, err := k.Session("tool-infrastructure-failure")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 0 {
		t.Fatalf("operations = %+v, want no command failure projection for infrastructure failure", projection.Operations)
	}
	if len(projection.Turns) != 1 || projection.Turns[0].Error == nil {
		t.Fatalf("turns = %+v, want failed turn with tool infrastructure error", projection.Turns)
	}
	if projection.Turns[0].Error.Code != "tool_infrastructure_failed" {
		t.Fatalf("turn error = %+v, want tool_infrastructure_failed", projection.Turns[0].Error)
	}
}

func TestSubmitTurnReturnsRepairFeedbackForUnsupportedModelToolCall(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{
				ToolCallID: "call_email",
				Name:       "email.send",
				Arguments:  json.RawMessage(`{"to":"someone@example.com"}`),
			},
		},
		final: "unsupported tool repair received",
	}
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  testTempDir(t),
		},
		Clock: func() time.Time {
			return time.Date(2026, 6, 22, 1, 2, 3, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "unsupported-tool-call",
		InputItems: []InputItem{{Type: "text", Text: "send email"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "unsupported tool repair received" {
		t.Fatalf("final text = %q, want unsupported tool repair received", resp.Final.Text)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	errorPayload := payload["error"].(map[string]interface{})
	if payload["status"] != "tool_request_invalid" || errorPayload["code"] != "unsupported_tool" {
		t.Fatalf("repair payload = %+v, want unsupported_tool", payload)
	}
	projection, err := k.Session("unsupported-tool-call")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 0 {
		t.Fatalf("operations = %+v, want no executed effects", projection.Operations)
	}
	if len(projection.Turns) != 1 || projection.Turns[0].Phase != RuntimePhaseEnded || projection.Turns[0].TerminalOutcome != TerminalOutcomeSucceeded {
		t.Fatalf("turns = %+v, want one completed turn after repair feedback", projection.Turns)
	}
	eventTypes := make([]string, 0, len(projection.Events))
	for _, event := range projection.Events {
		eventTypes = append(eventTypes, event.Type)
	}
	wantTypes := []string{"turn.submitted", "tool.call", "tool.result", "model.final"}
	if strings.Join(eventTypes, ",") != strings.Join(wantTypes, ",") {
		t.Fatalf("event types = %v, want %v", eventTypes, wantTypes)
	}
	if projection.Events[2].Data.ToolResult == nil || projection.Events[2].Data.ToolResult.ForEventID != projection.Events[1].EventID || projection.Events[2].Data.ToolResult.Status != "tool_request_invalid" {
		t.Fatalf("tool result event = %+v, want invalid request result linked to %s", projection.Events[2].Data.ToolResult, projection.Events[1].EventID)
	}
}

func TestSubmitTurnReturnsRepairFeedbackForMixedModelToolBatchBeforeAnyEffect(t *testing.T) {
	workspace := testTempDir(t)
	toolArgs, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": writeFileCommand("mixed-tool-effect.txt", "effect"),
	})
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_write", Name: "shell_exec", Arguments: json.RawMessage(toolArgs)},
			{ToolCallID: "call_email", Name: "email.send", Arguments: json.RawMessage(`{"to":"someone@example.com"}`)},
		},
		final: "mixed batch repair received",
	}
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "mixed-tool-batch",
		InputItems: []InputItem{{Type: "text", Text: "try mixed tools"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "mixed-tool-effect.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mixed batch created shell effect before rejecting unsupported tool; stat err=%v", err)
	}
	results := provider.Requests()[1].ToolRounds[0].Results
	if len(results) != 2 {
		t.Fatalf("tool results = %+v, want repair result for each call", results)
	}
	repairByCallID := toolRepairPayloadByCallID(t, results)
	writeError := repairByCallID["call_write"]["error"].(map[string]interface{})
	emailError := repairByCallID["call_email"]["error"].(map[string]interface{})
	if writeError["code"] != "tool_batch_not_executed" || emailError["code"] != "unsupported_tool" {
		t.Fatalf("repair payloads = %+v, want batch blocker plus unsupported tool", repairByCallID)
	}
	projection, err := k.Session("mixed-tool-batch")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 0 {
		t.Fatalf("operations = %+v, want no executed effects for mixed unsupported batch", projection.Operations)
	}
}

func TestSubmitTurnRejectsDuplicateToolCallIDBeforeAnyEffect(t *testing.T) {
	workspace := testTempDir(t)
	firstArgs, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": writeFileCommand("duplicate-first.txt", "first"),
	})
	if err != nil {
		t.Fatalf("marshal first args: %v", err)
	}
	secondArgs, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": writeFileCommand("duplicate-second.txt", "second"),
	})
	if err != nil {
		t.Fatalf("marshal second args: %v", err)
	}
	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider: &toolFeedbackProvider{
			calls: []ModelToolCall{
				{ToolCallID: "call_duplicate", Name: "shell_exec", Arguments: json.RawMessage(firstArgs)},
				{ToolCallID: "call_duplicate", Name: "shell_exec", Arguments: json.RawMessage(secondArgs)},
			},
			final: "should not be reached",
		},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "duplicate-tool-call-id",
		InputItems: []InputItem{{Type: "text", Text: "try duplicate tool call ids"}},
	})
	if !errors.Is(err, ErrModelToolCallRejected) {
		t.Fatalf("SubmitTurn error = %v, want ErrModelToolCallRejected", err)
	}
	for _, file := range []string{"duplicate-first.txt", "duplicate-second.txt"} {
		if _, err := os.Stat(filepath.Join(workspace, file)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("duplicate batch created %s before rejection; stat err=%v", file, err)
		}
	}
	projection, err := k.Session("duplicate-tool-call-id")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	var eventTypes []string
	for _, event := range projection.Events {
		eventTypes = append(eventTypes, event.Type)
	}
	wantTypes := []string{"turn.submitted", "turn.failed"}
	if strings.Join(eventTypes, ",") != strings.Join(wantTypes, ",") {
		t.Fatalf("event types = %v, want no tool.call before duplicate-id rejection", eventTypes)
	}
	if len(projection.Operations) != 0 {
		t.Fatalf("operations = %+v, want no shell operation for duplicate-id batch", projection.Operations)
	}
}

func TestSubmitTurnReturnsRepairFeedbackForUnknownModelToolArgumentFields(t *testing.T) {
	for _, field := range []string{
		"permission_mode",
		"authority_policy",
		"sandbox_profile",
		"approval_policy",
		"approval_id",
		"event_id",
		"operation_id",
		"lease_id",
		"task_id",
		"tool_call_event_id",
		"provider_tool_call_id",
	} {
		t.Run(field, func(t *testing.T) {
			workspace := testTempDir(t)
			arguments := json.RawMessage(`{"cwd":"` + filepath.ToSlash(workspace) + `","command":"` + writeFileCommand("unknown-arg-effect.txt", "effect") + `","` + field + `":"model-supplied"}`)
			ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
			provider := &toolFeedbackProvider{
				calls: []ModelToolCall{
					{
						ToolCallID: "call_unknown_arg_" + field,
						Name:       "shell_exec",
						Arguments:  arguments,
					},
				},
				final: "unknown argument repair received",
			}
			k, err := New(Config{
				LedgerPath:   ledgerPath,
				Provider:     provider,
				RuntimeToken: testRuntimeToken,
				ToolPolicy: ToolPolicy{
					PermissionMode: PermissionModeDefault,
					WorkspaceRoot:  workspace,
				},
			})
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}

			_, err = k.SubmitTurn(context.Background(), TurnRequest{
				SessionID:  "unknown-tool-arg-" + field,
				InputItems: []InputItem{{Type: "text", Text: "try unknown tool arg"}},
			})
			if err != nil {
				t.Fatalf("SubmitTurn returned error: %v", err)
			}
			if _, err := os.Stat(filepath.Join(workspace, "unknown-arg-effect.txt")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unknown argument call created shell effect before rejection; stat err=%v", err)
			}
			payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
			errorPayload := payload["error"].(map[string]interface{})
			if payload["status"] != "tool_request_invalid" || errorPayload["code"] != "invalid_tool_arguments" {
				t.Fatalf("repair payload = %+v, want invalid_tool_arguments", payload)
			}
			projection, err := k.Session("unknown-tool-arg-" + field)
			if err != nil {
				t.Fatalf("Session returned error: %v", err)
			}
			if len(projection.Operations) != 0 {
				t.Fatalf("operations = %+v, want no executed effects for unknown model tool argument", projection.Operations)
			}
		})
	}
}
