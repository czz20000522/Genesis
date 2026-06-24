package testsupport

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const projectTempRetention = 24 * time.Hour

func ProjectRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if runtime.GOOS == "windows" && strings.EqualFold(filepath.VolumeName(dir), "C:") {
				t.Fatalf("project root must not be on C: drive: %q", dir)
			}
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find project root from %q", dir)
		}
		dir = parent
	}
}

func ProjectTempDir(t testing.TB, name string) string {
	t.Helper()
	root := ProjectRoot(t)
	base := filepath.Join(root, ".test-tmp", "go")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("create project temp root: %v", err)
	}
	cleanupOldProjectTempDirs(t, base, time.Now().Add(-projectTempRetention))
	prefix := safeTempPrefix(name)
	dir, err := os.MkdirTemp(base, prefix+"-")
	if err != nil {
		t.Fatalf("create project temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

func cleanupOldProjectTempDirs(t testing.TB, base string, cutoff time.Time) {
	t.Helper()
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(base, entry.Name()))
		}
	}
}

func safeTempPrefix(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "test"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	prefix := strings.Trim(b.String(), "-")
	if prefix == "" {
		return "test"
	}
	if len(prefix) > 48 {
		return prefix[:48]
	}
	return prefix
}
