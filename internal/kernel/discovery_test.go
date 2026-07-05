package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiscoveryQueryReturnsOnlyApprovedAccumulation(t *testing.T) {
	provider := &recordingTextProvider{text: "ok"}
	k := newDiscoveryTestKernel(t, provider, nil)

	approved := createMemoryCandidateForDiscovery(t, k, "应用层优先使用成熟第三方依赖", MemoryCandidateApproved)
	createMemoryCandidateForDiscovery(t, k, "成熟第三方依赖 rejected should not be returned", MemoryCandidateRejected)
	createMemoryCandidateForDiscovery(t, k, "成熟第三方依赖 superseded should not be returned", MemoryCandidateSuperseded)
	createMemoryCandidateForDiscovery(t, k, "成熟第三方依赖 forgotten should not be returned", MemoryCandidateForgotten)

	result, err := k.DiscoverContext(DiscoveryQueryRequest{
		Intent: "成熟第三方依赖",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("DiscoverContext returned error: %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want one approved candidate", result.Candidates)
	}
	got := result.Candidates[0]
	if got.Ref != "memory:"+approved.CandidateID || got.Kind != MemoryKindPreference || got.Scope != MemoryScopeGlobal {
		t.Fatalf("candidate = %+v, want approved memory projection", got)
	}
	if !strings.Contains(got.Summary, "成熟第三方依赖") || got.Confidence != "medium" {
		t.Fatalf("candidate = %+v, want bounded summary and confidence", got)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID: "discovery_no_auto_context",
		InputItems: []InputItem{{
			Type: "text",
			Text: "成熟第三方依赖",
		}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	requests := provider.Requests()
	if len(requests) != 1 {
		t.Fatalf("provider requests = %d, want one", len(requests))
	}
	for _, item := range requests[0].InputItems {
		if strings.Contains(item.Text, "应用层优先使用成熟第三方依赖") {
			t.Fatalf("provider context auto-injected discovery candidate: %+v", requests[0].InputItems)
		}
	}
}

func TestHTTPDiscoveryQueryRejectsControlFieldsAndBoundsResults(t *testing.T) {
	k := newDiscoveryTestKernel(t, FakeProvider{}, nil)
	for i := 0; i < 8; i++ {
		createMemoryCandidateForDiscovery(t, k, "bounded discovery marker", MemoryCandidateApproved)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := postJSONWithAuth(server.URL+"/discovery/query", []byte(`{"intent":"bounded discovery marker","approval_id":"approval:fake"}`))
	if err != nil {
		t.Fatalf("post discovery with control field: %v", err)
	}
	assertErrorCode(t, resp, http.StatusBadRequest, "invalid_request")

	resp, err = postJSONWithAuth(server.URL+"/discovery/query", []byte(`{"intent":"bounded discovery marker","limit":3}`))
	if err != nil {
		t.Fatalf("post discovery: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var result DiscoveryQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode discovery response: %v", err)
	}
	if len(result.Candidates) != 3 {
		t.Fatalf("candidates = %d, want bounded 3", len(result.Candidates))
	}
}

func TestDiscoveryReturnsCapabilityDescriptorWithoutSkillBodyOrPath(t *testing.T) {
	root := filepath.Join(testTempDir(t), "skills")
	skillPath := writeSkillForTest(t, root, "lark-im", "lark-im", "Send and read chat messages", "FULL BODY MUST NOT BE PROJECTED")
	k := newDiscoveryTestKernel(t, FakeProvider{}, []string{root})

	result, err := k.DiscoverContext(DiscoveryQueryRequest{
		Intent:         "chat messages",
		RequestedKinds: []string{MemoryKindCapabilityHint},
	})
	if err != nil {
		t.Fatalf("DiscoverContext returned error: %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want one capability descriptor", result.Candidates)
	}
	got := result.Candidates[0]
	if got.Kind != MemoryKindCapabilityHint || got.Ref != "capability:lark-im" || got.Scope != MemoryScopeCapability {
		t.Fatalf("candidate = %+v, want capability descriptor", got)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal discovery result: %v", err)
	}
	for _, forbidden := range []string{root, skillPath, "FULL BODY MUST NOT BE PROJECTED"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("discovery result leaked %q: %s", forbidden, string(raw))
		}
	}
}

func TestDiscoveryReturnsUserSpaceCapabilityDescriptorWithoutRunnerTruth(t *testing.T) {
	k := newDiscoveryTestKernelWithCapabilities(t, FakeProvider{}, nil, []CapabilityDescriptor{{
		CapabilityRef: "capability:video-transcript",
		Name:          "视频字幕提取",
		Summary:       "从抖音或 Bilibili 链接生成字幕文件。",
		Intents:       []string{"douyin url", "subtitle extraction"},
		InputSummary:  "url or share text",
		HealthSummary: "ready",
	}})

	result, err := k.DiscoverContext(DiscoveryQueryRequest{
		Intent:         "抖音 字幕",
		RequestedKinds: []string{MemoryKindCapabilityHint},
	})
	if err != nil {
		t.Fatalf("DiscoverContext returned error: %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want one user-space capability descriptor", result.Candidates)
	}
	got := result.Candidates[0]
	if got.Ref != "capability:video-transcript" || got.Kind != MemoryKindCapabilityHint || got.Scope != MemoryScopeCapability {
		t.Fatalf("candidate = %+v, want user-space capability descriptor", got)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal discovery result: %v", err)
	}
	for _, forbidden := range []string{"entrypoint", "genesis.capability.json", ".ps1", "manifest_path"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("capability discovery leaked runner truth %q: %s", forbidden, string(raw))
		}
	}
}

func TestCapabilityDescriptorRejectsHostPathRef(t *testing.T) {
	_, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:   FakeProvider{},
		CapabilityDescriptors: []CapabilityDescriptor{{
			CapabilityRef: `C:\Users\Tomczz\tool`,
			Name:          "bad",
			Summary:       "bad",
		}},
	})
	if err == nil {
		t.Fatal("New returned nil error for host-path capability ref")
	}
	if !strings.Contains(err.Error(), "host path") {
		t.Fatalf("error = %v, want host path rejection", err)
	}
}

func TestContextDiscoverToolReturnsHintsWithoutGrantingAuthority(t *testing.T) {
	args, err := json.Marshal(map[string]interface{}{
		"intent":          "成熟第三方依赖",
		"requested_kinds": []string{MemoryKindPreference},
		"limit":           1,
	})
	if err != nil {
		t.Fatalf("marshal context_discover args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{{
			ToolCallID: "call_discover",
			Name:       "context_discover",
			Arguments:  args,
		}},
		final: "done",
	}
	k := newDiscoveryTestKernel(t, provider, nil)
	createMemoryCandidateForDiscovery(t, k, "应用层优先使用成熟第三方依赖", MemoryCandidateApproved)

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID: "context_discover_tool",
		InputItems: []InputItem{{
			Type: "text",
			Text: "查一下长期积累",
		}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	requests := provider.Requests()
	if len(requests) != 2 {
		t.Fatalf("provider requests = %d, want tool round then final", len(requests))
	}
	if len(requests[1].ToolRounds) != 1 || len(requests[1].ToolRounds[0].Results) != 1 {
		t.Fatalf("tool rounds = %+v, want one context_discover result", requests[1].ToolRounds)
	}
	result := requests[1].ToolRounds[0].Results[0]
	if result.Name != "context_discover" {
		t.Fatalf("tool result name = %q, want context_discover", result.Name)
	}
	if !strings.Contains(result.Content, "成熟第三方依赖") {
		t.Fatalf("tool result content = %s, want discovery hint", result.Content)
	}
	for _, forbidden := range []string{"approval_authority", "credential", "tool_call_event_id", "permission_mode"} {
		if strings.Contains(result.Content, forbidden) {
			t.Fatalf("context_discover result leaked control field %q: %s", forbidden, result.Content)
		}
	}
}

func newDiscoveryTestKernel(t *testing.T, provider Provider, skillRoots []string) *Kernel {
	t.Helper()
	return newDiscoveryTestKernelWithCapabilities(t, provider, skillRoots, nil)
}

func newDiscoveryTestKernelWithCapabilities(t *testing.T, provider Provider, skillRoots []string, capabilities []CapabilityDescriptor) *Kernel {
	t.Helper()
	k, err := New(Config{
		LedgerPath:            filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:              provider,
		RuntimeToken:          testRuntimeToken,
		SkillRoots:            append([]string(nil), skillRoots...),
		CapabilityDescriptors: append([]CapabilityDescriptor(nil), capabilities...),
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModePlan,
		},
		Clock: func() time.Time {
			return time.Date(2026, 7, 5, 1, 2, 3, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	t.Cleanup(k.Close)
	return k
}

func createMemoryCandidateForDiscovery(t *testing.T, k *Kernel, text string, status string) MemoryCandidateProjection {
	t.Helper()
	candidate, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID:   "discovery_session",
		Text:        text,
		SourceRef:   "memory:discovery-source",
		Kind:        MemoryKindPreference,
		Scope:       MemoryScopeGlobal,
		AppliesWhen: "when choosing application dependencies",
		Strength:    MemoryStrengthPreference,
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate returned error: %v", err)
	}
	switch status {
	case MemoryCandidatePending:
		return candidate
	case MemoryCandidateApproved:
		approved, err := k.ApproveMemoryCandidate(candidate.CandidateID, MemoryApprovalRequest{
			ApprovalAuthority:   "operator:tester",
			ApprovalReason:      "validated for discovery",
			ApprovalEvidenceRef: "review:discovery",
		})
		if err != nil {
			t.Fatalf("ApproveMemoryCandidate returned error: %v", err)
		}
		return approved
	case MemoryCandidateRejected:
		rejected, err := k.RejectMemoryCandidate(candidate.CandidateID, MemoryRejectionRequest{
			RejectionAuthority:   "operator:tester",
			RejectionReason:      "not valid for discovery",
			RejectionEvidenceRef: "review:discovery",
		})
		if err != nil {
			t.Fatalf("RejectMemoryCandidate returned error: %v", err)
		}
		return rejected
	case MemoryCandidateSuperseded:
		supersession, err := k.SupersedeMemoryCandidate(candidate.CandidateID, MemorySupersessionRequest{
			ReplacementText:         "replacement candidate should remain pending",
			ReplacementSourceRef:    "memory:replacement",
			SupersessionAuthority:   "operator:tester",
			SupersessionReason:      "replaced",
			SupersessionEvidenceRef: "review:discovery",
		})
		if err != nil {
			t.Fatalf("SupersedeMemoryCandidate returned error: %v", err)
		}
		return supersession.Superseded
	case MemoryCandidateForgotten:
		approved, err := k.ApproveMemoryCandidate(candidate.CandidateID, MemoryApprovalRequest{
			ApprovalAuthority:   "operator:tester",
			ApprovalReason:      "validated before forgetting",
			ApprovalEvidenceRef: "review:discovery",
		})
		if err != nil {
			t.Fatalf("ApproveMemoryCandidate returned error: %v", err)
		}
		forgotten, err := k.ForgetMemoryCandidate(approved.CandidateID, MemoryForgetRequest{
			ForgetAuthority:   "operator:tester",
			ForgetReason:      "no longer valid",
			ForgetEvidenceRef: "review:discovery",
		})
		if err != nil {
			t.Fatalf("ForgetMemoryCandidate returned error: %v", err)
		}
		return forgotten
	default:
		t.Fatalf("unsupported memory status %q", status)
		return MemoryCandidateProjection{}
	}
}

func TestDiscoveryResponseArrayShape(t *testing.T) {
	raw, err := json.Marshal(DiscoveryQueryResponse{Candidates: []DiscoveryCandidateProjection{}})
	if err != nil {
		t.Fatalf("marshal discovery response: %v", err)
	}
	if bytes.Contains(raw, []byte(`"candidates":null`)) {
		t.Fatalf("empty discovery candidates encoded as null: %s", string(raw))
	}
}
