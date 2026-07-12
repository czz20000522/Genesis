package kernel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestHTTPSessionModelBindingProjectsProfileIDAndRejectsActiveSession(t *testing.T) {
	k := newSessionModelBindingKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	defer k.Close()
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	invalidResp, err := postJSONWithAuth(server.URL+"/sessions/http-session/model", []byte(`{"profile_id":""}`))
	if err != nil {
		t.Fatalf("POST invalid model binding: %v", err)
	}
	defer invalidResp.Body.Close()
	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST invalid model binding status = %d, want 400", invalidResp.StatusCode)
	}

	resp, err := postJSONWithAuth(server.URL+"/sessions/http-session/model", []byte(`{"profile_id":"deepseek-flash"}`))
	if err != nil {
		t.Fatalf("POST model binding: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST model binding status = %d, want 200", resp.StatusCode)
	}
	var projection SessionProjection
	if err := json.NewDecoder(resp.Body).Decode(&projection); err != nil {
		t.Fatalf("decode projection: %v", err)
	}
	if projection.ModelProfileID != "deepseek-flash" {
		t.Fatalf("ModelProfileID = %q, want deepseek-flash", projection.ModelProfileID)
	}

	_, finish, admitted := k.tryBeginActiveTurn(nil, "http-session", "turn-active")
	if !admitted {
		t.Fatal("tryBeginActiveTurn did not admit test turn")
	}
	defer finish()
	resp, err = postJSONWithAuth(server.URL+"/sessions/http-session/model", []byte(`{"profile_id":"local-qwen"}`))
	if err != nil {
		t.Fatalf("POST active model binding: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("POST active model binding status = %d, want 409", resp.StatusCode)
	}
	var body errorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode active binding error: %v", err)
	}
	if body.Error.Code != "session_model_change_blocked_active_turn" {
		t.Fatalf("error code = %q", body.Error.Code)
	}
}

func TestHTTPSessionModelUnselectedFailsBeforeSubmittingTurn(t *testing.T) {
	k := newSessionModelBindingKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	defer k.Close()
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := postJSONWithAuth(server.URL+"/turn", []byte(`{"session_id":"unbound","input_items":[{"type":"text","text":"hello"}]}`))
	if err != nil {
		t.Fatalf("POST turn: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("POST unbound turn status = %d, want 409", resp.StatusCode)
	}
	var body errorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode unbound turn error: %v", err)
	}
	if body.Error.Code != "session_model_unselected" {
		t.Fatalf("error code = %q, want session_model_unselected", body.Error.Code)
	}
}
