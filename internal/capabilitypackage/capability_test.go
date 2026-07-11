package capabilitypackage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverProjectsHealthyPackageAndIsolatesInvalidSibling(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "report")
	if err := os.MkdirAll(good, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(good, "runner.exe"), []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(good, "genesis.capability.json"), []byte(`{"id":"report","name":"Report","description":"Generate report","entrypoint":"runner.exe","inputs":["path"],"outputs":["txt"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bad"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bad", "genesis.capability.json"), []byte(`{"id":"bad","entrypoint":"../escape"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "bad" || items[0].Readiness != "not_ready" || items[1].ID != "report" || items[1].Readiness != "ready" {
		t.Fatalf("items = %+v", items)
	}
	payload, err := json.Marshal(items[1])
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) == "" || string(payload) == `{"root":""}` || string(payload) == `{"entrypoint":""}` {
		t.Fatalf("projection = %s", payload)
	}
	for _, forbidden := range []string{"root", "entrypoint", "manifest_path"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("unsafe projection = %s", payload)
		}
	}
}
