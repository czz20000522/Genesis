package localconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// RuntimeSettings contains user-home bindings for application adapters. It is
// intentionally typed per adapter: lifecycle enablement is shared, while
// vendor protocol settings remain outside a forced cross-channel schema.
type RuntimeSettings struct {
	Feishu FeishuRuntimeSettings `json:"feishu"`
}

type FeishuRuntimeSettings struct {
	Listener          FeishuListenerSettings `json:"listener"`
	LarkCLI           FeishuLarkCLISettings  `json:"lark_cli"`
	AllowUnboundChats bool                   `json:"allow_unbound_chats"`
	AllowedChatIDs    []string               `json:"allowed_chat_ids"`
}

type FeishuListenerSettings struct {
	Enabled bool `json:"enabled"`
}

type FeishuLarkCLISettings struct {
	Profile  string `json:"profile"`
	Identity string `json:"identity"`
	Command  string `json:"command"`
}

func RuntimeSettingsPath(root string) string {
	return filepath.Join(ResolveConfigRoot(root), "runtime-settings.json")
}

func ReadRuntimeSettings(path string) (RuntimeSettings, error) {
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return RuntimeSettings{}, ErrConfigMissing
	}
	if err != nil {
		return RuntimeSettings{}, fmt.Errorf("%w: %v", ErrConfigInvalid, err)
	}
	var settings RuntimeSettings
	if err := json.Unmarshal(payload, &settings); err != nil {
		return RuntimeSettings{}, fmt.Errorf("%w: %v", ErrConfigInvalid, err)
	}
	return settings, nil
}
