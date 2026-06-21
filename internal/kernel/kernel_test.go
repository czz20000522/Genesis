package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestSubmitTurnPersistsAndProjectsAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID: "session-test",
		InputItems: []InputItem{
			{Type: "text", Text: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.SessionID != "session-test" {
		t.Fatalf("SessionID = %q, want session-test", resp.SessionID)
	}
	if resp.Final.Text != "fake: hello" {
		t.Fatalf("Final.Text = %q, want fake: hello", resp.Final.Text)
	}
	if len(resp.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2", len(resp.Events))
	}

	restarted := newTestKernel(t, ledgerPath)
	projection, err := restarted.Session("session-test")
	if err != nil {
		t.Fatalf("Session after restart returned error: %v", err)
	}
	if len(projection.Turns) != 1 {
		t.Fatalf("len(Turns) = %d, want 1", len(projection.Turns))
	}
	turn := projection.Turns[0]
	if turn.Status != "completed" {
		t.Fatalf("turn status = %q, want completed", turn.Status)
	}
	if turn.FinalMessage.Text != "fake: hello" {
		t.Fatalf("turn final = %q, want fake: hello", turn.FinalMessage.Text)
	}
	if len(projection.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2", len(projection.Events))
	}
}

func TestSubmitTurnRejectsInvalidInput(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))

	_, err := k.SubmitTurn(context.Background(), TurnRequest{})
	if err == nil {
		t.Fatal("SubmitTurn returned nil error for missing input_items")
	}

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		InputItems: []InputItem{{Type: "image", Text: "not supported"}},
	})
	if err == nil {
		t.Fatal("SubmitTurn returned nil error for unsupported input type")
	}
}

func TestHTTPReadyTurnAndSession(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	readyResp, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer readyResp.Body.Close()
	if readyResp.StatusCode != http.StatusOK {
		t.Fatalf("ready status = %d, want 200", readyResp.StatusCode)
	}

	body := []byte(`{"session_id":"http-session","input_items":[{"type":"text","text":"hello over http"}]}`)
	turnResp, err := http.Post(server.URL+"/turn", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer turnResp.Body.Close()
	if turnResp.StatusCode != http.StatusOK {
		t.Fatalf("turn status = %d, want 200", turnResp.StatusCode)
	}
	var turn TurnResponse
	if err := json.NewDecoder(turnResp.Body).Decode(&turn); err != nil {
		t.Fatalf("decode turn response: %v", err)
	}
	if turn.Final.Text != "fake: hello over http" {
		t.Fatalf("turn final = %q, want fake: hello over http", turn.Final.Text)
	}

	sessionResp, err := http.Get(server.URL + "/sessions/http-session")
	if err != nil {
		t.Fatalf("GET /sessions failed: %v", err)
	}
	defer sessionResp.Body.Close()
	if sessionResp.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d, want 200", sessionResp.StatusCode)
	}
	var projection SessionProjection
	if err := json.NewDecoder(sessionResp.Body).Decode(&projection); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if len(projection.Turns) != 1 {
		t.Fatalf("len(Turns) = %d, want 1", len(projection.Turns))
	}
}

func TestHTTPRejectsUnknownTurnFields(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"bad-session","input_items":[{"type":"text","text":"hello"}],"unexpected":true}`)
	resp, err := http.Post(server.URL+"/turn", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if _, err := k.Session("bad-session"); err != ErrSessionNotFound {
		t.Fatalf("Session error = %v, want ErrSessionNotFound", err)
	}
}

func TestHTTPRejectsTrailingJSON(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"bad-session","input_items":[{"type":"text","text":"hello"}]}{}`)
	resp, err := http.Post(server.URL+"/turn", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if _, err := k.Session("bad-session"); err != ErrSessionNotFound {
		t.Fatalf("Session error = %v, want ErrSessionNotFound", err)
	}
}

func TestHTTPRejectsOversizedTurnRequest(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := bytes.Repeat([]byte(" "), maxTurnRequestBytes+1)
	resp, err := http.Post(server.URL+"/turn", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func newTestKernel(t *testing.T, ledgerPath string) *Kernel {
	t.Helper()
	k, err := New(Config{
		LedgerPath: ledgerPath,
		Provider:   FakeProvider{},
		Clock: func() time.Time {
			return time.Date(2026, 6, 22, 1, 2, 3, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return k
}
