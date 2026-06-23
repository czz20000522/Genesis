package messageingress

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type InboundStore interface {
	Reserve(context.Context, SubmissionRecord) (SubmissionRecord, bool, error)
	Complete(context.Context, SubmissionRecord) error
}

type FileInboundStore struct {
	path    string
	mu      sync.Mutex
	records map[string]SubmissionRecord
}

type fileStorePayload struct {
	Records map[string]SubmissionRecord `json:"records"`
}

func NewFileInboundStore(path string) (*FileInboundStore, error) {
	if path == "" {
		return nil, errors.New("inbound store path is required")
	}
	store := &FileInboundStore{path: path, records: make(map[string]SubmissionRecord)}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileInboundStore) Reserve(_ context.Context, record SubmissionRecord) (SubmissionRecord, bool, error) {
	if record.RawKey == "" {
		return SubmissionRecord{}, false, errors.New("submission record missing raw key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.records[record.RawKey]; ok {
		return existing, false, nil
	}
	s.records[record.RawKey] = record
	if err := s.writeLocked(); err != nil {
		delete(s.records, record.RawKey)
		return SubmissionRecord{}, false, err
	}
	return record, true, nil
}

func (s *FileInboundStore) Complete(_ context.Context, record SubmissionRecord) error {
	if record.RawKey == "" {
		return errors.New("submission record missing raw key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.RawKey] = record
	return s.writeLocked()
}

func (s *FileInboundStore) load() error {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var payload fileStorePayload
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
	payload := fileStorePayload{Records: s.records}
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".message-ingress.*.tmp")
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
	return os.Rename(tmpPath, s.path)
}
