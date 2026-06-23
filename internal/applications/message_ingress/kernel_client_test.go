package messageingress

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPKernelClientSubmitsTurnWithBearerToken(t *testing.T) {
	var gotAuth string
	var gotRequest TurnSubmitRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/turn" {
			t.Fatalf("path = %q, want /turn", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(TurnSubmitResponse{
			SessionID: gotRequest.SessionID,
			TurnID:    "turn-1",
			Final:     FinalAnswer{Text: "reply"},
		})
	}))
	t.Cleanup(server.Close)

	client := HTTPKernelClient{BaseURL: server.URL, RuntimeToken: "secret"}
	resp, err := client.SubmitTurn(context.Background(), TurnSubmitRequest{
		SessionID:      "session-1",
		IdempotencyKey: "dedupe-1",
		InputItems:     []TurnInputItem{{Type: "text", Text: "hello"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotRequest.SessionID != "session-1" || gotRequest.IdempotencyKey != "dedupe-1" {
		t.Fatalf("request = %+v", gotRequest)
	}
	if resp.Final.Text != "reply" {
		t.Fatalf("final text = %q", resp.Final.Text)
	}
}
