package testsupport

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProjectTempDirCreatesDirectoryUnderProjectAndCleansIt(t *testing.T) {
	dir := ProjectTempDir(t, "sample")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("project temp dir was not created: %v", err)
	}
	root := ProjectRoot(t)
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		t.Fatalf("temp dir %q is not relatable to project root %q: %v", dir, root, err)
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		t.Fatalf("temp dir %q is outside project root %q", dir, root)
	}
	if runtime.GOOS == "windows" && strings.EqualFold(filepath.VolumeName(dir), "C:") {
		t.Fatalf("temp dir must not be on C: drive: %q", dir)
	}
}
