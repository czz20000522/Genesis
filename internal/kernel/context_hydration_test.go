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

	"genesis/internal/testsupport"
)

func TestContextHydrationAdmitsBoundedResourceIntoNextProviderContext(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "context-hydration-accepted")
	provider := &capturingProvider{text: "hydrated answer"}
	body := "FULL SKILL BODY Authorization: Bearer local-token should stay literal and bounded"
	k, err := New(Config{
		LedgerPath:   filepath.Join(dir, "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		Resources: []ResourceDescriptor{{
			Ref:      "cf:skill-lark-body",
			MimeType: "text/plain",
			Text:     body,
		}},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	admitted, err := k.AdmitContextResource(ContextHydrationAdmissionRequest{
		SessionID:       "hydration-session",
		SourceOwner:     "skill_catalog",
		ResourceRef:     "cf:skill-lark-body",
		MaxVisibleBytes: len(body),
		Reason:          "operator requested full skill body",
		DerivationRefs:  []string{"skill:lark-im"},
	})
	if err != nil {
		t.Fatalf("AdmitContextResource returned error: %v", err)
	}
	if admitted.Status != "accepted" || admitted.HydrationID == "" || admitted.InputKind != ModelInputKindHydratedContext {
		t.Fatalf("admitted hydration = %+v, want accepted with system-owned id and hydrated input kind", admitted)
	}
	if admitted.ResourceHash == "" || admitted.OriginalBytes != len([]byte(body)) || admitted.VisibleBytes != len([]byte(body)) || admitted.Truncated {
		t.Fatalf("admitted evidence = %+v, want hash and byte accounting without truncation", admitted)
	}
	if admitted.VisibleText != body || strings.Contains(admitted.VisibleText, "[REDACTED]") {
		t.Fatalf("visible text = %q, want original content inside budget", admitted.VisibleText)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "hydration-session",
		InputItems: []InputItem{{Type: "text", Text: "use the hydrated context"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "hydrated answer" {
		t.Fatalf("final text = %q, want provider response", resp.Final.Text)
	}
	hydratedText, ok := modelInputTextByKind(provider.inputItems, ModelInputKindHydratedContext)
	if !ok {
		t.Fatalf("provider input kinds = %v, want hydrated context", provider.InputKinds())
	}
	if hydratedText != body || strings.Contains(hydratedText, "[REDACTED]") {
		t.Fatalf("hydrated provider text = %q, want unredacted bounded body", hydratedText)
	}
	if providerText := provider.InputText(); strings.Contains(providerText, filepath.Clean(dir)) || strings.Contains(providerText, "SKILL.md") {
		t.Fatalf("provider context leaked path/package detail: %q", providerText)
	}

	inspection, err := k.ContextInspection(resp.TurnID)
	if err != nil {
		t.Fatalf("ContextInspection returned error: %v", err)
	}
	if got := strings.Join(inspection.ModelInputKinds, ","); !strings.Contains(got, ModelInputKindHydratedContext) {
		t.Fatalf("inspection model kinds = %v, want hydrated context", inspection.ModelInputKinds)
	}
	if len(inspection.HydratedContexts) != 1 {
		t.Fatalf("inspection hydrated contexts = %+v, want one accepted context", inspection.HydratedContexts)
	}
	evidence := inspection.HydratedContexts[0]
	if evidence.SourceOwner != "skill_catalog" || evidence.ResourceRef != "cf:skill-lark-body" || evidence.ResourceHash == "" || evidence.MimeType != "text/plain" {
		t.Fatalf("hydration evidence = %+v, want source owner/ref/hash/mime", evidence)
	}
	if evidence.OriginalBytes != len([]byte(body)) || evidence.VisibleBytes != len([]byte(body)) || evidence.InputKind != ModelInputKindHydratedContext {
		t.Fatalf("hydration byte/input evidence = %+v", evidence)
	}

	timeline, err := k.UITimeline("hydration-session")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	timelineJSON, err := json.Marshal(timeline)
	if err != nil {
		t.Fatalf("marshal timeline: %v", err)
	}
	if strings.Contains(string(timelineJSON), body) || strings.Contains(string(timelineJSON), ModelInputKindHydratedContext) {
		t.Fatalf("timeline rendered hydration as chat surface: %s", string(timelineJSON))
	}
	assertNoEventTypes(t, k, "tool.call", "tool.result")
}

func TestContextHydrationRejectsResourcesWithoutPromptSplicing(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "context-hydration-rejected")
	oversize := strings.Repeat("OVERSIZE-CONTENT-", 500)
	provider := &capturingProvider{text: "plain answer"}
	k, err := New(Config{
		LedgerPath:   filepath.Join(dir, "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		Resources: []ResourceDescriptor{
			{Ref: "cf:oversize", MimeType: "text/plain", Text: oversize},
			{Ref: "cf:json", MimeType: "application/json", Text: `{"body":"unsupported"}`},
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	cases := []struct {
		name       string
		request    ContextHydrationAdmissionRequest
		wantReason string
		forbidden  string
	}{
		{
			name: "missing resource",
			request: ContextHydrationAdmissionRequest{
				SessionID:   "reject-session",
				SourceOwner: "connector_attachment",
				ResourceRef: "cf:missing",
			},
			wantReason: "resource_not_found",
			forbidden:  "cf:missing",
		},
		{
			name: "unsupported mime",
			request: ContextHydrationAdmissionRequest{
				SessionID:       "reject-session",
				SourceOwner:     "connector_attachment",
				ResourceRef:     "cf:json",
				MaxVisibleBytes: 128,
			},
			wantReason: "unsupported_mime_type",
			forbidden:  "unsupported",
		},
		{
			name: "oversize without explicit cap",
			request: ContextHydrationAdmissionRequest{
				SessionID:   "reject-session",
				SourceOwner: "application_instruction",
				ResourceRef: "cf:oversize",
			},
			wantReason: "max_visible_bytes_required",
			forbidden:  "OVERSIZE-CONTENT",
		},
		{
			name: "missing session",
			request: ContextHydrationAdmissionRequest{
				SourceOwner:     "application_instruction",
				ResourceRef:     "cf:oversize",
				MaxVisibleBytes: 64,
			},
			wantReason: "invalid_session_id",
			forbidden:  "OVERSIZE-CONTENT",
		},
		{
			name: "missing source owner",
			request: ContextHydrationAdmissionRequest{
				SessionID:       "reject-session",
				ResourceRef:     "cf:oversize",
				MaxVisibleBytes: 64,
			},
			wantReason: "invalid_source_owner",
			forbidden:  "OVERSIZE-CONTENT",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rejected, err := k.AdmitContextResource(tc.request)
			if err != nil {
				t.Fatalf("AdmitContextResource returned error: %v", err)
			}
			if rejected.Status != "rejected" || rejected.RejectedReason != tc.wantReason || rejected.VisibleText != "" {
				t.Fatalf("rejected hydration = %+v, want %s without visible text", rejected, tc.wantReason)
			}
		})
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "reject-session",
		InputItems: []InputItem{{Type: "text", Text: "do not splice rejected context"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if _, ok := modelInputTextByKind(provider.inputItems, ModelInputKindHydratedContext); ok {
		t.Fatalf("provider input items = %+v, rejected hydration must not enter context", provider.inputItems)
	}
	providerJSON, err := json.Marshal(provider.inputItems)
	if err != nil {
		t.Fatalf("marshal provider inputs: %v", err)
	}
	for _, forbidden := range []string{"OVERSIZE-CONTENT", "unsupported", "cf:missing", "[REDACTED]"} {
		if strings.Contains(string(providerJSON), forbidden) {
			t.Fatalf("provider context leaked rejected payload/ref %q: %s", forbidden, string(providerJSON))
		}
	}
	inspection, err := k.ContextInspection(resp.TurnID)
	if err != nil {
		t.Fatalf("ContextInspection returned error: %v", err)
	}
	if len(inspection.HydratedContexts) != 0 {
		t.Fatalf("inspection hydrated contexts = %+v, want none for rejected admissions", inspection.HydratedContexts)
	}
}

func TestContextHydrationRejectsScopeMismatch(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "context-hydration-scope")
	k := newTestKernelWithResources(t, filepath.Join(dir, "events.jsonl"), []ResourceDescriptor{{
		Ref:      "cf:scoped",
		MimeType: "text/plain",
		Text:     "scoped body",
	}})
	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "scope-owner",
		InputItems: []InputItem{{Type: "text", Text: "existing turn"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}

	rejected, err := k.AdmitContextResource(ContextHydrationAdmissionRequest{
		SessionID:       "different-session",
		TurnID:          resp.TurnID,
		SourceOwner:     "connector_attachment",
		ResourceRef:     "cf:scoped",
		MaxVisibleBytes: 64,
	})
	if err != nil {
		t.Fatalf("AdmitContextResource returned error: %v", err)
	}
	if rejected.Status != "rejected" || rejected.RejectedReason != "scope_mismatch" || rejected.VisibleText != "" {
		t.Fatalf("scope mismatch projection = %+v, want rejected without text", rejected)
	}
}

func TestContextHydrationRejectsCallerOwnedControlFieldsOverHTTP(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "context-hydration-http")
	k := newTestKernelWithResources(t, filepath.Join(dir, "events.jsonl"), []ResourceDescriptor{{
		Ref:      "cf:control",
		MimeType: "text/plain",
		Text:     "control body",
	}})
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := postJSONWithAuth(server.URL+"/context/admit_resource", []byte(`{
		"session_id": "http-hydration",
		"source_owner": "skill_catalog",
		"resource_ref": "cf:control",
		"max_visible_bytes": 64,
		"hydration_id": "caller_supplied"
	}`))
	if err != nil {
		t.Fatalf("POST /context/admit_resource failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 400 for caller-owned hydration_id; body=%s", resp.StatusCode, string(body))
	}

	valid, err := postJSONWithAuth(server.URL+"/context/admit_resource", []byte(`{
		"session_id": "http-hydration",
		"source_owner": "skill_catalog",
		"resource_ref": "cf:control",
		"max_visible_bytes": 64
	}`))
	if err != nil {
		t.Fatalf("valid POST /context/admit_resource failed: %v", err)
	}
	defer valid.Body.Close()
	if valid.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(valid.Body)
		t.Fatalf("status = %d, want 200; body=%s", valid.StatusCode, string(body))
	}
	var projection ContextHydrationProjection
	if err := json.NewDecoder(valid.Body).Decode(&projection); err != nil {
		t.Fatalf("decode projection: %v", err)
	}
	if projection.HydrationID == "" || projection.HydrationID == "caller_supplied" || projection.CreatedAt.IsZero() {
		t.Fatalf("projection = %+v, want system-owned id and timestamp", projection)
	}
}

func TestContextHydrationDefaultSkillBodyAbsentAndForbiddenToolsRemainAbsent(t *testing.T) {
	root := testsupport.ProjectTempDir(t, "context-hydration-skill-index")
	writeSkillForTest(t, root, "lark-im", "lark-im", "Send chat messages", "FULL BODY MUST REQUIRE GENERIC HYDRATION")
	provider := &capturingProvider{text: "skill index answer"}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testsupport.ProjectTempDir(t, "context-hydration-skill-ledger"), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		SkillRoots:   []string{root},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "skill-default",
		InputItems: []InputItem{{Type: "text", Text: "what can you do?"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	providerBody := provider.InputText()
	if !strings.Contains(providerBody, "- lark-im: Send chat messages") {
		t.Fatalf("provider body = %q, want metadata skill index", providerBody)
	}
	if strings.Contains(providerBody, "FULL BODY MUST REQUIRE GENERIC HYDRATION") || strings.Contains(providerBody, root) {
		t.Fatalf("provider body leaked skill body/path without hydration: %q", providerBody)
	}
	inspection, err := k.ContextInspection(resp.TurnID)
	if err != nil {
		t.Fatalf("ContextInspection returned error: %v", err)
	}
	if len(inspection.HydratedContexts) != 0 {
		t.Fatalf("hydrated contexts = %+v, want none by default", inspection.HydratedContexts)
	}
	for _, tool := range k.toolGateway().ToolManifest() {
		visible, err := json.Marshal(tool)
		if err != nil {
			t.Fatalf("marshal tool: %v", err)
		}
		lower := strings.ToLower(string(visible))
		for _, forbidden := range []string{"skill.read", "read_skill", "skill_read"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("tool manifest exposed forbidden hydration tool %q: %s", forbidden, lower)
			}
		}
	}
}

func assertNoEventTypes(t *testing.T, k *Kernel, forbidden ...string) {
	t.Helper()
	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	for _, event := range events {
		for _, eventType := range forbidden {
			if event.Type == eventType {
				t.Fatalf("event %s present: %+v", eventType, event)
			}
		}
	}
}
