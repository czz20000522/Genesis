package connectorruntime

import (
	"strings"
	"testing"
)

func TestSafeCLIProbeExcerptRedactsCredentialShapedOutput(t *testing.T) {
	got := SafeCLIProbeExcerpt([]byte("Authorization: Bearer sk-secret\nplain line"))
	if strings.Contains(got, "Authorization") || strings.Contains(got, "sk-secret") {
		t.Fatalf("excerpt leaked credential-shaped output: %q", got)
	}
	if !strings.Contains(got, "plain line") {
		t.Fatalf("excerpt dropped non-secret diagnostic line: %q", got)
	}
}
