package connectorruntime

import (
	"archive/zip"
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	kernelpkg "genesis/internal/kernel"
	"genesis/internal/testsupport"
)

func TestFakeFeishuSourceCommandLongSessionUsesKernelContextQuality(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "feishu-context-quality")
	skillRoot := filepath.Join(dir, "skills")
	writeConnectorContextQualitySkill(t, skillRoot)
	zipPath := filepath.Join(dir, "package.zip")
	writeConnectorContextQualityZip(t, zipPath)
	provider := &connectorContextQualityProvider{}
	k, err := kernelpkg.New(kernelpkg.Config{
		LedgerPath:        filepath.Join(dir, "events.sqlite"),
		MaterialStorePath: filepath.Join(dir, "materials"),
		Provider:          provider,
		RuntimeToken:      "connector-context-quality-token",
		SkillRoots:        []string{skillRoot},
		Resources: []kernelpkg.ResourceDescriptor{{
			Ref:      "cf:connector-hydrated-context",
			MimeType: "text/plain",
			Text:     "bounded hydrated connector context for debug export",
		}},
		ContextPolicy: kernelpkg.ContextPolicy{
			ContextWindowTokens: 10,
			AutoCompactRatio:    0.5,
			RecentTurnLimit:     1,
			SkillIndexChars:     600,
		},
	})
	if err != nil {
		t.Fatalf("kernel.New returned error: %v", err)
	}

	seed := testExternalEvent("om_context_quality_seed")
	mapping, err := DefaultApplicationSessionMapper{}.Map(seed)
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}
	if _, err := k.EnableSessionDebug(mapping.KernelSessionID); err != nil {
		t.Fatalf("EnableSessionDebug returned error: %v", err)
	}
	intake, err := k.IntakeMaterial(kernelpkg.MaterialIntakeRequest{
		SessionID: mapping.KernelSessionID,
		Purpose:   kernelpkg.SourcePurposeAnalysis,
		Locator: kernelpkg.MaterialLocator{
			Kind: kernelpkg.MaterialLocatorKindLocalPath,
			Path: zipPath,
		},
	})
	if err != nil || intake.AdmissionResult != "admitted" {
		t.Fatalf("IntakeMaterial = %+v err=%v, want admitted source snapshot", intake, err)
	}
	if admitted, err := k.AdmitContextResource(kernelpkg.ContextHydrationAdmissionRequest{
		SessionID:       mapping.KernelSessionID,
		SourceOwner:     "connector_runtime",
		ResourceRef:     "cf:connector-hydrated-context",
		MaxVisibleBytes: 64,
		Reason:          "debug context quality smoke",
	}); err != nil || admitted.AdmissionResult != "admitted" {
		t.Fatalf("AdmitContextResource = %+v err=%v, want admitted", admitted, err)
	}

	server := httptestNewKernelServer(t, k)
	defer server.Close()
	runtime := &Runtime{
		InboundStore:  newTestInboundStore(t),
		Client:        HTTPKernelClient{BaseURL: server.URL, RuntimeToken: "connector-context-quality-token", HTTPClient: server.Client()},
		SessionMapper: DefaultApplicationSessionMapper{},
		Now: func() time.Time {
			return time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC)
		},
	}
	for i := 0; i < 4; i++ {
		event := testExternalEvent(fmt.Sprintf("om_context_quality_%02d", i))
		event.Body = fmt.Sprintf("Feishu long session message %02d", i)
		if _, err := runtime.ProcessSourceCommandEvent(context.Background(), event); err != nil {
			t.Fatalf("ProcessSourceCommandEvent(%d) returned error: %v", i, err)
		}
	}

	if provider.CompactionRequests() == 0 {
		t.Fatalf("compaction requests = 0, want auto compaction from kernel turn loop")
	}
	last := provider.LastNormalRequest()
	contextText := connectorModelInputText(last.InputItems)
	if !strings.Contains(contextText, "connector compacted summary") || !strings.Contains(contextText, "Feishu long session message 03") {
		t.Fatalf("last provider context = %q, want compacted summary plus recent fake Feishu tail", contextText)
	}

	timeline, err := k.UITimeline(mapping.KernelSessionID)
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	var notice bool
	walkConnectorTimelineItems(timeline.Items, func(item kernelpkg.UITimelineItem) {
		if strings.Contains(item.Text, "connector compacted summary") {
			t.Fatalf("timeline leaked compaction summary: %+v", item)
		}
		if item.Kind == "compaction_notice" {
			notice = true
		}
	})
	if !notice {
		t.Fatalf("timeline = %+v, want compaction notice", timeline.Items)
	}

	export, err := k.SessionDebugExport(mapping.KernelSessionID)
	if err != nil {
		t.Fatalf("SessionDebugExport returned error: %v", err)
	}
	if export.Readiness != kernelpkg.ReadinessReady || len(export.Steps) == 0 {
		t.Fatalf("debug export = %+v, want ready provider steps", export)
	}
	first := export.Steps[0]
	for _, want := range []string{
		kernelpkg.ModelInputKindSkillIndexContext,
		kernelpkg.ModelInputKindSourceSnapshotContext,
		kernelpkg.ModelInputKindHydratedContext,
		kernelpkg.ModelInputKindUserText,
	} {
		if !connectorContainsString(first.ModelInputKinds, want) {
			t.Fatalf("debug input kinds = %+v, want %s", first.ModelInputKinds, want)
		}
	}
	if len(first.SkillSummaries) == 0 || len(first.SourceContext) == 0 || len(first.HydratedContext) == 0 {
		t.Fatalf("debug step = %+v, want skill/source/hydrated bounded views", first)
	}
	if len(first.ToolManifest) == 0 || !connectorToolManifestContains(first.ToolManifest, "shell_exec") {
		t.Fatalf("debug tool manifest = %+v, want shell_exec", first.ToolManifest)
	}
}

func TestConnectorSourceRuntimeDoesNotWriteKernelCompactionTruth(t *testing.T) {
	root := testsupport.ProjectRoot(t)
	for _, rel := range []string{
		filepath.Join("internal", "applications", "connector_runtime"),
	} {
		dir := filepath.Join(root, rel)
		if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return err
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if strings.Contains(string(content), "context.compaction.") {
				t.Fatalf("%s contains kernel compaction event vocabulary; connector sources must submit requests, not compaction truth", path)
			}
			return nil
		}); err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
}

type connectorContextQualityProvider struct {
	mu                 sync.Mutex
	normalRequests     []kernelpkg.ModelRequest
	compactionRequests []kernelpkg.ModelRequest
}

func (p *connectorContextQualityProvider) Name() string {
	return "connector-context-quality-provider"
}

func (p *connectorContextQualityProvider) Ready() kernelpkg.ProviderStatus {
	return kernelpkg.ProviderStatus{Name: p.Name(), Readiness: kernelpkg.ReadinessReady}
}

func (p *connectorContextQualityProvider) Complete(_ context.Context, req kernelpkg.ModelRequest) (kernelpkg.ModelResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(req.InputItems) > 0 && req.InputItems[0].Kind == "context_compaction_source" {
		p.compactionRequests = append(p.compactionRequests, req)
		return kernelpkg.ModelResponse{
			Text:  "connector compacted summary",
			Model: "connector-compact-model",
			Usage: &kernelpkg.TokenUsage{InputTokens: 4, OutputTokens: 2, TotalTokens: 6, CacheMissTokens: 4},
		}, nil
	}
	p.normalRequests = append(p.normalRequests, req)
	return kernelpkg.ModelResponse{
		Text:  "connector final",
		Model: "connector-normal-model",
		Usage: &kernelpkg.TokenUsage{InputTokens: 20, OutputTokens: 2, TotalTokens: 22, CacheMissTokens: 20},
	}, nil
}

func (p *connectorContextQualityProvider) CompactionRequests() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.compactionRequests)
}

func (p *connectorContextQualityProvider) LastNormalRequest() kernelpkg.ModelRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.normalRequests) == 0 {
		return kernelpkg.ModelRequest{}
	}
	return p.normalRequests[len(p.normalRequests)-1]
}

func httptestNewKernelServer(t *testing.T, k *kernelpkg.Kernel) *httptest.Server {
	t.Helper()
	return httptest.NewServer(kernelpkg.Handler(k))
}

func writeConnectorContextQualitySkill(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "scientific-operator")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	body := "---\nname: scientific-operator\ndescription: Inspect source snapshots carefully\n---\nUse source tools only when needed.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func writeConnectorContextQualityZip(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("README.md")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write([]byte("connector context quality fixture")); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
}

func connectorModelInputText(items []kernelpkg.ModelInputItem) string {
	var parts []string
	for _, item := range items {
		parts = append(parts, item.Text)
	}
	return strings.Join(parts, "\n")
}

func connectorContainsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func connectorToolManifestContains(items []kernelpkg.ToolManifestInspection, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func walkConnectorTimelineItems(items []kernelpkg.UITimelineItem, visit func(kernelpkg.UITimelineItem)) {
	for _, item := range items {
		visit(item)
		walkConnectorTimelineItems(item.Children, visit)
	}
}
