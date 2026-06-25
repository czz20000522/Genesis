package kernel

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
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

func TestResourceReadPreservesKeyShapedTextInLocalAndModelInternalProjection(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "resource-read-content")
	secret := "sk-resource-secret"
	rawText := "resource body GENESIS_PROVIDER_API_KEY=" + secret + " still useful"
	args := mustMarshalToolArgs(t, map[string]interface{}{
		"resource_ref": "res_secret",
	})
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{{
			ToolCallID: "call_resource_secret",
			Name:       "resource_read",
			Arguments:  args,
		}},
		final: "resource content observed",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(dir, "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModePlan,
		},
		Resources: []ResourceDescriptor{{
			Ref:      "res_secret",
			MimeType: "text/plain",
			Text:     rawText,
		}},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "resource-read-content",
		InputItems: []InputItem{{Type: "text", Text: "read secret resource"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "resource content observed" {
		t.Fatalf("final text = %q, want resource content observed", resp.Final.Text)
	}
	requests := provider.Requests()
	if len(requests) != 2 || len(requests[1].ToolRounds) != 1 || len(requests[1].ToolRounds[0].Results) != 1 {
		t.Fatalf("provider requests = %+v, want one tool result round", requests)
	}
	result := requests[1].ToolRounds[0].Results[0]
	payload := decodeJSONMap(t, result.Content)
	text, _ := payload["text"].(string)
	if !strings.Contains(text, rawText) {
		t.Fatalf("resource_read text = %q, want original resource body", text)
	}
	assertDoesNotContain(t, result.Content, "[REDACTED]", "provider tool result")

	providerContext, err := k.ProviderContextProjection(resp.TurnID)
	if err != nil {
		t.Fatalf("ProviderContextProjection returned error: %v", err)
	}
	contextJSON, err := json.Marshal(providerContext.ModelRequest())
	if err != nil {
		t.Fatalf("marshal provider context: %v", err)
	}
	if !strings.Contains(string(contextJSON), secret) || strings.Contains(string(contextJSON), "[REDACTED]") {
		t.Fatalf("provider context should preserve model-internal resource result without lossy redaction: %s", string(contextJSON))
	}

	commandRounds, err := json.Marshal(providerCommandModelToolRounds(providerContext.ModelRequest().ToolRounds))
	if err != nil {
		t.Fatalf("marshal provider_command rounds: %v", err)
	}
	if !strings.Contains(string(commandRounds), secret) || strings.Contains(string(commandRounds), "[REDACTED]") {
		t.Fatalf("provider_command tool rounds should preserve resource content without lossy redaction: %s", string(commandRounds))
	}

	openAIMessages, err := json.Marshal(chatMessagesFromModelRequest(providerContext.ModelRequest()))
	if err != nil {
		t.Fatalf("marshal OpenAI-compatible messages: %v", err)
	}
	if !strings.Contains(string(openAIMessages), secret) || strings.Contains(string(openAIMessages), "[REDACTED]") {
		t.Fatalf("OpenAI-compatible tool messages should preserve resource content without lossy redaction: %s", string(openAIMessages))
	}

	session, err := k.Session("resource-read-content")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal session projection: %v", err)
	}
	if !strings.Contains(string(sessionJSON), secret) || strings.Contains(string(sessionJSON), "[REDACTED]") {
		t.Fatalf("session projection should preserve resource content without lossy redaction: %s", string(sessionJSON))
	}

	timeline, err := k.UITimeline("resource-read-content")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	timelineJSON, err := json.Marshal(timeline)
	if err != nil {
		t.Fatalf("marshal timeline: %v", err)
	}
	if !strings.Contains(string(timelineJSON), secret) || strings.Contains(string(timelineJSON), "[REDACTED]") {
		t.Fatalf("UI timeline should preserve resource content without lossy redaction: %s", string(timelineJSON))
	}
}

func TestResourceReadOffsetUsesBudgetSliceWithoutCredentialMasking(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "resource-read-offset-budget")
	secret := "sk-resource-secret"
	rawText := "prefix GENESIS_PROVIDER_API_KEY=" + secret + " suffix"
	offset := strings.Index(rawText, "resource-secret")
	if offset < 0 {
		t.Fatal("test fixture missing partial secret")
	}
	k := newTestKernelWithResources(t, filepath.Join(dir, "events.jsonl"), []ResourceDescriptor{{
		Ref:      "res_secret",
		MimeType: "text/plain",
		Text:     rawText,
	}})
	args := mustMarshalToolArgs(t, map[string]interface{}{
		"resource_ref": "res_secret",
		"offset_bytes": offset,
		"limit_bytes":  len("resource-secret"),
	})

	prepared, err := k.toolGateway().PrepareBatch([]ModelToolCall{{
		ToolCallID:      "call_resource_secret_offset",
		ToolCallEventID: "evt_tool_resource_secret_offset",
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
	payload := decodeJSONMap(t, result.Content)
	text, _ := payload["text"].(string)
	if text != "resource-secret" {
		t.Fatalf("offset resource_read text = %q, want requested byte slice", text)
	}
	if strings.Contains(result.Content, "[REDACTED]") {
		t.Fatalf("offset resource_read should not use lossy redaction: %s", result.Content)
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
		if call.accessPlan.EffectClass != ToolEffectClassPureRead || call.accessPlan.ParallelPolicy != ToolParallelPolicyCompatibleLocks || call.accessPlan.ParallelClass() != ToolEffectClassPureRead {
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

func assertDoesNotContain(t *testing.T, text string, forbidden string, label string) {
	t.Helper()
	if strings.Contains(text, forbidden) {
		t.Fatalf("%s leaked %q: %s", label, forbidden, text)
	}
}
