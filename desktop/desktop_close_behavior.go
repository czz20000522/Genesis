package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	closeBehaviorExit = "exit"
	closeBehaviorTray = "minimize_to_tray"
)

type desktopSettings struct {
	CloseBehavior string `json:"close_behavior"`
}

var desktopUserConfigDir = os.UserConfigDir

func normalizedCloseBehavior(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "", closeBehaviorExit:
		return closeBehaviorExit, nil
	case closeBehaviorTray:
		return closeBehaviorTray, nil
	default:
		return "", errors.New("invalid close behavior")
	}
}

func desktopSettingsPath() string {
	dir, err := desktopUserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Genesis", "desktop-settings.json")
}

func loadDesktopCloseBehavior() string {
	payload, err := os.ReadFile(desktopSettingsPath())
	if err != nil {
		return closeBehaviorExit
	}
	var settings desktopSettings
	if json.Unmarshal(payload, &settings) != nil {
		return closeBehaviorExit
	}
	value, err := normalizedCloseBehavior(settings.CloseBehavior)
	if err != nil {
		return closeBehaviorExit
	}
	return value
}

func saveDesktopCloseBehavior(value string) error {
	path := desktopSettingsPath()
	if path == "" {
		return errors.New("desktop settings path unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.Marshal(desktopSettings{CloseBehavior: value})
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o600)
}
