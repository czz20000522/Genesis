package kernel

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSQLiteLedgerPersistsEventBodyInSessionEventFile(t *testing.T) {
	dir := testTempDir(t)
	ledgerPath := filepath.Join(dir, "events.sqlite")
	body := strings.Repeat("event-body-", 700)
	event := StoredEvent{
		EventID:   "evt_sqlite_file_body",
		SessionID: "session-sqlite-file",
		TurnID:    "turn-sqlite-file",
		Type:      "model.final",
		CreatedAt: time.Date(2026, 6, 29, 1, 2, 3, 0, time.UTC),
		Data: EventData{
			Final: &FinalMessage{Text: body, Model: "test-model"},
		},
	}

	ledger := NewSQLiteLedger(ledgerPath)
	if err := ledger.Append(event); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	restarted := NewSQLiteLedger(ledgerPath)
	events, err := restarted.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("loaded events = %d, want 1", len(events))
	}
	if events[0].EventID != event.EventID || events[0].SessionID != event.SessionID || events[0].Data.Final == nil || events[0].Data.Final.Text != body {
		t.Fatalf("loaded event = %+v, want original event body", events[0])
	}

	row := sqliteLedgerRow(t, ledgerPath, event.EventID)
	if row.FileRef == "" || row.EventBytes <= 0 || row.FrameBytes <= row.EventBytes {
		t.Fatalf("sqlite row = %+v, want file ref and framed event metadata", row)
	}
	if row.EventInline.Valid {
		t.Fatalf("sqlite row unexpectedly stores inline event body")
	}
	eventPath := filepath.Join(filepath.Dir(ledgerPath), filepath.FromSlash(row.FileRef))
	if _, err := os.Stat(eventPath); err != nil {
		t.Fatalf("event file %s is not readable: %v", eventPath, err)
	}
	dbBytes, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read sqlite db: %v", err)
	}
	if strings.Contains(string(dbBytes), body) {
		t.Fatalf("sqlite db contains full event body; event body must live in session event files")
	}
}

func TestSQLiteLedgerFailsClosedWhenIndexedEventFileIsMissing(t *testing.T) {
	dir := testTempDir(t)
	ledgerPath := filepath.Join(dir, "events.sqlite")
	event := StoredEvent{
		EventID:   "evt_missing_event_file",
		SessionID: "session-missing-file",
		TurnID:    "turn-missing-file",
		Type:      "turn.submitted",
		CreatedAt: time.Date(2026, 6, 29, 2, 3, 4, 0, time.UTC),
		Data: EventData{
			InputItems: []InputItem{{Type: "text", Text: "hello"}},
		},
	}
	ledger := NewSQLiteLedger(ledgerPath)
	if err := ledger.Append(event); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}
	row := sqliteLedgerRow(t, ledgerPath, event.EventID)
	if err := os.Remove(filepath.Join(filepath.Dir(ledgerPath), filepath.FromSlash(row.FileRef))); err != nil {
		t.Fatalf("remove event file: %v", err)
	}

	ready := ledger.Ready()
	if ready.Readiness != ReadinessNotReady || ready.ReadinessReason != "ledger_corrupt" {
		t.Fatalf("ready = %+v, want ledger_corrupt", ready)
	}
	if _, err := ledger.Load(); !errors.Is(err, ErrLedgerCorrupt) {
		t.Fatalf("Load error = %v, want ErrLedgerCorrupt", err)
	}
}

func TestHTTPListSessionsDerivesMinimalIndexFromEvents(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	first := time.Date(2026, 6, 29, 3, 0, 0, 0, time.UTC)
	second := first.Add(time.Minute)
	for _, event := range []StoredEvent{
		{EventID: "evt_list_a", SessionID: "session-a", TurnID: "turn-a", Type: "turn.submitted", CreatedAt: first, Data: EventData{InputItems: []InputItem{{Type: "text", Text: "first"}}}},
		{EventID: "evt_list_b", SessionID: "session-b", TurnID: "turn-b", Type: "model.final", CreatedAt: second, Data: EventData{Final: &FinalMessage{Text: "second"}}},
	} {
		if err := k.appendEvent(event); err != nil {
			t.Fatalf("append event %s: %v", event.EventID, err)
		}
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := getWithAuth(server.URL + "/sessions")
	if err != nil {
		t.Fatalf("GET /sessions failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var payload struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode sessions response: %v", err)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("items = %+v, want two sessions", payload.Items)
	}
	if payload.Items[0]["session_id"] != "session-b" || payload.Items[1]["session_id"] != "session-a" {
		t.Fatalf("items = %+v, want updated_at descending", payload.Items)
	}
	for _, item := range payload.Items {
		for key := range item {
			if key != "session_id" && key != "updated_at" {
				t.Fatalf("session list item contains unapproved field %q in %+v", key, item)
			}
		}
	}
}

type sqliteLedgerTestRow struct {
	FileRef     string
	EventBytes  int
	FrameBytes  int
	EventInline sql.NullString
}

func sqliteLedgerRow(t *testing.T, ledgerPath string, eventID string) sqliteLedgerTestRow {
	t.Helper()
	db, err := sql.Open("sqlite", ledgerPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()
	var row sqliteLedgerTestRow
	if err := db.QueryRow(`SELECT file_ref, event_bytes, frame_bytes, event_inline FROM session_events WHERE event_id = ?`, eventID).Scan(&row.FileRef, &row.EventBytes, &row.FrameBytes, &row.EventInline); err != nil {
		t.Fatalf("query session_events row: %v", err)
	}
	return row
}
