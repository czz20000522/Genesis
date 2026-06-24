package connectorruntime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"genesis/internal/testsupport"
)

func TestFileSourceFailureStoreConcurrentInstancesPreserveIndependentRecords(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "source-failure-concurrent"), "source-failures.json")
	first, err := NewFileSourceFailureStore(path)
	if err != nil {
		t.Fatalf("first NewFileSourceFailureStore returned error: %v", err)
	}
	second, err := NewFileSourceFailureStore(path)
	if err != nil {
		t.Fatalf("second NewFileSourceFailureStore returned error: %v", err)
	}

	if err := first.RecordSourceFailure(ctx, testSourceFailureRecord("first", "missing sender")); err != nil {
		t.Fatalf("first RecordSourceFailure returned error: %v", err)
	}
	if err := second.RecordSourceFailure(ctx, testSourceFailureRecord("second", "bad timestamp")); err != nil {
		t.Fatalf("second RecordSourceFailure returned error: %v", err)
	}

	reloaded, err := NewFileSourceFailureStore(path)
	if err != nil {
		t.Fatalf("reload NewFileSourceFailureStore returned error: %v", err)
	}
	records, err := reloaded.ListSourceFailures(ctx)
	if err != nil {
		t.Fatalf("ListSourceFailures returned error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records after two independent writers = %+v, want both writes preserved", records)
	}
}

func TestFileSourceFailureStoreRedactsCredentialShapedDiagnostics(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "source-failure-redaction"), "source-failures.json")
	store, err := NewFileSourceFailureStore(path)
	if err != nil {
		t.Fatalf("NewFileSourceFailureStore returned error: %v", err)
	}
	if err := store.RecordSourceFailure(ctx, SourceFailureRecord{
		Connector:         "feishu",
		EventSource:       "feishu.message.receive",
		Reason:            "source_runtime_error",
		Detail:            "Authorization: Bearer sk-secret",
		DiagnosticExcerpt: "token=sk-secret",
		SourceValidation:  SourceValidationUnchecked,
		CreatedAt:         time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordSourceFailure returned error: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read source failure store: %v", err)
	}
	for _, forbidden := range []string{"Authorization", "Bearer", "sk-secret", "token="} {
		if strings.Contains(string(content), forbidden) {
			t.Fatalf("source failure store leaked %q:\n%s", forbidden, string(content))
		}
	}
}

func TestFileSourceFailureStoreRedactsStructuredRawDiagnostics(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "source-failure-structured-redaction"), "source-failures.json")
	store, err := NewFileSourceFailureStore(path)
	if err != nil {
		t.Fatalf("NewFileSourceFailureStore returned error: %v", err)
	}
	if err := store.RecordSourceFailure(ctx, SourceFailureRecord{
		Connector:         "feishu",
		EventSource:       "feishu.message.receive",
		Reason:            "malformed_source_event",
		Detail:            `{"event_id":"evt_secret","content":"private body"}`,
		DiagnosticExcerpt: `{"chat_id":"oc_secret","text":"private body"}`,
		SourceValidation:  SourceValidationRejected,
		CreatedAt:         time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordSourceFailure returned error: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read source failure store: %v", err)
	}
	for _, forbidden := range []string{"evt_secret", "oc_secret", "private body"} {
		if strings.Contains(string(content), forbidden) {
			t.Fatalf("source failure store leaked %q:\n%s", forbidden, string(content))
		}
	}
}

func testSourceFailureRecord(id string, detail string) SourceFailureRecord {
	return SourceFailureRecord{
		RecordID:          "source_failure_" + id,
		Connector:         "feishu",
		EventSource:       "feishu.message.receive",
		Reason:            "malformed_source_event",
		Detail:            detail,
		DiagnosticExcerpt: detail + "; source_bytes=12",
		SourceValidation:  SourceValidationRejected,
		CreatedAt:         time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
	}
}
