package localconfig

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotProjectsConfiguredProfilesWithoutSecrets(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "models.json")
	if err := os.WriteFile(configPath, []byte(`{
  "model_gateway": {"routes": {"opencode": {"protocol": "openai-chat-completions", "base_url": "https://example.invalid/v1", "credential_ref": "secret://models/opencode/go"}}},
  "active_model_profile_bindings": {"coordinator": "opencode-glm"},
  "model_profiles": {"cloud": {"gateway": {"opencode-glm": {"profile_id": "opencode-glm", "model_id": "glm-5-2", "gateway_route": "opencode", "provider_adapter_id": "openai-compatible", "provider_adapter_profile_id": "zai-glm"}}}}
}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	snapshot, err := Snapshot(SnapshotRequest{ConfigRoot: root, CredentialStoreRoot: filepath.Join(root, "credentials")})
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	if len(snapshot.Profiles) != 1 {
		t.Fatalf("profiles = %+v, want one", snapshot.Profiles)
	}
	profile := snapshot.Profiles[0]
	if profile.ProfileID != "opencode-glm" || profile.ModelID != "glm-5-2" || profile.Protocol != "openai-chat-completions" {
		t.Fatalf("profile = %+v", profile)
	}
	if len(profile.Roles) != 1 || profile.Roles[0] != "coordinator" {
		t.Fatalf("roles = %+v", profile.Roles)
	}
	if profile.CredentialPresent {
		t.Fatalf("credential must be absent before a credential record exists")
	}
}

func TestRotateProfileCredentialWritesOnlyTheReferencedProtectedRecord(t *testing.T) {
	root := t.TempDir()
	credentialRoot := filepath.Join(root, "credentials")
	configPath := filepath.Join(root, "models.json")
	if err := os.WriteFile(configPath, []byte(`{
  "model_gateway": {"routes": {"cloud": {"protocol": "openai-chat-completions", "credential_ref": "secret://models/cloud/glm"}}},
  "model_profiles": {"cloud": {"gateway": {"glm": {"profile_id": "glm", "model_id": "glm-5-2", "gateway_route": "cloud"}}}}
}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	result, err := RotateProfileCredential(RotateProfileCredentialRequest{
		ConfigRoot:          root,
		CredentialStoreRoot: credentialRoot,
		ProfileID:           "glm",
		Secret:              "top-secret",
		Protector:           func(data []byte) ([]byte, error) { return append([]byte("sealed:"), data...), nil },
	})
	if err != nil {
		t.Fatalf("RotateProfileCredential returned error: %v", err)
	}
	payload, err := os.ReadFile(result.CredentialPath)
	if err != nil {
		t.Fatalf("read credential: %v", err)
	}
	if bytes.Contains(payload, []byte("top-secret")) || result.CredentialRef != "secret://models/cloud/glm" {
		t.Fatalf("credential result/payload leaked a secret: %+v %s", result, string(payload))
	}
}

func TestBindRoleUpdatesOnlyTheSelectedExistingProfile(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "models.json")
	if err := os.WriteFile(configPath, []byte(`{
  "model_gateway": {"routes": {"local": {"protocol": "provider_command"}}},
  "active_model_profile_bindings": {"coordinator": "old"},
  "model_profiles": {"local": {"gateway": {"old": {"profile_id": "old", "model_id": "qwen", "gateway_route": "local"}, "new": {"profile_id": "new", "model_id": "glm", "gateway_route": "local"}}}}
}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := BindRole(BindRoleRequest{ConfigRoot: root, ModelRole: "coordinator", ProfileID: "new"})
	if err != nil {
		t.Fatalf("BindRole returned error: %v", err)
	}
	if result.PreviousProfileID != "old" || result.ProfileID != "new" || result.ModelRole != "coordinator" {
		t.Fatalf("result = %+v", result)
	}
	snapshot, err := Snapshot(SnapshotRequest{ConfigRoot: root})
	if err != nil {
		t.Fatalf("Snapshot after bind returned error: %v", err)
	}
	if snapshot.RoleBindings["coordinator"] != "new" {
		t.Fatalf("bindings = %+v", snapshot.RoleBindings)
	}
}
