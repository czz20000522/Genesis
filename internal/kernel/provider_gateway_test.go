package kernel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenAICompatibleProviderReadyRequiresConfiguration(t *testing.T) {
	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{})

	status := provider.Ready()
	if status.Readiness != ReadinessNotReady {
		t.Fatalf("status = %q, want blocked", status.Readiness)
	}
	if status.ReadinessReason == "" {
		t.Fatal("status reason is empty")
	}
}

func TestOpenAICompatibleProviderCompletesAgainstCompatibleServer(t *testing.T) {
	var sawAuth bool
	var sawPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		if r.Header.Get("Authorization") == "Bearer test-key" {
			sawAuth = true
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var req chatCompletionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if req.Model != "test-model" {
			t.Fatalf("model = %q, want test-model", req.Model)
		}
		if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != "hello\nworld" {
			t.Fatalf("messages = %+v, want one joined user message", req.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"provider answer"}}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	resp, err := provider.Complete(context.Background(), ModelRequest{
		InputItems: []ModelInputItem{
			{Kind: ModelInputKindUserText, Text: "hello"},
			{Kind: ModelInputKindUserText, Text: "world"},
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if sawPath != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", sawPath)
	}
	if !sawAuth {
		t.Fatal("provider did not send expected bearer token")
	}
	if resp.Text != "provider answer" || resp.Model != "served-model" {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Usage == nil || resp.Usage.InputTokens != 5 || resp.Usage.OutputTokens != 3 || resp.Usage.TotalTokens != 8 {
		t.Fatalf("usage = %+v, want normalized provider usage", resp.Usage)
	}
}

func TestOpenAICompatibleProviderNormalizesPromptCacheUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"provider answer"}}],"usage":{"prompt_tokens":1000,"completion_tokens":20,"total_tokens":1020,"prompt_cache_hit_tokens":750,"prompt_cache_miss_tokens":250}}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	resp, err := provider.Complete(context.Background(), ModelRequest{
		InputItems: []ModelInputItem{{Kind: ModelInputKindUserText, Text: "hello"}},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if resp.Usage == nil {
		t.Fatal("usage is nil")
	}
	if resp.Usage.CacheHitTokens != 750 || resp.Usage.CacheMissTokens != 250 {
		t.Fatalf("cache usage = hit %d miss %d, want 750/250", resp.Usage.CacheHitTokens, resp.Usage.CacheMissTokens)
	}
}

func TestCommandProviderCompletesFromTypedStdoutEvent(t *testing.T) {
	provider := NewCommandProvider(ProviderCommandConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestProviderCommandAdapterHelper", "--", "final"},
		Model:          "command-model",
		RequestTimeout: 5 * time.Second,
		Env:            []string{"GENESIS_PROVIDER_COMMAND_HELPER=1"},
	})
	status := provider.Ready()
	if status.Readiness != ReadinessReady || status.Name != "provider_command" {
		t.Fatalf("ready = %+v, want ok provider_command", status)
	}

	resp, err := provider.Complete(context.Background(), ModelRequest{
		SessionID: "command-session",
		TurnID:    "turn-command",
		InputItems: []ModelInputItem{
			{Kind: ModelInputKindUserText, Text: "hello command provider"},
		},
		ToolManifest: []ToolSpec{{
			Name:            "shell_exec",
			Description:     "execute a governed shell command",
			InputSchema:     map[string]interface{}{"type": "object"},
			SideEffectLevel: "write",
			ExecutionKind:   "sandboxed_process",
		}},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if resp.Text != "command final: hello command provider" || resp.Model != "command-model" {
		t.Fatalf("response = %+v, want command final from configured model", resp)
	}
	if resp.Usage == nil || resp.Usage.InputTokens != 7 || resp.Usage.OutputTokens != 3 || resp.Usage.TotalTokens != 10 {
		t.Fatalf("usage = %+v, want normalized command usage", resp.Usage)
	}
}

func TestCommandProviderRejectsInvalidAdapterResults(t *testing.T) {
	for _, tc := range []struct {
		mode      string
		wantError string
	}{
		{mode: "bad-json", wantError: "decode provider command response"},
		{mode: "unknown-kind", wantError: "unknown kind"},
		{mode: "missing-final-text", wantError: "final response missing text"},
		{mode: "missing-tool-name", wantError: "tool call missing name"},
		{mode: "exit-nonzero", wantError: "provider command failed"},
		{mode: "oversized-stdout", wantError: "stdout exceeded"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			provider := NewCommandProvider(ProviderCommandConfig{
				Command:        os.Args[0],
				Args:           []string{"-test.run=TestProviderCommandAdapterHelper", "--", tc.mode},
				Model:          "command-model",
				RequestTimeout: 5 * time.Second,
				Env:            []string{"GENESIS_PROVIDER_COMMAND_HELPER=1"},
			})
			_, err := provider.Complete(context.Background(), ModelRequest{
				SessionID:  "command-session",
				TurnID:     "turn-command",
				InputItems: []ModelInputItem{{Kind: ModelInputKindUserText, Text: "hello command provider"}},
				ToolManifest: []ToolSpec{{
					Name:            "shell_exec",
					Description:     "execute a governed shell command",
					InputSchema:     map[string]interface{}{"type": "object"},
					SideEffectLevel: "write",
					ExecutionKind:   "sandboxed_process",
				}},
			})
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("Complete error = %v, want substring %q", err, tc.wantError)
			}
		})
	}
}

func TestCommandProviderDoesNotInheritDaemonEnvironment(t *testing.T) {
	t.Setenv("GENESIS_PROVIDER_COMMAND_SENTINEL", "leaked")
	provider := NewCommandProvider(ProviderCommandConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestProviderCommandAdapterHelper", "--", "env-default-clean"},
		Model:          "command-model",
		RequestTimeout: 5 * time.Second,
	})
	resp, err := provider.Complete(context.Background(), commandProviderTestRequest())
	if err != nil {
		t.Fatalf("Complete with default env returned error: %v", err)
	}
	if resp.Text != "env default clean" {
		t.Fatalf("default env response = %q, want env default clean", resp.Text)
	}

	provider = NewCommandProvider(ProviderCommandConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestProviderCommandAdapterHelper", "--", "env-clean"},
		Model:          "command-model",
		RequestTimeout: 5 * time.Second,
		Env:            []string{"GENESIS_PROVIDER_COMMAND_HELPER=1"},
	})
	if _, err := provider.Complete(context.Background(), commandProviderTestRequest()); err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	provider = NewCommandProvider(ProviderCommandConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestProviderCommandAdapterHelper", "--", "env-explicit"},
		Model:          "command-model",
		RequestTimeout: 5 * time.Second,
		Env: []string{
			"GENESIS_PROVIDER_COMMAND_HELPER=1",
			"GENESIS_PROVIDER_COMMAND_SENTINEL=explicit",
		},
	})
	if _, err := provider.Complete(context.Background(), commandProviderTestRequest()); err != nil {
		t.Fatalf("Complete with explicit env returned error: %v", err)
	}
}

func TestProviderCommandFailureRedactsStderrFromTurnAndHTTP(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	provider := NewCommandProvider(ProviderCommandConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestProviderCommandAdapterHelper", "--", "stderr-secret"},
		Model:          "command-model",
		RequestTimeout: 5 * time.Second,
		Env:            []string{"GENESIS_PROVIDER_COMMAND_HELPER=1"},
	})
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := postJSONWithAuth(server.URL+"/turn", []byte(`{"session_id":"provider-command-secret-stderr","input_items":[{"type":"text","text":"trigger provider command stderr"}]}`))
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	for _, leaked := range []string{"sk-provider-stderr", "tokentest123456", "sk-jsonstderr"} {
		if strings.Contains(string(responseBody), leaked) {
			t.Fatalf("HTTP response leaked %q: %s", leaked, string(responseBody))
		}
	}
	if !strings.Contains(string(responseBody), "[REDACTED]") {
		t.Fatalf("HTTP response = %s, want redaction marker", string(responseBody))
	}

	projection, err := k.Session("provider-command-secret-stderr")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	sessionJSON, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("marshal session projection: %v", err)
	}
	ledgerData, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	for _, leaked := range []string{"sk-provider-stderr", "tokentest123456", "sk-jsonstderr"} {
		if strings.Contains(string(sessionJSON), leaked) {
			t.Fatalf("session projection leaked %q: %s", leaked, string(sessionJSON))
		}
		if strings.Contains(string(ledgerData), leaked) {
			t.Fatalf("ledger leaked %q: %s", leaked, string(ledgerData))
		}
	}
	if !strings.Contains(string(sessionJSON), "[REDACTED]") || !strings.Contains(string(ledgerData), "[REDACTED]") {
		t.Fatalf("session/ledger missing redaction marker: session=%s ledger=%s", string(sessionJSON), string(ledgerData))
	}
}

func TestProviderCommandRequestOmitsKernelEventIdentity(t *testing.T) {
	provider := NewCommandProvider(ProviderCommandConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestProviderCommandAdapterHelper", "--", "no-kernel-id-round"},
		Model:          "command-model",
		RequestTimeout: 5 * time.Second,
		Env:            []string{"GENESIS_PROVIDER_COMMAND_HELPER=1"},
	})
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "provider-command-model-visible-round",
		InputItems: []InputItem{{Type: "text", Text: "request an unsupported external tool"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "provider command saw model-visible tool round" {
		t.Fatalf("final text = %q, want provider command saw model-visible tool round", resp.Final.Text)
	}
	projection, err := k.Session("provider-command-model-visible-round")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Events) < 4 || projection.Events[1].Data.ToolCall == nil || projection.Events[2].Data.ToolResult == nil {
		t.Fatalf("events = %+v, want ledger tool call/result events", projection.Events)
	}
	if projection.Events[1].Data.ToolCall.ToolCallEventID == "" || projection.Events[2].Data.ToolResult.ForEventID != projection.Events[1].EventID {
		t.Fatalf("ledger projections lost kernel event identity: call=%+v result=%+v", projection.Events[1].Data.ToolCall, projection.Events[2].Data.ToolResult)
	}
}

func TestCommandProviderAppliesDefaultTimeout(t *testing.T) {
	provider := NewCommandProvider(ProviderCommandConfig{
		Command: os.Args[0],
		Model:   "command-model",
	})
	if provider.requestTimeout != defaultProviderCommandTimeout {
		t.Fatalf("request timeout = %s, want %s", provider.requestTimeout, defaultProviderCommandTimeout)
	}
}

func TestCommandProviderToolLoopThroughKernel(t *testing.T) {
	workspace := testTempDir(t)
	toolCommand := writeFileCommand("command-provider-tool.txt", "command-tool-value")
	toolArgs, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": toolCommand,
	})
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}
	provider := NewCommandProvider(ProviderCommandConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestProviderCommandAdapterHelper", "--", "tool-loop", string(toolArgs)},
		Model:          "command-model",
		RequestTimeout: 5 * time.Second,
		Env:            []string{"GENESIS_PROVIDER_COMMAND_HELPER=1"},
	})

	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
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

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "command-provider-tool-loop",
		InputItems: []InputItem{{Type: "text", Text: "write through command provider"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "command provider saw tool status completed" {
		t.Fatalf("final text = %q, want command provider tool completion", resp.Final.Text)
	}
	fileContent, err := os.ReadFile(filepath.Join(workspace, "command-provider-tool.txt"))
	if err != nil {
		t.Fatalf("read tool output file: %v", err)
	}
	if string(fileContent) != "command-tool-value" {
		t.Fatalf("tool output file = %q, want command-tool-value", string(fileContent))
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
	if !ok || toolCallData.ToolCall == nil || toolCallData.ToolCall.Tool != "shell_exec" {
		t.Fatalf("tool.call event = %#v, want shell_exec payload", events[1].Data)
	}
	toolResultData, ok := events[4].Data.(EventData)
	if !ok || toolResultData.ToolResult == nil || toolResultData.ToolResult.ForEventID != events[1].EventID {
		t.Fatalf("tool.result event = %#v, want link to %s", events[4].Data, events[1].EventID)
	}
}

func TestCommandProviderMalformedArgumentsReturnRepairFeedback(t *testing.T) {
	workspace := testTempDir(t)
	provider := NewCommandProvider(ProviderCommandConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestProviderCommandAdapterHelper", "--", "malformed-tool-args"},
		Model:          "command-model",
		RequestTimeout: 5 * time.Second,
		Env:            []string{"GENESIS_PROVIDER_COMMAND_HELPER=1"},
	})

	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
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
		SessionID:  "command-provider-malformed-args",
		InputItems: []InputItem{{Type: "text", Text: "try malformed command provider args"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "command provider saw repair invalid_tool_arguments" {
		t.Fatalf("final text = %q, want command provider repair feedback", resp.Final.Text)
	}
	projection, err := k.Session("command-provider-malformed-args")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 0 {
		t.Fatalf("operations = %+v, want no shell effect for malformed provider-command arguments", projection.Operations)
	}
	if len(projection.Events) != 4 {
		t.Fatalf("events = %+v, want turn/tool call/tool result/final", projection.Events)
	}
	if projection.Events[1].Data.ToolCall == nil || projection.Events[1].Data.ToolCall.ProviderToolCallID != "call_bad_command_provider_args" {
		t.Fatalf("tool.call = %+v, want provider correlation", projection.Events[1].Data.ToolCall)
	}
	if projection.Events[2].Data.ToolResult == nil ||
		projection.Events[2].Data.ToolResult.ProviderToolCallID != "call_bad_command_provider_args" ||
		projection.Events[2].Data.ToolResult.ToolCallEventID != projection.Events[1].EventID ||
		projection.Events[2].Data.ToolResult.ForEventID != projection.Events[1].EventID ||
		projection.Events[2].Data.ToolResult.Status != "tool_request_invalid" {
		t.Fatalf("tool.result = %+v, want repair linked to tool.call", projection.Events[2].Data.ToolResult)
	}
}

func TestOpenAICompatibleMalformedToolArgumentsReturnRepairFeedback(t *testing.T) {
	callCount := 0
	var repairContent string
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
		w.Header().Set("Content-Type", "application/json")
		switch callCount {
		case 1:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"model": "served-model",
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role": "assistant",
							"tool_calls": []interface{}{
								map[string]interface{}{
									"id":   "call_bad_json",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "shell_exec",
										"arguments": `{"command":`,
									},
								},
							},
						},
					},
				},
			})
		case 2:
			messages, ok := req["messages"].([]interface{})
			if !ok || len(messages) != 3 {
				t.Fatalf("second request messages = %#v, want user, assistant tool call, tool result", req["messages"])
			}
			toolMessage, ok := messages[2].(map[string]interface{})
			if !ok || toolMessage["tool_call_id"] != "call_bad_json" {
				t.Fatalf("tool message = %#v, want repair for call_bad_json", messages[2])
			}
			repairContent, _ = toolMessage["content"].(string)
			payload := decodeJSONMap(t, repairContent)
			errorPayload, _ := payload["error"].(map[string]interface{})
			if payload["status"] != "tool_request_invalid" || errorPayload["code"] != "invalid_tool_arguments" {
				t.Fatalf("repair payload = %+v, want invalid_tool_arguments", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"model": "served-model",
				"choices": []interface{}{
					map[string]interface{}{"message": map[string]interface{}{"role": "assistant", "content": "malformed args repaired"}},
				},
			})
		default:
			t.Fatalf("unexpected provider call %d", callCount)
		}
	}))
	defer server.Close()

	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.jsonl"),
		Provider: NewOpenAICompatibleProvider(OpenAICompatibleConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "test-model",
		}),
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  testTempDir(t),
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "openai-malformed-tool-args",
		InputItems: []InputItem{{Type: "text", Text: "try malformed tool args"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "malformed args repaired" {
		t.Fatalf("final text = %q, want malformed args repaired", resp.Final.Text)
	}
	if repairContent == "" {
		t.Fatal("provider did not receive repair feedback")
	}
	projection, err := k.Session("openai-malformed-tool-args")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 0 {
		t.Fatalf("operations = %+v, want no shell operation for malformed args", projection.Operations)
	}
	var eventTypes []string
	for _, event := range projection.Events {
		eventTypes = append(eventTypes, event.Type)
	}
	wantTypes := []string{"turn.submitted", "tool.call", "tool.result", "model.final"}
	if strings.Join(eventTypes, ",") != strings.Join(wantTypes, ",") {
		t.Fatalf("event types = %v, want %v", eventTypes, wantTypes)
	}
	if projection.Events[2].Data.ToolResult == nil || projection.Events[2].Data.ToolResult.Status != "tool_request_invalid" || projection.Events[2].Data.ToolResult.ForEventID != projection.Events[1].EventID {
		t.Fatalf("tool.result = %+v, want invalid repair linked to tool.call", projection.Events[2].Data.ToolResult)
	}
}

func TestLiveOpenAICompatibleProviderThroughKernel(t *testing.T) {
	if os.Getenv("GENESIS_LIVE_PROVIDER") != "1" {
		t.Skip("set GENESIS_LIVE_PROVIDER=1 to run the Genesis model config live provider smoke")
	}
	providerConfig, err := ResolveOpenAICompatibleConfigFromGenesis(GenesisModelConfigRequest{
		ConfigRoot:          os.Getenv("GENESIS_CONFIG_ROOT"),
		CredentialStoreRoot: os.Getenv("GENESIS_CREDENTIAL_STORE_ROOT"),
		ModelRole:           os.Getenv("GENESIS_MODEL_ROLE"),
		ModelProfileID:      os.Getenv("GENESIS_MODEL_PROFILE_ID"),
	})
	if err != nil {
		t.Fatalf("Genesis model config live smoke blocked: %s", ProviderConfigReason(err))
	}

	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     NewOpenAICompatibleProvider(providerConfig),
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ready := k.Ready()
	if ready.Readiness != ReadinessReady {
		t.Fatalf("ready = %+v, want ok", ready)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	resp, err := k.SubmitTurn(ctx, TurnRequest{
		SessionID:  "live-provider-smoke",
		InputItems: []InputItem{{Type: "text", Text: "Reply with a short confirmation that Genesis live provider smoke succeeded."}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if strings.TrimSpace(resp.Final.Text) == "" {
		t.Fatal("live provider returned empty final text")
	}
	projection, err := k.Session("live-provider-smoke")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 || projection.Turns[0].Status != "completed" {
		t.Fatalf("projection turns = %+v, want one completed turn", projection.Turns)
	}
}

func TestLiveOpenAICompatibleProviderToolLoopThroughKernel(t *testing.T) {
	if os.Getenv("GENESIS_LIVE_PROVIDER") != "1" {
		t.Skip("set GENESIS_LIVE_PROVIDER=1 to run the Genesis model config live provider tool-loop smoke")
	}
	providerConfig, err := ResolveOpenAICompatibleConfigFromGenesis(GenesisModelConfigRequest{
		ConfigRoot:          os.Getenv("GENESIS_CONFIG_ROOT"),
		CredentialStoreRoot: os.Getenv("GENESIS_CREDENTIAL_STORE_ROOT"),
		ModelRole:           os.Getenv("GENESIS_MODEL_ROLE"),
		ModelProfileID:      os.Getenv("GENESIS_MODEL_PROFILE_ID"),
	})
	if err != nil {
		t.Fatalf("Genesis model config live tool-loop smoke blocked: %s", ProviderConfigReason(err))
	}

	workspace := testTempDir(t)
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     NewOpenAICompatibleProvider(providerConfig),
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ready := k.Ready()
	if ready.Readiness != ReadinessReady {
		t.Fatalf("ready = %+v, want ok", ready)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := k.SubmitTurn(ctx, TurnRequest{
		SessionID: "live-provider-tool-loop-smoke",
		InputItems: []InputItem{{
			Type: "text",
			Text: "You must call the available tool named shell_exec with JSON arguments {\"command\":\"echo GENESIS_LIVE_TOOL_LOOP_OK\"}. After the tool result is returned, reply exactly GENESIS_LIVE_TOOL_LOOP_OK.",
		}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if !strings.Contains(resp.Final.Text, "GENESIS_LIVE_TOOL_LOOP_OK") {
		t.Fatalf("final text = %q, want live tool loop marker", resp.Final.Text)
	}
	projection, err := k.Session("live-provider-tool-loop-smoke")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 || projection.Turns[0].Status != "completed" {
		t.Fatalf("projection turns = %+v, want one completed turn", projection.Turns)
	}
	if len(projection.Operations) != 1 {
		t.Fatalf("operations = %+v, want one shell operation", projection.Operations)
	}
	operation := projection.Operations[0]
	if operation.Tool != "shell_exec" || operation.Status != "completed" || !strings.Contains(operation.Stdout, "GENESIS_LIVE_TOOL_LOOP_OK") {
		t.Fatalf("operation = %+v, want completed canonical shell_exec with marker stdout", operation)
	}
	events, err := k.TurnEvents(resp.TurnID)
	if err != nil {
		t.Fatalf("TurnEvents returned error: %v", err)
	}
	eventTypes := make([]string, 0, len(events))
	for _, event := range events {
		eventTypes = append(eventTypes, event.Type)
	}
	joined := strings.Join(eventTypes, ",")
	for _, want := range []string{"tool.call", "operation.completed", "tool.result", "model.final"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("turn event types = %v, want %s", eventTypes, want)
		}
	}
}

func TestProviderCommandAdapterHelper(t *testing.T) {
	mode := providerCommandHelperMode()
	if os.Getenv("GENESIS_PROVIDER_COMMAND_HELPER") != "1" && mode != "env-default-clean" {
		return
	}
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatalf("read stdin: %v", err)
	}
	var req providerCommandRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		t.Fatalf("decode provider command request: %v", err)
	}
	if req.Protocol != providerCommandProtocol {
		t.Fatalf("protocol = %q, want %s", req.Protocol, providerCommandProtocol)
	}
	if req.SessionID == "" || req.TurnID == "" {
		t.Fatalf("missing session/turn in provider command request: %+v", req)
	}
	if len(req.InputItems) == 0 || req.InputItems[0].Kind != ModelInputKindUserText {
		t.Fatalf("input items = %+v, want user_text", req.InputItems)
	}
	if len(req.ToolManifest) == 0 || req.ToolManifest[0].Name != "shell_exec" {
		t.Fatalf("tool manifest = %+v, want shell_exec", req.ToolManifest)
	}

	switch mode {
	case "final":
		writeProviderCommandHelperResponse(t, providerCommandResponse{
			Kind:  providerCommandResponseKindFinal,
			Model: req.Model,
			Text:  "command final: " + req.InputItems[0].Text,
			Usage: &TokenUsage{InputTokens: 7, OutputTokens: 3, TotalTokens: 10},
		})
	case "env-clean":
		if value := os.Getenv("GENESIS_PROVIDER_COMMAND_SENTINEL"); value != "" {
			t.Fatalf("provider command inherited daemon sentinel env %q", value)
		}
		writeProviderCommandHelperResponse(t, providerCommandResponse{
			Kind:  providerCommandResponseKindFinal,
			Model: req.Model,
			Text:  "env clean",
		})
	case "env-default-clean":
		if value := os.Getenv("GENESIS_PROVIDER_COMMAND_SENTINEL"); value != "" {
			t.Fatalf("provider command inherited daemon sentinel env %q", value)
		}
		writeProviderCommandHelperResponse(t, providerCommandResponse{
			Kind:  providerCommandResponseKindFinal,
			Model: req.Model,
			Text:  "env default clean",
		})
	case "env-explicit":
		if value := os.Getenv("GENESIS_PROVIDER_COMMAND_SENTINEL"); value != "explicit" {
			t.Fatalf("provider command explicit env = %q, want explicit", value)
		}
		writeProviderCommandHelperResponse(t, providerCommandResponse{
			Kind:  providerCommandResponseKindFinal,
			Model: req.Model,
			Text:  "env explicit",
		})
	case "bad-json":
		_, _ = os.Stdout.WriteString("not-json\n")
		os.Exit(0)
	case "unknown-kind":
		writeProviderCommandHelperResponse(t, providerCommandResponse{
			Kind:  "surprise",
			Model: req.Model,
		})
	case "missing-final-text":
		writeProviderCommandHelperResponse(t, providerCommandResponse{
			Kind:  providerCommandResponseKindFinal,
			Model: req.Model,
		})
	case "missing-tool-name":
		writeProviderCommandHelperResponse(t, providerCommandResponse{
			Kind:  providerCommandResponseKindToolCalls,
			Model: req.Model,
			ToolCalls: []ModelToolCall{{
				ToolCallID: "call_missing_name",
				Arguments:  json.RawMessage("{}"),
			}},
		})
	case "exit-nonzero":
		_, _ = os.Stderr.WriteString("adapter failed deliberately\n")
		os.Exit(3)
	case "stderr-secret":
		_, _ = os.Stderr.WriteString("GENESIS_PROVIDER_API_KEY=sk-provider-stderr\nAuthorization: Bearer tokentest123456\n{\"api_key\":\"sk-jsonstderr\"}\n")
		os.Exit(3)
	case "oversized-stdout":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", maxProviderCommandOutputBytes+1))
		os.Exit(0)
	case "tool-loop":
		if len(req.ToolRounds) == 0 {
			toolArgs := os.Args[len(os.Args)-1]
			writeProviderCommandHelperResponse(t, providerCommandResponse{
				Kind:  providerCommandResponseKindToolCalls,
				Model: req.Model,
				ToolCalls: []ModelToolCall{{
					ToolCallID: "call_command_provider_write",
					Name:       "shell_exec",
					Arguments:  json.RawMessage(toolArgs),
				}},
			})
			return
		}
		if len(req.ToolRounds[0].Results) != 1 {
			t.Fatalf("tool rounds = %+v, want one result", req.ToolRounds)
		}
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(req.ToolRounds[0].Results[0].Content), &result); err != nil {
			t.Fatalf("decode tool result: %v", err)
		}
		status, _ := result["status"].(string)
		writeProviderCommandHelperResponse(t, providerCommandResponse{
			Kind:  providerCommandResponseKindFinal,
			Model: req.Model,
			Text:  "command provider saw tool status " + status,
		})
	case "malformed-tool-args":
		if len(req.ToolRounds) == 0 {
			writeProviderCommandHelperResponse(t, providerCommandResponse{
				Kind:  providerCommandResponseKindToolCalls,
				Model: req.Model,
				ToolCalls: []ModelToolCall{{
					ToolCallID: "call_bad_command_provider_args",
					Name:       "shell_exec",
					Arguments:  json.RawMessage(`{"command":`),
				}},
			})
			return
		}
		if len(req.ToolRounds) != 1 || len(req.ToolRounds[0].Calls) != 1 || len(req.ToolRounds[0].Results) != 1 {
			t.Fatalf("tool rounds = %+v, want malformed call plus one repair result", req.ToolRounds)
		}
		call := req.ToolRounds[0].Calls[0]
		result := req.ToolRounds[0].Results[0]
		if call.ToolCallID != "call_bad_command_provider_args" || call.ToolCallEventID != "" || string(call.Arguments) != `{"command":` {
			t.Fatalf("tool round call = %+v arguments=%q, want provider echo plus raw malformed arguments without event id", call, string(call.Arguments))
		}
		if result.ToolCallID != "call_bad_command_provider_args" || result.ToolCallEventID != "" {
			t.Fatalf("tool round result = %+v, want provider echo without event id", result)
		}
		payload := decodeJSONMap(t, result.Content)
		errorPayload, _ := payload["error"].(map[string]interface{})
		code, _ := errorPayload["code"].(string)
		if payload["status"] != "tool_request_invalid" || code != "invalid_tool_arguments" {
			t.Fatalf("repair payload = %+v, want invalid_tool_arguments", payload)
		}
		writeProviderCommandHelperResponse(t, providerCommandResponse{
			Kind:  providerCommandResponseKindFinal,
			Model: req.Model,
			Text:  "command provider saw repair " + code,
		})
	case "no-kernel-id-round":
		if len(req.ToolRounds) == 0 {
			writeProviderCommandHelperResponse(t, providerCommandResponse{
				Kind:  providerCommandResponseKindToolCalls,
				Model: req.Model,
				ToolCalls: []ModelToolCall{{
					ToolCallID: "call_provider_visible",
					Name:       "unknown_external_tool",
					Arguments:  json.RawMessage(`{}`),
				}},
			})
			return
		}
		rawRequest := string(payload)
		for _, forbidden := range []string{
			"tool_call_event_id",
			"event_id",
			"operation_id",
			"lease_id",
			"permission_mode",
			"authority_policy",
			"sandbox_profile",
			"approval_policy",
			"for_event_id",
		} {
			if strings.Contains(rawRequest, forbidden) {
				t.Fatalf("provider command request leaked kernel-owned field %q: %s", forbidden, rawRequest)
			}
		}
		if !strings.Contains(rawRequest, `"tool_call_id":"call_provider_visible"`) {
			t.Fatalf("provider command request = %s, want provider-visible tool_call_id", rawRequest)
		}
		if len(req.ToolRounds) != 1 || len(req.ToolRounds[0].Calls) != 1 || len(req.ToolRounds[0].Results) != 1 {
			t.Fatalf("tool rounds = %+v, want one model-visible call and result", req.ToolRounds)
		}
		call := req.ToolRounds[0].Calls[0]
		result := req.ToolRounds[0].Results[0]
		if call.ToolCallEventID != "" || result.ToolCallEventID != "" {
			t.Fatalf("tool round = %+v / %+v, want no kernel event identity", call, result)
		}
		if call.ToolCallID != "call_provider_visible" || result.ToolCallID != "call_provider_visible" {
			t.Fatalf("tool round = %+v / %+v, want provider-visible id preserved", call, result)
		}
		resultPayload := decodeJSONMap(t, result.Content)
		if resultPayload["status"] != "tool_request_invalid" {
			t.Fatalf("result content = %+v, want repair feedback", resultPayload)
		}
		writeProviderCommandHelperResponse(t, providerCommandResponse{
			Kind:  providerCommandResponseKindFinal,
			Model: req.Model,
			Text:  "provider command saw model-visible tool round",
		})
	default:
		t.Fatalf("unknown helper mode %q args=%v", mode, os.Args)
	}
}

func TestHTTPReportsBlockedProvider(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     NewOpenAICompatibleProvider(OpenAICompatibleConfig{}),
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	readyResp, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer readyResp.Body.Close()
	var ready ReadyResponse
	if err := json.NewDecoder(readyResp.Body).Decode(&ready); err != nil {
		t.Fatalf("decode ready response: %v", err)
	}
	if ready.Readiness != ReadinessNotReady || ready.Provider.Readiness != ReadinessNotReady {
		t.Fatalf("ready = %+v, want blocked provider", ready)
	}

	body := []byte(`{"session_id":"blocked-session","input_items":[{"type":"text","text":"hello"}]}`)
	turnResp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer turnResp.Body.Close()
	if turnResp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("turn status = %d, want 503", turnResp.StatusCode)
	}

	restarted := newTestKernelWithRuntimeToken(t, ledgerPath, testRuntimeToken)
	projection, err := restarted.Session("blocked-session")
	if err != nil {
		t.Fatalf("Session after provider failure returned error: %v", err)
	}
	if len(projection.Turns) != 1 {
		t.Fatalf("len(Turns) = %d, want 1", len(projection.Turns))
	}
	if projection.Turns[0].Status != "failed" {
		t.Fatalf("turn status = %q, want failed", projection.Turns[0].Status)
	}
	if projection.Turns[0].Error == nil || projection.Turns[0].Error.Code != "provider_unavailable" {
		t.Fatalf("turn error = %+v, want provider_unavailable", projection.Turns[0].Error)
	}
	if len(projection.Events) != 3 || projection.Events[0].Type != "turn.submitted" || projection.Events[1].Type != "model.provider_attempt" || projection.Events[2].Type != "turn.failed" {
		t.Fatalf("events = %+v, want submitted, provider attempt, then failed", projection.Events)
	}
}
