package kernel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeepSeekThinkingAdapterOmitsReasoningOnOrdinaryFollowUp(t *testing.T) {
	const (
		hiddenReasoning = "SECRET-CHAIN-OF-THOUGHT"
		firstAnswer     = "visible final answer"
		secondAnswer    = "second visible answer"
	)
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read provider request: %v", err)
		}
		requests = append(requests, string(body))
		w.Header().Set("Content-Type", "application/json")
		switch len(requests) {
		case 1:
			_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"` + firstAnswer + `","reasoning_content":"` + hiddenReasoning + `"}}],"usage":{"prompt_tokens":7,"completion_tokens":5,"total_tokens":12}}`))
		case 2:
			if !strings.Contains(string(body), firstAnswer) {
				t.Fatalf("second provider request = %s, want visible first answer in history", body)
			}
			if strings.Contains(string(body), hiddenReasoning) || strings.Contains(string(body), "reasoning_content") {
				t.Fatalf("ordinary DeepSeek continuation replayed reasoning: %s", body)
			}
			_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"` + secondAnswer + `"}}]}`))
		default:
			t.Fatalf("unexpected provider request %d: %s", len(requests), body)
		}
	}))
	defer server.Close()

	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider: NewOpenAICompatibleProvider(OpenAICompatibleConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "test-model",
			Adapter: ProviderAdapterBinding{
				AdapterID:         "deepseek",
				ProfileID:         "deepseek-v4-flash",
				TransportProtocol: "openai-chat-completions",
			},
		}),
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	first, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "reasoning-response-only",
		InputItems: []InputItem{{Type: "text", Text: "answer visibly"}},
	})
	if err != nil {
		t.Fatalf("first SubmitTurn returned error: %v", err)
	}
	if first.Final.Text != firstAnswer {
		t.Fatalf("first final = %q, want visible provider answer", first.Final.Text)
	}
	if first.Final.Usage == nil || first.Final.Usage.InputTokens != 7 || first.Final.Usage.OutputTokens != 5 || first.Final.Usage.TotalTokens != 12 {
		t.Fatalf("first usage = %+v, want normalized usage without reasoning replay", first.Final.Usage)
	}

	second, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "reasoning-response-only",
		InputItems: []InputItem{{Type: "text", Text: "continue"}},
	})
	if err != nil {
		t.Fatalf("second SubmitTurn returned error: %v", err)
	}
	if second.Final.Text != secondAnswer {
		t.Fatalf("second final = %q, want visible provider answer", second.Final.Text)
	}
	if len(requests) != 2 {
		t.Fatalf("provider request count = %d, want 2", len(requests))
	}

	storedEvents, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	if !strings.Contains(mustJSON(t, storedEvents), hiddenReasoning) {
		t.Fatalf("stored events = %#v, want durable canonical reasoning", storedEvents)
	}

	turnEvents, err := k.TurnEvents(first.TurnID)
	if err != nil {
		t.Fatalf("TurnEvents returned error: %v", err)
	}
	if !strings.Contains(mustJSON(t, turnEvents), hiddenReasoning) {
		t.Fatalf("turn events = %#v, want durable canonical reasoning", turnEvents)
	}

	session, err := k.Session("reasoning-response-only")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(session.Turns) == 0 || len(session.Turns[0].ReasoningMessages) != 1 || session.Turns[0].ReasoningMessages[0].Text != hiddenReasoning {
		t.Fatalf("session = %#v, want projected canonical reasoning", session)
	}

	timeline, err := k.UITimeline("reasoning-response-only")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	if !strings.Contains(mustJSON(t, timeline), hiddenReasoning) {
		t.Fatalf("timeline = %#v, want projected canonical reasoning", timeline)
	}

	audit, err := k.AuditReplay(first.TurnID)
	if err != nil {
		t.Fatalf("AuditReplay returned error: %v", err)
	}
	if !strings.Contains(mustJSON(t, audit), "model.reasoning") {
		t.Fatalf("audit replay = %#v, want reasoning event evidence", audit)
	}

	contextInspection, err := k.ContextInspection(first.TurnID)
	if err != nil {
		t.Fatalf("ContextInspection returned error: %v", err)
	}
	if strings.Contains(mustJSON(t, contextInspection), hiddenReasoning) {
		t.Fatalf("context inspection leaked reasoning: %#v", contextInspection)
	}

	providerContext, err := k.ProviderContextProjection(second.TurnID)
	if err != nil {
		t.Fatalf("ProviderContextProjection returned error: %v", err)
	}
	messages, err := chatMessagesFromModelRequestForAdapter(providerContext.ModelRequest(), ProviderAdapterBinding{
		AdapterID:         "deepseek",
		ProfileID:         "deepseek-v4-flash",
		TransportProtocol: "openai-chat-completions",
	})
	if err != nil {
		t.Fatalf("map provider context: %v", err)
	}
	contextJSON, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("marshal provider messages: %v", err)
	}
	if !strings.Contains(string(contextJSON), firstAnswer) {
		t.Fatalf("provider messages = %s, want visible first answer", contextJSON)
	}
	if strings.Contains(string(contextJSON), hiddenReasoning) {
		t.Fatalf("provider messages = %s, ordinary history must omit reasoning", contextJSON)
	}

	commandPayload := providerCommandRequest{
		Protocol:     providerCommandProtocol,
		SessionID:    providerContext.SessionID,
		TurnID:       providerContext.TurnID,
		Model:        "test-model",
		InputItems:   providerContext.ModelRequest().InputItems,
		ToolManifest: providerContext.ModelRequest().ToolManifest,
		ToolRounds:   providerCommandModelToolRounds(providerContext.ModelRequest().ToolRounds),
	}
	if strings.Contains(mustJSON(t, commandPayload), hiddenReasoning) {
		t.Fatalf("provider command request leaked reasoning: %#v", commandPayload)
	}
}

func TestZAIGLMThinkingAdapterRejectsToolReplayWithoutBoundReasoning(t *testing.T) {
	for _, tc := range []struct {
		name      string
		reasoning string
		adapterID string
		profileID string
	}{
		{name: "missing reasoning"},
		{name: "different binding", reasoning: "must not cross adapters", adapterID: "another-adapter", profileID: "another-profile"},
		{name: "case-distinct binding", reasoning: "must not cross adapters", adapterID: "ZAI-GLM", profileID: "GLM-5.2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requestCount++
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"unexpected"}}]}`))
			}))
			defer server.Close()

			provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
				BaseURL: server.URL,
				APIKey:  "test-key",
				Model:   "test-model",
				Adapter: ProviderAdapterBinding{
					AdapterID:         "zai-glm",
					ProfileID:         "glm-5.2",
					TransportProtocol: "openai-chat-completions",
				},
			})
			_, err := provider.Complete(context.Background(), ModelRequest{
				Conversation: []ModelConversationMessage{
					{
						Role:                    "assistant",
						ReasoningText:           tc.reasoning,
						ReasoningAdapterID:      tc.adapterID,
						ReasoningAdapterProfile: tc.profileID,
						ToolCalls: []ModelToolCall{{
							ToolCallID: "call_missing_reasoning",
							Name:       "shell_exec",
							Arguments:  json.RawMessage(`{}`),
						}},
					},
					{Role: "tool", ToolCallID: "call_missing_reasoning", Text: "tool result"},
				},
			})
			if err == nil {
				t.Fatal("Complete returned nil error for unavailable GLM continuation reasoning")
			}
			failure := providerFailureFromError(err)
			if failure.ReasonCode != "provider_reasoning_continuation_unavailable" || failure.Retryable {
				t.Fatalf("failure = %+v, want nonretryable provider_reasoning_continuation_unavailable", failure)
			}
			if requestCount != 0 {
				t.Fatalf("provider request count = %d, want no egress", requestCount)
			}
		})
	}
}

func TestDeepSeekThinkingAdapterOmitsReasoningForToolContinuation(t *testing.T) {
	const reasoning = "DEEPSEEK-REASONING-MUST-NOT-REPLAY"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		messages, ok := payload["messages"].([]any)
		if !ok || len(messages) != 2 {
			t.Fatalf("messages = %#v, want assistant tool call and tool result", payload["messages"])
		}
		assistant, _ := messages[0].(map[string]any)
		if _, replayed := assistant["reasoning_content"]; replayed {
			t.Fatalf("assistant = %#v, DeepSeek must not receive response-only reasoning", assistant)
		}
		_, _ = w.Write([]byte(`{"model":"deepseek-v4-flash","choices":[{"message":{"role":"assistant","content":"tool continuation complete"}}]}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "deepseek-v4-flash",
		Adapter: ProviderAdapterBinding{
			AdapterID:         "deepseek",
			ProfileID:         "deepseek-v4-flash",
			TransportProtocol: modelGatewayProtocolChatCompletions,
		},
	})
	_, err := provider.Complete(context.Background(), ModelRequest{Conversation: []ModelConversationMessage{
		{
			Role:                    "assistant",
			ReasoningText:           reasoning,
			ReasoningAdapterID:      "deepseek",
			ReasoningAdapterProfile: "deepseek-v4-flash",
			ToolCalls: []ModelToolCall{{
				ToolCallID: "call_deepseek",
				Name:       "shell_exec",
				Arguments:  json.RawMessage(`{"command":"pwd"}`),
			}},
		},
		{Role: "tool", ToolCallID: "call_deepseek", Text: "D:/workspace"},
	}})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
}

func TestDeepSeekThinkingAdapterSerializesEmptyContentForToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		var messages []map[string]json.RawMessage
		if err := json.Unmarshal(raw["messages"], &messages); err != nil {
			t.Fatalf("decode messages: %v", err)
		}
		if len(messages) != 2 {
			t.Fatalf("messages = %#v, want assistant tool call and tool result", messages)
		}
		if content, ok := messages[0]["content"]; !ok || string(content) != `""` {
			t.Fatalf("assistant content = %s, want serialized empty string", content)
		}
		_, _ = w.Write([]byte(`{"model":"deepseek-v4-flash","choices":[{"message":{"role":"assistant","content":"tool continuation complete"}}]}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "deepseek-v4-flash",
		Adapter: ProviderAdapterBinding{
			AdapterID:         "deepseek",
			ProfileID:         "deepseek-v4-flash",
			TransportProtocol: modelGatewayProtocolChatCompletions,
		},
	})
	_, err := provider.Complete(context.Background(), ModelRequest{Conversation: []ModelConversationMessage{
		{Role: "assistant", ToolCalls: []ModelToolCall{{ToolCallID: "call_deepseek", Name: "shell_exec", Arguments: json.RawMessage(`{"command":"pwd"}`)}}},
		{Role: "tool", ToolCallID: "call_deepseek", Text: "D:/workspace"},
	}})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
}

func TestConversationMessagesKeepEachToolRoundReasoning(t *testing.T) {
	events := []StoredEvent{
		{SessionID: "session", TurnID: "turn", Type: "turn.submitted", Data: EventData{InputItems: []InputItem{{Type: "text", Text: "work"}}}},
		{SessionID: "session", TurnID: "turn", Type: "model.reasoning", Data: EventData{Reasoning: &ReasoningMessage{Text: "first reasoning", AdapterID: "deepseek", AdapterProfileID: "deepseek-v4-flash"}}},
		{SessionID: "session", TurnID: "turn", Type: "tool.call", Data: EventData{ToolCall: &ToolCallProjection{ProviderToolCallID: "call_one", Tool: "shell_exec", Arguments: `{}`}}},
		{SessionID: "session", TurnID: "turn", Type: "tool.result", Data: EventData{ToolResult: &ToolResultProjection{ProviderToolCallID: "call_one", Content: "one"}}},
		{SessionID: "session", TurnID: "turn", Type: "model.reasoning", Data: EventData{Reasoning: &ReasoningMessage{Text: "second reasoning", AdapterID: "deepseek", AdapterProfileID: "deepseek-v4-flash"}}},
		{SessionID: "session", TurnID: "turn", Type: "tool.call", Data: EventData{ToolCall: &ToolCallProjection{ProviderToolCallID: "call_two", Tool: "shell_exec", Arguments: `{}`}}},
		{SessionID: "session", TurnID: "turn", Type: "tool.result", Data: EventData{ToolResult: &ToolResultProjection{ProviderToolCallID: "call_two", Content: "two"}}},
	}
	messages := conversationMessagesForTurns(events, "session", map[string]bool{"turn": true}, false)
	if len(messages) != 5 {
		t.Fatalf("messages = %#v, want user, assistant/tool for each round", messages)
	}
	if messages[1].ReasoningText != "first reasoning" || messages[3].ReasoningText != "second reasoning" {
		t.Fatalf("assistant tool messages = %#v, want distinct reasoning per round", messages)
	}
}

func TestOpenAICompatibleProviderRejectsVendorHiddenReasoningWithoutAdapterPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"visible answer","reasoning_content":"vendor hidden reasoning"}}]}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	_, err := provider.Complete(context.Background(), ModelRequest{
		InputItems: []ModelInputItem{{Kind: ModelInputKindUserText, Text: "answer visibly"}},
	})
	if err == nil {
		t.Fatal("Complete returned nil error for unsupported vendor hidden reasoning")
	}
	failure := providerFailureFromError(err)
	if failure.ReasonCode != "provider_vendor_field_unsupported" || failure.Retryable {
		t.Fatalf("failure = %+v, want nonretryable provider_vendor_field_unsupported", failure)
	}
}

func TestZAIGLMThinkingAdapterReplaysSameBoundReasoningForToolContinuation(t *testing.T) {
	const reasoning = "GLM-REASONING-TO-PRESERVE"
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		thinking, ok := payload["thinking"].(map[string]any)
		if !ok || thinking["type"] != "enabled" || thinking["clear_thinking"] != false {
			t.Fatalf("thinking = %#v, want preserved GLM thinking", payload["thinking"])
		}
		messages, ok := payload["messages"].([]any)
		if !ok || len(messages) != 2 {
			t.Fatalf("messages = %#v, want assistant tool call and tool result", payload["messages"])
		}
		assistant, _ := messages[0].(map[string]any)
		if assistant["reasoning_content"] != reasoning {
			t.Fatalf("assistant = %#v, want replayed reasoning", assistant)
		}
		if _, ok := assistant["tool_calls"]; !ok {
			t.Fatalf("assistant = %#v, want tool calls after reasoning", assistant)
		}
		_, _ = w.Write([]byte(`{"model":"glm-5.2","choices":[{"message":{"role":"assistant","content":"tool continuation complete"}}]}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "glm-5.2",
		Adapter: ProviderAdapterBinding{
			AdapterID:         "zai-glm",
			ProfileID:         "glm-5.2",
			TransportProtocol: modelGatewayProtocolChatCompletions,
		},
	})
	response, err := provider.Complete(context.Background(), ModelRequest{Conversation: []ModelConversationMessage{
		{
			Role:                    "assistant",
			ReasoningText:           reasoning,
			ReasoningAdapterID:      "zai-glm",
			ReasoningAdapterProfile: "glm-5.2",
			ToolCalls: []ModelToolCall{{
				ToolCallID: "call_glm",
				Name:       "shell_exec",
				Arguments:  json.RawMessage(`{"command":"pwd"}`),
			}},
		},
		{Role: "tool", ToolCallID: "call_glm", Text: "D:/workspace"},
	}})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if response.Text != "tool continuation complete" || requestCount != 1 {
		t.Fatalf("response = %+v, request count = %d", response, requestCount)
	}
}

func TestZAIGLMThinkingAdapterClearsReasoningOnOrdinaryFollowUp(t *testing.T) {
	const reasoning = "GLM-REASONING-LOCAL-ONLY"
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if requests == 2 {
			thinking, _ := payload["thinking"].(map[string]any)
			if thinking["clear_thinking"] != true {
				t.Fatalf("thinking = %#v, want ordinary GLM clear thinking", payload["thinking"])
			}
			if strings.Contains(mustJSON(t, payload["messages"]), reasoning) {
				t.Fatalf("ordinary GLM continuation replayed reasoning: %#v", payload["messages"])
			}
		}
		if requests == 1 {
			_, _ = w.Write([]byte(`{"model":"glm-5.2","choices":[{"message":{"role":"assistant","content":"visible answer","reasoning_content":"` + reasoning + `"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"model":"glm-5.2","choices":[{"message":{"role":"assistant","content":"later answer"}}]}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "glm-5.2",
		Adapter: ProviderAdapterBinding{
			AdapterID:         "zai-glm",
			ProfileID:         "glm-5.2",
			TransportProtocol: modelGatewayProtocolChatCompletions,
		},
	})
	first, err := provider.Complete(context.Background(), ModelRequest{InputItems: []ModelInputItem{{Kind: ModelInputKindUserText, Text: "answer"}}})
	if err != nil || first.Reasoning == nil || first.Reasoning.Text != reasoning {
		t.Fatalf("first response = %+v, err = %v", first, err)
	}
	second, err := provider.Complete(context.Background(), ModelRequest{Conversation: []ModelConversationMessage{
		{Role: "assistant", Text: first.Text, ReasoningText: first.Reasoning.Text, ReasoningAdapterID: "zai-glm", ReasoningAdapterProfile: "glm-5.2"},
		{Role: "user", Text: "continue"},
	}})
	if err != nil || second.Text != "later answer" || requests != 2 {
		t.Fatalf("second response = %+v, err = %v, requests = %d", second, err, requests)
	}
}

func TestZAIGLMThinkingAdapterRejectsToolReplayWithoutSameBoundReasoning(t *testing.T) {
	for _, tc := range []struct {
		name      string
		reasoning string
		adapterID string
		profileID string
	}{
		{name: "missing reasoning"},
		{name: "different profile", reasoning: "must not cross GLM profiles", adapterID: "zai-glm", profileID: "glm-5.1"},
		{name: "case-distinct binding", reasoning: "must not cross GLM bindings", adapterID: "ZAI-GLM", profileID: "GLM-5.2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requestCount++
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"model":"glm-5.2","choices":[{"message":{"role":"assistant","content":"unexpected"}}]}`))
			}))
			defer server.Close()

			provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
				BaseURL: server.URL,
				APIKey:  "test-key",
				Model:   "glm-5.2",
				Adapter: ProviderAdapterBinding{
					AdapterID:         "zai-glm",
					ProfileID:         "glm-5.2",
					TransportProtocol: modelGatewayProtocolChatCompletions,
				},
			})
			_, err := provider.Complete(context.Background(), ModelRequest{Conversation: []ModelConversationMessage{
				{
					Role:                    "assistant",
					ReasoningText:           tc.reasoning,
					ReasoningAdapterID:      tc.adapterID,
					ReasoningAdapterProfile: tc.profileID,
					ToolCalls: []ModelToolCall{{
						ToolCallID: "call_missing_glm_reasoning",
						Name:       "shell_exec",
						Arguments:  json.RawMessage(`{}`),
					}},
				},
				{Role: "tool", ToolCallID: "call_missing_glm_reasoning", Text: "tool result"},
			}})
			if err == nil {
				t.Fatal("Complete returned nil error for unavailable GLM continuation reasoning")
			}
			failure := providerFailureFromError(err)
			if failure.ReasonCode != "provider_reasoning_continuation_unavailable" || failure.Retryable {
				t.Fatalf("failure = %+v, want nonretryable provider_reasoning_continuation_unavailable", failure)
			}
			if requestCount != 0 {
				t.Fatalf("provider request count = %d, want no egress", requestCount)
			}
		})
	}
}

func TestZAIGLMThinkingAdapterStreamClearsReasoningOnOrdinaryFollowUp(t *testing.T) {
	const reasoning = "GLM-STREAM-REASONING-LOCAL-ONLY"
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		thinking, _ := payload["thinking"].(map[string]any)
		if thinking["type"] != "enabled" || thinking["clear_thinking"] != true {
			t.Fatalf("thinking = %#v, want ordinary GLM clear thinking", payload["thinking"])
		}
		if requestCount == 2 && strings.Contains(mustJSON(t, payload["messages"]), reasoning) {
			t.Fatalf("ordinary streamed GLM continuation replayed reasoning: %#v", payload["messages"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if requestCount == 1 {
			_, _ = w.Write([]byte("data: {\"model\":\"glm-5.2\",\"choices\":[{\"delta\":{\"reasoning_content\":\"" + reasoning + "\"}}]}\n\n"))
			_, _ = w.Write([]byte("data: {\"model\":\"glm-5.2\",\"choices\":[{\"delta\":{\"content\":\"first answer\"}}]}\n\n"))
		} else {
			_, _ = w.Write([]byte("data: {\"model\":\"glm-5.2\",\"choices\":[{\"delta\":{\"content\":\"second answer\"}}]}\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "glm-5.2",
		Adapter: ProviderAdapterBinding{
			AdapterID:         "zai-glm",
			ProfileID:         "glm-5.2",
			TransportProtocol: modelGatewayProtocolChatCompletions,
		},
	})
	var chunks []string
	first, err := provider.StreamComplete(context.Background(), ModelRequest{
		InputItems: []ModelInputItem{{Kind: ModelInputKindUserText, Text: "answer"}},
	}, func(delta ModelStreamDelta) error {
		chunks = append(chunks, delta.Text)
		return nil
	})
	if err != nil || first.Text != "first answer" || first.Reasoning == nil || first.Reasoning.Text != reasoning || strings.Join(chunks, "") != first.Text {
		t.Fatalf("first response = %+v, chunks = %#v, err = %v", first, chunks, err)
	}
	second, err := provider.StreamComplete(context.Background(), ModelRequest{Conversation: []ModelConversationMessage{
		{Role: "assistant", Text: first.Text, ReasoningText: first.Reasoning.Text, ReasoningAdapterID: "zai-glm", ReasoningAdapterProfile: "glm-5.2"},
		{Role: "user", Text: "continue"},
	}}, nil)
	if err != nil || second.Text != "second answer" || requestCount != 2 {
		t.Fatalf("second response = %+v, err = %v, requests = %d", second, err, requestCount)
	}
}

func TestZAIGLMThinkingAdapterStreamReplaysSameBoundReasoningForToolContinuation(t *testing.T) {
	const reasoning = "GLM-STREAM-REASONING-TO-PRESERVE"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		thinking, _ := payload["thinking"].(map[string]any)
		if thinking["type"] != "enabled" || thinking["clear_thinking"] != false {
			t.Fatalf("thinking = %#v, want preserved GLM thinking", payload["thinking"])
		}
		messages, _ := payload["messages"].([]any)
		assistant, _ := messages[0].(map[string]any)
		if len(messages) != 2 || assistant["reasoning_content"] != reasoning {
			t.Fatalf("messages = %#v, want same-bound reasoning before tool continuation", payload["messages"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"model\":\"glm-5.2\",\"choices\":[{\"delta\":{\"content\":\"stream tool continuation complete\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "glm-5.2",
		Adapter: ProviderAdapterBinding{
			AdapterID:         "zai-glm",
			ProfileID:         "glm-5.2",
			TransportProtocol: modelGatewayProtocolChatCompletions,
		},
	})
	response, err := provider.StreamComplete(context.Background(), ModelRequest{Conversation: []ModelConversationMessage{
		{
			Role:                    "assistant",
			ReasoningText:           reasoning,
			ReasoningAdapterID:      "zai-glm",
			ReasoningAdapterProfile: "glm-5.2",
			ToolCalls: []ModelToolCall{{
				ToolCallID: "call_stream_glm",
				Name:       "shell_exec",
				Arguments:  json.RawMessage(`{"command":"pwd"}`),
			}},
		},
		{Role: "tool", ToolCallID: "call_stream_glm", Text: "D:/workspace"},
	}}, nil)
	if err != nil || response.Text != "stream tool continuation complete" {
		t.Fatalf("response = %+v, err = %v", response, err)
	}
}

func mustJSON(t *testing.T, value interface{}) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal value: %v", err)
	}
	return string(encoded)
}
