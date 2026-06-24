package testsupport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKernelAndCommandTestsUseProjectTempDir(t *testing.T) {
	root := ProjectRoot(t)
	for _, relRoot := range []string{
		filepath.Join("internal", "kernel"),
		filepath.Join("cmd", "genesisd"),
		filepath.Join("cmd", "genesisctl"),
	} {
		walkTestFiles(t, filepath.Join(root, relRoot), func(path string, content string) {
			for _, forbidden := range []string{"t." + "TempDir(", "os." + "MkdirTemp("} {
				if strings.Contains(content, forbidden) {
					t.Fatalf("%s uses system temp via %s; use testsupport.ProjectTempDir", path, forbidden)
				}
			}
		})
	}
}

func walkTestFiles(t *testing.T, root string, visit func(path string, content string)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
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
		visit(path, string(content))
		return nil
	})
	if err != nil {
		t.Fatalf("walk test files under %s: %v", root, err)
	}
}
