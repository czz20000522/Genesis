package kernel

import (
	"testing"

	"genesis/internal/testsupport"
)

func testTempDir(t testing.TB) string {
	t.Helper()
	return testsupport.ProjectTempDir(t, t.Name())
}
