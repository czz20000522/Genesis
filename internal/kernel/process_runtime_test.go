package kernel

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestPlatformShellCommandOnWindowsForcesUTF8OutputEncoding(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows shell argv is platform-specific")
	}

	cmd := platformShellCommand(context.Background(), "Write-Output '你好'")
	if len(cmd.Args) < 5 {
		t.Fatalf("cmd args = %+v, want pwsh invocation with command payload", cmd.Args)
	}
	if cmd.Args[0] != "pwsh.exe" {
		t.Fatalf("cmd args[0] = %q, want pwsh.exe", cmd.Args[0])
	}
	if cmd.Args[1] != "-NoProfile" || cmd.Args[2] != "-NonInteractive" || cmd.Args[3] != "-Command" {
		t.Fatalf("cmd args = %+v, want -NoProfile -NonInteractive -Command", cmd.Args)
	}
	const utf8Prologue = "$OutputEncoding=[Console]::OutputEncoding=[System.Text.Encoding]::UTF8;"
	if !strings.HasPrefix(cmd.Args[4], utf8Prologue) {
		t.Fatalf("command payload = %q, want UTF-8 output prologue", cmd.Args[4])
	}
	if !strings.Contains(cmd.Args[4], "Write-Output '你好'") {
		t.Fatalf("command payload = %q, want original command preserved", cmd.Args[4])
	}
}
