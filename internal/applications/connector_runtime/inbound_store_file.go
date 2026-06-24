package connectorruntime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type InboundStore interface {
	Reserve(context.Context, InboundSubmissionRecord) (InboundSubmissionRecord, bool, error)
	Complete(context.Context, InboundSubmissionRecord) error
	ListInbound(context.Context) ([]InboundSubmissionRecord, error)
}

type FileInboundStore struct {
	path    string
	mu      sync.Mutex
	records map[string]InboundSubmissionRecord
}

type fileInboundPayload struct {
	Records map[string]InboundSubmissionRecord `json:"records"`
}

func NewFileInboundStore(path string) (*FileInboundStore, error) {
	if path == "" {
		return nil, errors.New("inbound store path is required")
	}
	store := &FileInboundStore{path: path, records: make(map[string]InboundSubmissionRecord)}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileInboundStore) Reserve(_ context.Context, record InboundSubmissionRecord) (InboundSubmissionRecord, bool, error) {
	if record.DedupeKey == "" {
		return InboundSubmissionRecord{}, false, errors.New("inbound submission missing dedupe key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.records[record.DedupeKey]; ok {
		return existing, false, nil
	}
	s.records[record.DedupeKey] = record
	if err := s.writeLocked(); err != nil {
		delete(s.records, record.DedupeKey)
		return InboundSubmissionRecord{}, false, err
	}
	return record, true, nil
}

func (s *FileInboundStore) Complete(_ context.Context, record InboundSubmissionRecord) error {
	if record.DedupeKey == "" {
		return errors.New("inbound submission missing dedupe key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.DedupeKey] = record
	return s.writeLocked()
}

func (s *FileInboundStore) ListInbound(_ context.Context) ([]InboundSubmissionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]InboundSubmissionRecord, 0, len(s.records))
	for _, record := range s.records {
		records = append(records, record)
	}
	return records, nil
}

func (s *FileInboundStore) load() error {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var payload fileInboundPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return err
	}
	if payload.Records != nil {
		s.records = payload.Records
	}
	return nil
}

func (s *FileInboundStore) writeLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := fileInboundPayload{Records: s.records}
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".connector-inbound.*.tmp")
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
