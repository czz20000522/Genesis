package jobruntime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJobProjectionJSONShape(t *testing.T) {
	exitCode := 0
	payload, err := json.Marshal(JobProjection{
		JobID:           "job_1",
		SessionID:       "session_1",
		TurnID:          "turn_1",
		Tool:            "shell_exec",
		IdempotencyKey:  "idem_1",
		Status:          "completed",
		CWD:             "D:/repo",
		Command:         "go test ./...",
		TimeoutSec:      600,
		ExitCode:        &exitCode,
		Stdout:          "ok",
		StdoutTruncated: true,
		StartedAt:       time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC),
		CompletedAt:     time.Date(2026, 6, 25, 10, 1, 0, 0, time.UTC),
		ToolCallEventID: "evt_tool",
	})
	if err != nil {
		t.Fatalf("marshal JobProjection: %v", err)
	}
	text := string(payload)
	for _, want := range []string{
		`"job_id":"job_1"`,
		`"session_id":"session_1"`,
		`"tool":"shell_exec"`,
		`"status":"completed"`,
		`"exit_code":0`,
		`"stdout_truncated":true`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("job projection payload = %s, want %s", text, want)
		}
	}
}

func TestObservationDeliveryJSONShape(t *testing.T) {
	payload, err := json.Marshal(ObservationDelivery{
		ObservationEventIDs: []string{"evt_job_completed"},
		ModelInputKind:      "kernel_observation_context",
	})
	if err != nil {
		t.Fatalf("marshal ObservationDelivery: %v", err)
	}
	text := string(payload)
	for _, want := range []string{
		`"observation_event_ids":["evt_job_completed"]`,
		`"model_input_kind":"kernel_observation_context"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("observation delivery payload = %s, want %s", text, want)
		}
	}
}
