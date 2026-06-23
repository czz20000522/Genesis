package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"genesis/internal/applications/message_ingress"
)

func TestFeishuOnceSubmitsInboundMessageWithoutOutboundCLIFlags(t *testing.T) {
	var got messageingress.TurnSubmitRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/turn" {
			t.Fatalf("path = %q, want /turn", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode turn request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(messageingress.TurnSubmitResponse{
			SessionID: got.SessionID,
			TurnID:    "turn-1",
			Final:     messageingress.FinalAnswer{Text: "local final"},
		})
	}))
	t.Cleanup(server.Close)

	err := run(context.Background(), []string{
		"feishu-once",
		"--kernel-url", server.URL,
		"--runtime-token", "token",
		"--state", filepath.Join(t.TempDir(), "state.json"),
		"--message-id", "msg-1",
		"--thread-id", "chat-1",
		"--user-id", "user-1",
		"--chat-id", "oc_123",
		"--text", "hello",
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if len(got.InputItems) != 1 {
		t.Fatalf("input items = %+v", got.InputItems)
	}
	input := got.InputItems[0].Text
	for _, want := range []string{"source_channel: feishu", "adapter: feishu-inbound", "chat_id: oc_123", "text:\nhello"} {
		if !strings.Contains(input, want) {
			t.Fatalf("turn input missing %q in:\n%s", want, input)
		}
	}
}
