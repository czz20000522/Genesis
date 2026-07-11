package feishucli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func SelectExecutable(explicit string, installed string) string {
	explicit = strings.TrimSpace(explicit)
	installed = strings.TrimSpace(installed)
	if explicit != "" && !(explicit == "lark-cli" && installed != "") {
		return explicit
	}
	if installed != "" {
		return installed
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
