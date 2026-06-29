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

func TestSQLiteLedgerImportsOrphanEventFramesWhenIndexIsMissing(t *testing.T) {
	dir := testTempDir(t)
	ledgerPath := filepath.Join(dir, "events.sqlite")
	first := StoredEvent{
		EventID:   "evt_orphan_first",
		SessionID: "session-orphan-a",
		TurnID:    "turn-orphan-a",
		Type:      "turn.submitted",
		CreatedAt: time.Date(2026, 6, 29, 3, 0, 0, 0, time.UTC),
		Data:      EventData{InputItems: []InputItem{{Type: "text", Text: "first orphan message with enough words"}}},
	}
	second := StoredEvent{
		EventID:   "evt_orphan_second",
		SessionID: "session-orphan-b",
		TurnID:    "turn-orphan-b",
		Type:      "model.final",
		CreatedAt: first.CreatedAt.Add(time.Minute),
		Data:      EventData{Final: &FinalMessage{Text: "second", Model: "fake"}},
	}
	ledger := NewSQLiteLedger(ledgerPath)
	for _, event := range []StoredEvent{first, second} {
		if err := ledger.Append(event); err != nil {
			t.Fatalf("Append %s returned error: %v", event.EventID, err)
		}
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	removeSQLiteIndexFiles(t, ledgerPath)

	restarted := NewSQLiteLedger(ledgerPath)
	events, err := restarted.Load()
	if err != nil {
		t.Fatalf("Load after index loss returned error: %v", err)
	}
	if len(events) != 2 || events[0].EventID != first.EventID || events[1].EventID != second.EventID {
		t.Fatalf("events = %+v, want orphan frames imported in event order", events)
	}
	sessions, err := restarted.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 2 || sessions[0].SessionID != second.SessionID || sessions[1].SessionID != first.SessionID {
		t.Fatalf("sessions = %+v, want sqlite-backed updated_at ordering", sessions)
	}
	if sessions[1].Title == "" || strings.Contains(sessions[1].Title, "\n") {
		t.Fatalf("imported session title = %q, want bounded first-message title", sessions[1].Title)
	}
}

func TestSQLiteLedgerSessionTitleUsesExternalEventBody(t *testing.T) {
	dir := testTempDir(t)
	ledgerPath := filepath.Join(dir, "events.sqlite")
	ledger := NewSQLiteLedger(ledgerPath)
	if err := ledger.Append(StoredEvent{
		EventID:   "evt_external_title",
		SessionID: "session-external-title",
		TurnID:    "turn-external-title",
		Type:      "turn.submitted",
		CreatedAt: time.Date(2026, 6, 30, 5, 0, 0, 0, time.UTC),
		Data: EventData{InputItems: []InputItem{{Type: "text", Text: strings.Join([]string{
			"External application event",
			"connector: feishu",
			"event_type: message.created",
			"",
			"text:",
			"Genesis 入站 smoke 20260630",
		}, "\n")}}},
	}); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}
	sessions, err := ledger.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Title != "Genesis 入站 smoke 20260630" {
		t.Fatalf("sessions = %+v, want external event body title", sessions)
	}
}

func TestSQLiteLedgerRecoversStaleWriterLock(t *testing.T) {
	dir := testTempDir(t)
	ledgerPath := filepath.Join(dir, "events.sqlite")
	staleCreatedAt := time.Now().Add(-2 * time.Hour)
	if err := os.WriteFile(ledgerPath+".lock", []byte("pid=0\ncreated_at="+staleCreatedAt.Format(time.RFC3339Nano)+"\n"), 0o644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}
	ledger := NewSQLiteLedger(ledgerPath)
	if err := ledger.Append(StoredEvent{
		EventID:   "evt_stale_lock",
		SessionID: "session-stale-lock",
		TurnID:    "turn-stale-lock",
		Type:      "turn.submitted",
		CreatedAt: time.Date(2026, 6, 30, 4, 0, 0, 0, time.UTC),
		Data:      EventData{InputItems: []InputItem{{Type: "text", Text: "hello"}}},
	}); err != nil {
		t.Fatalf("Append with stale lock returned error: %v", err)
	}
}

func TestSQLiteLedgerFailsClosedWhenExternalWriterLockExists(t *testing.T) {
	dir := testTempDir(t)
	ledgerPath := filepath.Join(dir, "events.sqlite")
	lockFile, err := os.OpenFile(ledgerPath+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("create external lock: %v", err)
	}
	defer lockFile.Close()

	ledger := NewSQLiteLedger(ledgerPath)
	ready := ledger.Ready()
	if ready.Readiness != ReadinessNotReady || ready.ReadinessReason != "ledger_locked" {
		t.Fatalf("ready = %+v, want ledger_locked", ready)
	}
	err = ledger.Append(StoredEvent{
		EventID:   "evt_external_lock",
		SessionID: "session-external-lock",
		TurnID:    "turn-external-lock",
		Type:      "turn.submitted",
		CreatedAt: time.Date(2026, 6, 30, 1, 2, 3, 0, time.UTC),
		Data:      EventData{InputItems: []InputItem{{Type: "text", Text: "hello"}}},
	})
	if !errors.Is(err, ErrLedgerLocked) {
		t.Fatalf("Append error = %v, want ErrLedgerLocked", err)
	}
}

func TestKernelReadyReportsLedgerLocked(t *testing.T) {
	dir := testTempDir(t)
	ledgerPath := filepath.Join(dir, "events.sqlite")
	lockFile, err := os.OpenFile(ledgerPath+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("create external lock: %v", err)
	}
	defer lockFile.Close()
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     FakeProvider{},
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer k.Close()

	ready := k.Ready()
	if ready.Ledger.Readiness != ReadinessNotReady || ready.Ledger.ReadinessReason != "ledger_locked" {
		t.Fatalf("ledger ready = %+v, want ledger_locked", ready.Ledger)
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
			if key != "session_id" && key != "updated_at" && key != "title" {
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

func removeSQLiteIndexFiles(t *testing.T, ledgerPath string) {
	t.Helper()
	for _, path := range []string{ledgerPath, ledgerPath + "-wal", ledgerPath + "-shm"} {
		err := os.Remove(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("remove %s: %v", path, err)
		}
	}
}
