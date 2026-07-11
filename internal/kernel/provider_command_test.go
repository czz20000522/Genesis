package kernel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommandProviderStrictlyRejectsUnknownResponseFields(t *testing.T) {
	for _, tc := range []struct {
		mode      string
		wantError string
	}{
		{mode: "extra-final-field", wantError: "unknown field"},
		{mode: "tool-lease-id", wantError: "unknown field"},
		{mode: "tool-budget-lease-id", wantError: "unknown field"},
		{mode: "tool-unknown-field", wantError: "unknown field"},
		{mode: "empty-reasoning", wantError: "reasoning missing text"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			provider := strictProviderCommandHelper(t, tc.mode)
			_, err := provider.Complete(context.Background(), providerCommandStrictTestRequest())
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("Complete error = %v, want substring %q", err, tc.wantError)
			}
		})
	}
}

func TestCommandProviderStrictShapeFailureDoesNotAdmitToolCall(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	k.provider = strictProviderCommandHelper(t, "tool-lease-id")

	_, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "provider-command-strict-e2e",
		InputItems: []InputItem{{Type: "text", Text: "try hidden lease"}},
	})
	if err == nil || !strings.Contains(err.Error(), "provider command") {
		t.Fatalf("SubmitTurn error = %v, want provider command shape failure", err)
	}
	projection, sessionErr := k.Session("provider-command-strict-e2e")
	if sessionErr != nil {
		t.Fatalf("Session returned error: %v", sessionErr)
	}
	for _, eventType := range []string{"tool.call", "tool.result", "operation.started", "operation.completed"} {
		if got := countSessionEventType(projection.Events, eventType); got != 0 {
			t.Fatalf("%s events = %d, want no tool admission/effect after provider command shape failure", eventType, got)
		}
	}
}

func TestCommandProviderStrictDecoderPreservesValidResponses(t *testing.T) {
	provider := strictProviderCommandHelper(t, "valid-final")
	resp, err := provider.Complete(context.Background(), providerCommandStrictTestRequest())
	if err != nil {
		t.Fatalf("Complete valid final returned error: %v", err)
	}
	if resp.Text != "strict final" {
		t.Fatalf("final text = %q, want strict final", resp.Text)
	}

	k := newTestKernelWithResources(t, filepath.Join(testTempDir(t), "events.sqlite"), []ResourceDescriptor{{
		Ref:      "cf:strict-provider-command",
		MimeType: "text/plain",
		Text:     "STRICT PROVIDER COMMAND RESOURCE",
	}})
	k.provider = strictProviderCommandHelper(t, "valid-tool-loop")
	turnResp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "provider-command-strict-valid-tool",
		InputItems: []InputItem{{Type: "text", Text: "read resource"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn valid tool loop returned error: %v", err)
	}
	if turnResp.Final.Text != "strict tool loop final" {
		t.Fatalf("final text = %q, want strict tool loop final", turnResp.Final.Text)
	}
}

func TestCommandProviderReturnsReasoningAsStructuredResponse(t *testing.T) {
	provider := strictProviderCommandHelper(t, "final-with-reasoning")

	resp, err := provider.Complete(context.Background(), providerCommandStrictTestRequest())
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if resp.Text != "strict final" {
		t.Fatalf("final text = %q, want strict final", resp.Text)
	}
	if resp.Reasoning == nil || resp.Reasoning.Text != "inspect the request before answering" {
		t.Fatalf("reasoning = %#v, want structured provider reasoning", resp.Reasoning)
	}
}

func TestCommandProviderForwardsCanonicalConversation(t *testing.T) {
	provider := strictProviderCommandHelper(t, "assert-conversation")
	response, err := provider.Complete(context.Background(), ModelRequest{
		SessionID: "canonical-session",
		TurnID:    "canonical-turn",
		Conversation: []ModelConversationMessage{
			{Role: "system", Text: "stable prefix"},
			{Role: "user", Text: "latest user request"},
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if response.Text != "canonical conversation forwarded" {
		t.Fatalf("response text = %q", response.Text)
	}
}

func TestCommandProviderExplicitUnboundedRequestDoesNotAddDeadline(t *testing.T) {
	provider := NewCommandProvider(ProviderCommandConfig{
		Command:               os.Args[0],
		Args:                  []string{"-test.run=TestStrictProviderCommandAdapterHelper", "--", "delayed-final"},
		Model:                 "command-model",
		RequestTimeout:        5 * time.Millisecond,
		AllowUnboundedRequest: true,
		Env:                   []string{"GENESIS_PROVIDER_COMMAND_STRICT_HELPER=1"},
	})

	response, err := provider.Complete(context.Background(), providerCommandStrictTestRequest())
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if response.Text != "delayed final" {
		t.Fatalf("response text = %q, want delayed final", response.Text)
	}
}

func TestCommandProviderReasoningPersistsAsAssistantMessage(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	k.provider = strictProviderCommandHelper(t, "final-with-reasoning")

	response, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "provider-command-reasoning",
		InputItems: []InputItem{{Type: "text", Text: "explain this"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	for _, event := range response.Events {
		if event.Type != "model.reasoning" {
			continue
		}
		data, ok := event.Data.(EventData)
		if !ok || data.Reasoning == nil || data.Reasoning.Text != "inspect the request before answering" {
			t.Fatalf("reasoning event = %#v, want persisted provider-command reasoning", event.Data)
		}
		return
	}
	t.Fatalf("turn events = %#v, want model.reasoning", response.Events)
}

func TestOpenAICompatibleProviderToleratesVendorNativeExtraFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"served-model","vendor_top_level":true,"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"provider answer","vendor_message_field":{"ignored":true}}}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8,"vendor_usage_field":1}}`))
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
		t.Fatalf("Complete returned error for vendor extras: %v", err)
	}
	if resp.Text != "provider answer" || resp.Model != "served-model" {
		t.Fatalf("response = %+v, want tolerant vendor response", resp)
	}
}

func strictProviderCommandHelper(t *testing.T, mode string) *CommandProvider {
	t.Helper()
	return NewCommandProvider(ProviderCommandConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestStrictProviderCommandAdapterHelper", "--", mode},
		Model:          "command-model",
		RequestTimeout: 5 * time.Second,
		Env:            []string{"GENESIS_PROVIDER_COMMAND_STRICT_HELPER=1"},
	})
}

func providerCommandStrictTestRequest() ModelRequest {
	return ModelRequest{
		SessionID:  "command-session",
		TurnID:     "turn-command",
		InputItems: []ModelInputItem{{Kind: ModelInputKindUserText, Text: "hello command provider"}},
		ToolManifest: []ToolSpec{{
			Name:            "resource_read",
			Description:     "read governed resource",
			InputSchema:     map[string]interface{}{"type": "object"},
			SideEffectLevel: ToolSideEffectRead,
			ExecutionKind:   ToolExecutionKindKernelControl,
		}},
	}
}

func TestStrictProviderCommandAdapterHelper(t *testing.T) {
	if os.Getenv("GENESIS_PROVIDER_COMMAND_STRICT_HELPER") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	switch mode {
	case "valid-final":
		_, _ = os.Stdout.WriteString(`{"kind":"final","model":"command-model","text":"strict final"}`)
	case "delayed-final":
		time.Sleep(40 * time.Millisecond)
		_, _ = os.Stdout.WriteString(`{"kind":"final","model":"command-model","text":"delayed final"}`)
	case "final-with-reasoning":
		_, _ = os.Stdout.WriteString(`{"kind":"final","model":"command-model","text":"strict final","reasoning":{"text":"inspect the request before answering"}}`)
	case "empty-reasoning":
		_, _ = os.Stdout.WriteString(`{"kind":"final","model":"command-model","text":"strict final","reasoning":{"text":" "}}`)
	case "extra-final-field":
		_, _ = os.Stdout.WriteString(`{"kind":"final","model":"command-model","text":"strict final","extra":"drift"}`)
	case "tool-lease-id":
		_, _ = os.Stdout.WriteString(`{"kind":"tool_calls","model":"command-model","tool_calls":[{"tool_call_id":"call_hidden_lease","name":"resource_read","arguments":{"resource_ref":"cf:strict-provider-command"},"lease_id":"lease_model_supplied"}]}`)
	case "tool-budget-lease-id":
		_, _ = os.Stdout.WriteString(`{"kind":"tool_calls","model":"command-model","tool_calls":[{"tool_call_id":"call_hidden_budget_lease","name":"resource_read","arguments":{"resource_ref":"cf:strict-provider-command"},"budget_lease_id":"lease_model_supplied"}]}`)
	case "tool-unknown-field":
		_, _ = os.Stdout.WriteString(`{"kind":"tool_calls","model":"command-model","tool_calls":[{"tool_call_id":"call_unknown_field","name":"resource_read","arguments":{"resource_ref":"cf:strict-provider-command"},"surprise":"drift"}]}`)
	case "valid-tool-loop":
		var req providerCommandRequest
		decoder := json.NewDecoder(os.Stdin)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			t.Fatalf("decode provider command request: %v", err)
		}
		if len(req.ToolRounds) == 0 {
			_, _ = os.Stdout.WriteString(`{"kind":"tool_calls","model":"command-model","tool_calls":[{"tool_call_id":"call_valid_resource","name":"resource_read","arguments":{"resource_ref":"cf:strict-provider-command"}}]}`)
			os.Exit(0)
		}
		_, _ = os.Stdout.WriteString(`{"kind":"final","model":"command-model","text":"strict tool loop final"}`)
	case "assert-conversation":
		var req providerCommandRequest
		decoder := json.NewDecoder(os.Stdin)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			t.Fatalf("decode provider command request: %v", err)
		}
		if len(req.Conversation) != 2 || req.Conversation[0].Role != "system" || req.Conversation[0].Text != "stable prefix" || req.Conversation[1].Role != "user" || req.Conversation[1].Text != "latest user request" {
			t.Fatalf("conversation = %#v, want canonical ordered messages", req.Conversation)
		}
		_, _ = os.Stdout.WriteString(`{"kind":"final","model":"command-model","text":"canonical conversation forwarded"}`)
	default:
		t.Fatalf("unknown strict helper mode %q", mode)
	}
	os.Exit(0)
}
