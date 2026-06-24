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

func (s *FileSourceFailureStore) RecordSourceFailure(_ context.Context, record SourceFailureRecord) error {
	if strings.TrimSpace(record.Connector) == "" {
		return errors.New("source failure connector is required")
	}
	if strings.TrimSpace(record.Reason) == "" {
		return errors.New("source failure reason is required")
	}
	record.Connector = strings.TrimSpace(record.Connector)
	record.EventSource = strings.TrimSpace(record.EventSource)
	record.Reason = strings.TrimSpace(record.Reason)
	record.Detail = strings.TrimSpace(record.Detail)
	record.SourceValidation = strings.TrimSpace(record.SourceValidation)
	if record.SourceValidation == "" {
		record.SourceValidation = SourceValidationRejected
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(record.RecordID) == "" {
		record.RecordID = stableOpaqueID("source_failure", record.Connector, record.EventSource, record.Reason, record.Detail, record.RawExcerpt)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.RecordID] = record
	return s.writeLocked()
}

func (s *FileSourceFailureStore) ListSourceFailures(_ context.Context) ([]SourceFailureRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]SourceFailureRecord, 0, len(s.records))
	for _, record := range s.records {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if !records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].CreatedAt.Before(records[j].CreatedAt)
		}
		return records[i].RecordID < records[j].RecordID
	})
	return records, nil
}

func (s *FileSourceFailureStore) load() error {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var payload fileSourceFailurePayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return err
	}
	if payload.Records != nil {
		s.records = payload.Records
	}
	return nil
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
	return os.WriteFile(s.path, content, 0o600)
}
