package localconfig

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadRuntimeSettingsPreservesTypedFeishuBinding(t *testing.T) {
	root := t.TempDir()
	path := RuntimeSettingsPath(root)
	if err := os.WriteFile(path, []byte(`{
  "feishu": {
    "listener": {"enabled": true},
    "lark_cli": {"profile": "genesis", "identity": "bot", "command": "lark-cli"},
    "allow_unbound_chats": false,
    "allowed_chat_ids": ["oc_1"]
  }
}`), 0o600); err != nil {
		t.Fatalf("write runtime settings: %v", err)
	}

	settings, err := ReadRuntimeSettings(path)
	if err != nil {
		t.Fatalf("ReadRuntimeSettings returned error: %v", err)
	}
	if !settings.Feishu.Listener.Enabled || settings.Feishu.LarkCLI.Profile != "genesis" || settings.Feishu.LarkCLI.Identity != "bot" || settings.Feishu.LarkCLI.Command != "lark-cli" {
		t.Fatalf("Feishu binding = %+v", settings.Feishu)
	}
	if len(settings.Feishu.AllowedChatIDs) != 1 || settings.Feishu.AllowedChatIDs[0] != "oc_1" || settings.Feishu.AllowUnboundChats {
		t.Fatalf("Feishu policy = %+v", settings.Feishu)
	}
}

func TestReadRuntimeSettingsRejectsMissingFile(t *testing.T) {
	_, err := ReadRuntimeSettings(filepath.Join(t.TempDir(), "runtime-settings.json"))
	if !errors.Is(err, ErrConfigMissing) {
		t.Fatalf("ReadRuntimeSettings error = %v, want ErrConfigMissing", err)
	}
}
