package kernel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKernelInjectsSkillCatalogBeforeProviderWithoutSkillBodies(t *testing.T) {
	root := t.TempDir()
	larkSkillPath := writeSkillForTest(t, root, "lark-im", "lark-im", "Send and read chat messages", "FULL LARK BODY MUST NOT BE INJECTED")
	mailSkillPath := writeSkillForTest(t, root, "mail", "mail", "Send email through an installed CLI", "FULL MAIL BODY MUST NOT BE INJECTED")
	writeMalformedSkillForTest(t, root, "broken")
	provider := &capturingProvider{text: "skill-aware answer"}
	k, err := New(Config{
		LedgerPath:   filepath.Join(t.TempDir(), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		SkillRoots:   []string{root, filepath.Join(root, "missing")},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "skill-catalog-context",
		InputItems: []InputItem{{Type: "text", Text: "How can you use installed external tools?"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "skill-aware answer" {
		t.Fatalf("final text = %q, want provider response", resp.Final.Text)
	}

	content := provider.InputText()
	if !strings.Contains(content, "Available external skills:") {
		t.Fatalf("provider content = %q, want skill catalog header", content)
	}
	for _, want := range []string{
		"- lark-im: Send and read chat messages",
		"- mail: Send email through an installed CLI",
		filepath.Clean(larkSkillPath),
		filepath.Clean(mailSkillPath),
		"How can you use installed external tools?",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("provider content = %q, want %q", content, want)
		}
	}
	for _, forbidden := range []string{"FULL LARK BODY MUST NOT BE INJECTED", "FULL MAIL BODY MUST NOT BE INJECTED", "broken"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("provider content = %q, must not contain %q", content, forbidden)
		}
	}
}

func TestMissingAndMalformedSkillCatalogDoesNotBlockTurn(t *testing.T) {
	root := t.TempDir()
	writeMalformedSkillForTest(t, root, "broken")
	provider := &capturingProvider{text: "plain answer"}
	k, err := New(Config{
		LedgerPath:   filepath.Join(t.TempDir(), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		SkillRoots:   []string{root, filepath.Join(root, "missing")},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "skill-catalog-empty",
		InputItems: []InputItem{{Type: "text", Text: "hello"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if content := provider.InputText(); content != "hello" {
		t.Fatalf("provider content = %q, want only user text", content)
	}
}

func writeSkillForTest(t *testing.T, root string, dir string, name string, description string, body string) string {
	t.Helper()
	skillDir := filepath.Join(root, dir)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	content := "---\nname: " + name + "\ndescription: \"" + description + "\"\n---\n\n# Body\n\n" + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	return path
}

func writeMalformedSkillForTest(t *testing.T, root string, dir string) {
	t.Helper()
	skillDir := filepath.Join(root, dir)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir malformed skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# missing front matter\nbroken"), 0o644); err != nil {
		t.Fatalf("write malformed skill: %v", err)
	}
}

type capturingProvider struct {
	text       string
	inputItems []InputItem
}

func (p *capturingProvider) Name() string {
	return "capturing"
}

func (p *capturingProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Status: "ok"}
}

func (p *capturingProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	p.inputItems = cloneInputItems(req.InputItems)
	return ModelResponse{
		Text:  p.text,
		Model: "capturing-model",
	}, nil
}

func (p *capturingProvider) InputText() string {
	var parts []string
	for _, item := range p.inputItems {
		if item.Type == "text" && item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "\n")
}
