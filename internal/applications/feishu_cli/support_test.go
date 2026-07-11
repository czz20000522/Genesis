package feishucli

import (
	"path/filepath"
	"testing"
)

func TestSelectExecutablePrefersExplicitExecutable(t *testing.T) {
	got := SelectExecutable("D:\\tools\\lark-cli.exe", "C:\\fallback\\lark-cli.exe")
	if got != "D:\\tools\\lark-cli.exe" {
		t.Fatalf("executable = %q, want explicit path", got)
	}
}

func TestSelectExecutableResolvesConfiguredLarkCLINameToInstalledBinary(t *testing.T) {
	got := SelectExecutable("lark-cli", "C:\\official\\lark-cli.exe")
	if got != "C:\\official\\lark-cli.exe" {
		t.Fatalf("executable = %q, want installed binary", got)
	}
}

func TestOfficialExecutableUsesOfficialWindowsPackageBinary(t *testing.T) {
	got := OfficialExecutable("C:\\Users\\Tomczz\\AppData\\Roaming", "windows")
	want := filepath.Join("C:\\Users\\Tomczz\\AppData\\Roaming", "npm", "node_modules", "@larksuite", "cli", "bin", "lark-cli.exe")
	if got != want {
		t.Fatalf("official executable = %q, want %q", got, want)
	}
}
