package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"genesis/internal/kernel"
)

func TestProviderSetupCommandDryRunDoesNotRequireAPIKey(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{
		"provider-setup",
		"-config-root", t.TempDir(),
		"-credential-store-root", t.TempDir(),
		"-base-url", "https://provider.example.com/api",
		"-model", "provider-model",
		"-dry-run",
	}, strings.NewReader(""), &stdout)
	if err != nil {
		t.Fatalf("dry-run command returned error: %v", err)
	}
	var response providerSetupResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || !response.DryRun || response.Verified {
		t.Fatalf("response = %+v, want dry-run ok without verify", response)
	}
}

func TestProviderSetupCommandWritesCredentialWithoutPrintingSecret(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("real local credential setup uses Windows DPAPI")
	}
	configRoot := t.TempDir()
	credentialRoot := t.TempDir()
	secret := "sk-command-secret"
	t.Setenv("GENESIS_PROVIDER_API_KEY", secret)

	var stdout bytes.Buffer
	err := run([]string{
		"provider-setup",
		"-config-root", configRoot,
		"-credential-store-root", credentialRoot,
		"-profile-id", "command-profile",
		"-gateway-route", "command-route",
		"-base-url", "https://provider.example.com/api",
		"-model", "provider-command-model",
		"-credential-ref", "secret://models/provider/command",
	}, strings.NewReader(""), &stdout)
	if err != nil {
		t.Fatalf("provider-setup command returned error: %v", err)
	}
	if strings.Contains(stdout.String(), secret) {
		t.Fatalf("command output leaked secret: %s", stdout.String())
	}
	var response providerSetupResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || !response.Verified {
		t.Fatalf("response = %+v, want verified ok", response)
	}
	configPayload, err := os.ReadFile(filepath.Join(configRoot, "models.json"))
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	if strings.Contains(string(configPayload), secret) {
		t.Fatalf("models.json leaked secret: %s", string(configPayload))
	}
	credentialPayload, err := os.ReadFile(response.CredentialPath)
	if err != nil {
		t.Fatalf("read credential record: %v", err)
	}
	if strings.Contains(string(credentialPayload), secret) {
		t.Fatalf("credential record leaked secret: %s", string(credentialPayload))
	}
	resolved, err := kernel.ResolveLocalCredentialSecret(response.CredentialRef, credentialRoot)
	if err != nil {
		t.Fatalf("ResolveLocalCredentialSecret returned error: %v", err)
	}
	if resolved != secret {
		t.Fatalf("resolved secret = %q, want original secret", resolved)
	}
}
