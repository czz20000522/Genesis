package codeintelligenceruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchitectureCodeGraphDoesNotEnterKernelCore(t *testing.T) {
	kernelDir := filepath.Join("..", "..", "kernel")
	err := filepath.WalkDir(kernelDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(string(content)), "codegraph") {
			t.Fatalf("kernel core file %s mentions codegraph; code intelligence belongs to user-space applications", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk kernel dir: %v", err)
	}
}
