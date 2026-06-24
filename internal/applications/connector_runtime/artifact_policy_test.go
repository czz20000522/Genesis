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
		filepath.Join("cmd", "genesis-feishu-connector-adapter"),
		filepath.Join("cmd", "genesis-ingress"),
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
			if strings.Contains(string(content), "t."+"TempDir(") {
				t.Fatalf("application test %s uses system temp; use testsupport.ProjectTempDir", path)
			}
			return nil
		}); err != nil {
			t.Fatalf("scan application tests under %s: %v", dir, err)
		}
	}
}
