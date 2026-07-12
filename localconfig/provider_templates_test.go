package localconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportProviderTemplateRouteAndMaterializeProfilesWithoutLeakingSecret(t *testing.T) {
	root := t.TempDir()
	_, err := ImportProviderTemplateRoute(ProviderTemplateRouteImportRequest{
		ConfigRoot: root, CredentialStoreRoot: filepath.Join(root, "credentials"), TemplateID: ProviderTemplateDeepSeek, APIKey: "secret-key",
		Protector: func(data []byte) ([]byte, error) { return append([]byte("sealed:"), data...), nil },
	})
	if err != nil {
		t.Fatalf("ImportProviderTemplateRoute: %v", err)
	}
	result, err := MaterializeProviderTemplateModels(ProviderTemplateModelsRequest{ConfigRoot: root, RouteID: ProviderTemplateDeepSeek, Models: []string{"deepseek-v4-pro", "deepseek-v4-flash", "deepseek-v4-pro"}})
	if err != nil {
		t.Fatalf("MaterializeProviderTemplateModels: %v", err)
	}
	if result.RouteID != ProviderTemplateDeepSeek || len(result.ProfileIDs) != 2 {
		t.Fatalf("result = %+v", result)
	}
	payload, err := os.ReadFile(filepath.Join(root, "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "secret-key") {
		t.Fatalf("models config leaked secret: %s", payload)
	}
}

func TestImportAdvancedProviderRefusesInsecureRemoteURL(t *testing.T) {
	_, err := ImportProviderTemplateRoute(ProviderTemplateRouteImportRequest{ConfigRoot: t.TempDir(), TemplateID: ProviderTemplateAdvanced, APIKey: "secret-key", BaseURL: "http://example.com/v1"})
	if err == nil || err.Error() != "provider_base_url_insecure" {
		t.Fatalf("error = %v, want provider_base_url_insecure", err)
	}
}

func TestRepointLocalProviderAdapterChangesOnlyAdapterArgument(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "models.json")
	if err := os.WriteFile(path, []byte(`{"model_gateway":{"routes":{"local":{"protocol":"provider_command","command":"python.exe","args":["C:\\old\\llama_cpp_provider_command.py","--base-url","http://127.0.0.1:8081/v1"]}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RepointLocalProviderAdapter(root, `D:\software\Genesis\kernel\scripts\providers\llama_cpp_provider_command.py`); err != nil {
		t.Fatal(err)
	}
	config, err := ReadModels(path)
	if err != nil {
		t.Fatal(err)
	}
	args := config.ModelGateway.Routes["local"].Args
	if args[0] != `D:\software\Genesis\kernel\scripts\providers\llama_cpp_provider_command.py` || args[1] != "--base-url" {
		t.Fatalf("args = %#v", args)
	}
}
