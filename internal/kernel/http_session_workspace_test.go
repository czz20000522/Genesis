package kernel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTPSessionWorkspaceBindingPersistsModeWithoutRootProjection(t *testing.T) {
	workspace := testTempDir(t)
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	defer k.Close()
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := postJSONWithAuth(server.URL+"/sessions/http-project/workspace", []byte(`{"kind":"project","root":`+jsonString(workspace)+`}`))
	if err != nil {
		t.Fatalf("POST workspace binding returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST workspace binding status = %d", resp.StatusCode)
	}

	readResp, err := getWithAuth(server.URL + "/sessions/http-project")
	if err != nil {
		t.Fatalf("GET session returned error: %v", err)
	}
	defer readResp.Body.Close()
	var projection SessionProjection
	if err := json.NewDecoder(readResp.Body).Decode(&projection); err != nil {
		t.Fatalf("decode session projection: %v", err)
	}
	if projection.WorkspaceMode != SessionWorkspaceKindProject {
		t.Fatalf("WorkspaceMode = %q, want project", projection.WorkspaceMode)
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("marshal session projection: %v", err)
	}
	if strings.Contains(string(encoded), workspace) {
		t.Fatalf("session projection leaked workspace root: %s", string(encoded))
	}
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
