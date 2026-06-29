package feishucli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func SelectExecutable(explicit string, installed string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	if strings.TrimSpace(installed) != "" {
		return strings.TrimSpace(installed)
	}
	return "lark-cli"
}

func InstalledOfficialExecutable() string {
	candidate := OfficialExecutable(os.Getenv("APPDATA"), runtime.GOOS)
	if candidate == "" {
		return ""
	}
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return ""
	}
	return candidate
}

func OfficialExecutable(appData string, goos string) string {
	if goos != "windows" || strings.TrimSpace(appData) == "" {
		return ""
	}
	return filepath.Join(appData, "npm", "node_modules", "@larksuite", "cli", "bin", "lark-cli.exe")
}
