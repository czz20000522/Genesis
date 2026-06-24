package connectorruntime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type SourceFailureStore interface {
	RecordSourceFailure(context.Context, SourceFailureRecord) error
	ListSourceFailures(context.Context) ([]SourceFailureRecord, error)
}

type FileSourceFailureStore struct {
	path    string
	mu      sync.Mutex
	records map[string]SourceFailureRecord
}

type fileSourceFailurePayload struct {
	Records map[string]SourceFailureRecord `json:"records"`
}

func NewFileSourceFailureStore(path string) (*FileSourceFailureStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("source failure store path is required")
	}
	store := &FileSourceFailureStore{path: path, records: map[string]SourceFailureRecord{}}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileSourceFailureStore) RecordSourceFailure(ctx context.Context, record SourceFailureRecord) error {
	if strings.TrimSpace(record.Connector) == "" {
		return errors.New("source failure connector is required")
	}
	if strings.TrimSpace(record.Reason) == "" {
		return errors.New("source failure reason is required")
	}
	record.Connector = strings.TrimSpace(record.Connector)
	record.EventSource = strings.TrimSpace(record.EventSource)
	record.Reason = strings.TrimSpace(record.Reason)
	record.Detail = safeSourceFailureDiagnostic(strings.TrimSpace(record.Detail))
	record.DiagnosticExcerpt = safeSourceFailureDiagnostic(strings.TrimSpace(record.DiagnosticExcerpt))
	record.SourceValidation = strings.TrimSpace(record.SourceValidation)
	if record.SourceValidation == "" {
		record.SourceValidation = SourceValidationRejected
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(record.RecordID) == "" {
		record.RecordID = stableOpaqueID("source_failure", record.Connector, record.EventSource, record.Reason, record.Detail, record.DiagnosticExcerpt)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.withLockedState(ctx, func() error {
		s.records[record.RecordID] = record
		return s.writeLocked()
	})
}

func (s *FileSourceFailureStore) ListSourceFailures(ctx context.Context) ([]SourceFailureRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var records []SourceFailureRecord
	err := s.withLockedState(ctx, func() error {
		records = make([]SourceFailureRecord, 0, len(s.records))
		for _, record := range s.records {
			records = append(records, record)
		}
		sort.Slice(records, func(i, j int) bool {
			if !records[i].CreatedAt.Equal(records[j].CreatedAt) {
				return records[i].CreatedAt.Before(records[j].CreatedAt)
			}
			return records[i].RecordID < records[j].RecordID
		})
		return nil
	})
	return records, err
}

func (s *FileSourceFailureStore) load() error {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.records = map[string]SourceFailureRecord{}
		return nil
	}
	if err != nil {
		return err
	}
	var payload fileSourceFailurePayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return err
	}
	s.records = map[string]SourceFailureRecord{}
	if payload.Records != nil {
		s.records = payload.Records
	}
	return nil
}

func (s *FileSourceFailureStore) withLockedState(ctx context.Context, fn func() error) error {
	release, err := acquireConnectorStateFileLock(ctx, s.path+".lock")
	if err != nil {
		return err
	}
	defer release()
	if err := s.load(); err != nil {
		return err
	}
	return fn()
}

func (s *FileSourceFailureStore) writeLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := fileSourceFailurePayload{Records: s.records}
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".connector-source-failure.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceConnectorStateFile(tmpPath, s.path)
}

func safeSourceFailureDiagnostic(value string) string {
	const limit = 512
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if isCredentialShapedExternalValue(value) {
		return "[redacted credential-shaped source diagnostic]"
	}
	if looksLikeRawStructuredSourcePayload(value) {
		return "[redacted raw source diagnostic]"
	}
	if len(value) > limit {
		return value[:limit] + "\n[truncated]"
	}
	return value
}

func looksLikeRawStructuredSourcePayload(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return false
	}
	return (strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}")) ||
		(strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]"))
}
