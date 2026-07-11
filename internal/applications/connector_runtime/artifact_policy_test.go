package connectorruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"genesis/internal/testsupport"
)

func TestApplicationTestsUseProjectLocalArtifacts(t *testing.T) {
	root := testsupport.ProjectRoot(t)
	for _, relRoot := range []string{
		filepath.Join("cmd", "genesis-console"),
		filepath.Join("internal", "applications", "code_intelligence_runtime"),
		filepath.Join("internal", "applications", "connector_runtime"),
	} {
		dir := filepath.Join(root, relRoot)
		if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, forbidden := range []string{"t." + "TempDir(", "os." + "MkdirTemp("} {
				if strings.Contains(string(content), forbidden) {
					t.Fatalf("application test %s uses system temp via %s; use testsupport.ProjectTempDir", path, forbidden)
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("scan application tests under %s: %v", dir, err)
		}
	}
}
