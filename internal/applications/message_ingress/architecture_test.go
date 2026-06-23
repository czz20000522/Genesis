package messageingress

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchitectureMessageIngressDoesNotImportKernelInternals(t *testing.T) {
	walkGoFiles(t, ".", func(path string, text string) {
		if strings.Contains(text, "\"genesis/internal/kernel\"") || strings.Contains(text, "`genesis/internal/kernel`") {
			t.Fatalf("%s imports kernel internals", path)
		}
	})
}

func TestArchitectureMessageIngressDoesNotContainExternalOutboundSender(t *testing.T) {
	walkGoFiles(t, ".", func(path string, text string) {
		for _, forbidden := range []string{"im send", "lark-cli", "mail-cli", "wechat-cli", "ReplySender", "Deliver("} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s contains outbound sender marker %q", path, forbidden)
			}
		}
	})
}

func walkGoFiles(t *testing.T, root string, visit func(path string, text string)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		visit(path, string(content))
		return nil
	})
	if err != nil {
		t.Fatalf("walk go files: %v", err)
	}
}
