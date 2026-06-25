package workregistry

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWorkProjectionJSONShape(t *testing.T) {
	canceledAt := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	payload, err := json.Marshal(WorkProjection{
		WorkID:            "work_1",
		SessionID:         "session_1",
		Title:             "collect evidence",
		SourceRef:         "turn:abc",
		IdempotencyKey:    "idem_1",
		Status:            StatusCanceled,
		CreatedAt:         canceledAt.Add(-time.Hour),
		CancelAuthority:   "runtime:test",
		CancelReason:      "no longer needed",
		CancelEvidenceRef: "review:cancel",
		CanceledAt:        &canceledAt,
	})
	if err != nil {
		t.Fatalf("marshal WorkProjection: %v", err)
	}
	text := string(payload)
	for _, want := range []string{
		`"work_id":"work_1"`,
		`"session_id":"session_1"`,
		`"source_ref":"turn:abc"`,
		`"status":"canceled"`,
		`"cancel_authority":"runtime:test"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("work projection payload = %s, want %s", text, want)
		}
	}
}

func TestWorkRequestJSONShape(t *testing.T) {
	payload, err := json.Marshal(SubmitRequest{
		SessionID:      "session_1",
		Title:          "collect evidence",
		SourceRef:      "turn:abc",
		IdempotencyKey: "idem_1",
	})
	if err != nil {
		t.Fatalf("marshal SubmitRequest: %v", err)
	}
	text := string(payload)
	for _, want := range []string{
		`"session_id":"session_1"`,
		`"title":"collect evidence"`,
		`"source_ref":"turn:abc"`,
		`"idempotency_key":"idem_1"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("submit request payload = %s, want %s", text, want)
		}
	}
}
