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

type SourceLifecycleStore interface {
	UpsertSourceRun(context.Context, SourceRun) error
	ListSourceRuns(context.Context) ([]SourceRun, error)
	RecordSourceAttempt(context.Context, SourceAttempt) error
	ListSourceAttempts(context.Context, string) ([]SourceAttempt, error)
	SaveSourceCursor(context.Context, SourceCursor) error
	GetSourceCursor(context.Context, string, string) (SourceCursor, bool, error)
	ListSourceCursors(context.Context) ([]SourceCursor, error)
	RecordSourceVerification(context.Context, SourceVerificationEvidence) error
	ListSourceVerifications(context.Context) ([]SourceVerificationEvidence, error)
}

type FileSourceLifecycleStore struct {
	path            string
	mu              sync.Mutex
	runs            map[string]SourceRun
	attempts        map[string][]SourceAttempt
	cursors         map[string]SourceCursor
	verifications   map[string]SourceVerificationEvidence
	operatorActions map[string][]SourceOperatorActionRecord
}

type fileSourceLifecyclePayload struct {
	Runs            map[string]SourceRun                    `json:"runs"`
	Attempts        map[string][]SourceAttempt              `json:"attempts"`
	Cursors         map[string]SourceCursor                 `json:"cursors"`
	Verifications   map[string]SourceVerificationEvidence   `json:"verifications,omitempty"`
	OperatorActions map[string][]SourceOperatorActionRecord `json:"operator_actions,omitempty"`
}

func NewFileSourceLifecycleStore(path string) (*FileSourceLifecycleStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("source lifecycle store path is required")
	}
	store := &FileSourceLifecycleStore{path: path}
	store.reset()
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileSourceLifecycleStore) UpsertSourceRun(ctx context.Context, run SourceRun) error {
	run, err := normalizeSourceRun(run)
	if err != nil {
		return err
	}
	return s.withLockedState(ctx, func() error {
		if existing, ok := s.runs[run.SourceID]; ok && !existing.StartedAt.IsZero() {
			run.StartedAt = existing.StartedAt
		}
		s.runs[run.SourceID] = run
		return s.writeLocked()
	})
}

func (s *FileSourceLifecycleStore) ListSourceRuns(ctx context.Context) ([]SourceRun, error) {
	var runs []SourceRun
	err := s.withLockedState(ctx, func() error {
		runs = make([]SourceRun, 0, len(s.runs))
		for _, run := range s.runs {
			runs = append(runs, run)
		}
		sort.Slice(runs, func(i, j int) bool {
			if !runs[i].UpdatedAt.Equal(runs[j].UpdatedAt) {
				return runs[i].UpdatedAt.Before(runs[j].UpdatedAt)
			}
			return runs[i].SourceID < runs[j].SourceID
		})
		return nil
	})
	return runs, err
}

func (s *FileSourceLifecycleStore) RecordSourceAttempt(ctx context.Context, attempt SourceAttempt) error {
	attempt, err := normalizeSourceAttempt(attempt)
	if err != nil {
		return err
	}
	return s.withLockedState(ctx, func() error {
		s.attempts[attempt.SourceRunID] = append(s.attempts[attempt.SourceRunID], attempt)
		sort.Slice(s.attempts[attempt.SourceRunID], func(i, j int) bool {
			return s.attempts[attempt.SourceRunID][i].StartedAt.Before(s.attempts[attempt.SourceRunID][j].StartedAt)
		})
		return s.writeLocked()
	})
}

func (s *FileSourceLifecycleStore) ListSourceAttempts(ctx context.Context, sourceRunID string) ([]SourceAttempt, error) {
	sourceRunID = strings.TrimSpace(sourceRunID)
	if sourceRunID == "" {
		return nil, errors.New("source run id is required")
	}
	var attempts []SourceAttempt
	err := s.withLockedState(ctx, func() error {
		attempts = append([]SourceAttempt(nil), s.attempts[sourceRunID]...)
		sort.Slice(attempts, func(i, j int) bool {
			return attempts[i].StartedAt.Before(attempts[j].StartedAt)
		})
		return nil
	})
	return attempts, err
}

func (s *FileSourceLifecycleStore) SaveSourceCursor(ctx context.Context, cursor SourceCursor) error {
	cursor, err := normalizeSourceCursor(cursor)
	if err != nil {
		return err
	}
	return s.withLockedState(ctx, func() error {
		s.cursors[sourceCursorKey(cursor.SourceID, cursor.CursorKind)] = cursor
		return s.writeLocked()
	})
}

func (s *FileSourceLifecycleStore) GetSourceCursor(ctx context.Context, sourceID string, cursorKind string) (SourceCursor, bool, error) {
	sourceID = strings.TrimSpace(sourceID)
	cursorKind = strings.TrimSpace(cursorKind)
	if sourceID == "" || cursorKind == "" {
		return SourceCursor{}, false, errors.New("source id and cursor kind are required")
	}
	var cursor SourceCursor
	var ok bool
	err := s.withLockedState(ctx, func() error {
		cursor, ok = s.cursors[sourceCursorKey(sourceID, cursorKind)]
		return nil
	})
	return cursor, ok, err
}

func (s *FileSourceLifecycleStore) ListSourceCursors(ctx context.Context) ([]SourceCursor, error) {
	var cursors []SourceCursor
	err := s.withLockedState(ctx, func() error {
		cursors = make([]SourceCursor, 0, len(s.cursors))
		for _, cursor := range s.cursors {
			cursors = append(cursors, cursor)
		}
		sort.Slice(cursors, func(i, j int) bool {
			if !cursors[i].UpdatedAt.Equal(cursors[j].UpdatedAt) {
				return cursors[i].UpdatedAt.Before(cursors[j].UpdatedAt)
			}
			return sourceCursorKey(cursors[i].SourceID, cursors[i].CursorKind) < sourceCursorKey(cursors[j].SourceID, cursors[j].CursorKind)
		})
		return nil
	})
	return cursors, err
}

func (s *FileSourceLifecycleStore) RecordSourceVerification(ctx context.Context, evidence SourceVerificationEvidence) error {
	evidence, err := normalizeSourceVerificationEvidence(evidence)
	if err != nil {
		return err
	}
	return s.withLockedState(ctx, func() error {
		s.verifications[sourceVerificationKey(evidence)] = evidence
		return s.writeLocked()
	})
}

func (s *FileSourceLifecycleStore) ListSourceVerifications(ctx context.Context) ([]SourceVerificationEvidence, error) {
	var evidence []SourceVerificationEvidence
	err := s.withLockedState(ctx, func() error {
		evidence = make([]SourceVerificationEvidence, 0, len(s.verifications))
		for _, item := range s.verifications {
			evidence = append(evidence, item)
		}
		sort.Slice(evidence, func(i, j int) bool {
			if !evidence[i].CheckedAt.Equal(evidence[j].CheckedAt) {
				return evidence[i].CheckedAt.Before(evidence[j].CheckedAt)
			}
			return sourceVerificationKey(evidence[i]) < sourceVerificationKey(evidence[j])
		})
		return nil
	})
	return evidence, err
}

func (s *FileSourceLifecycleStore) load() error {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.reset()
		return nil
	}
	if err != nil {
		return err
	}
	var payload fileSourceLifecyclePayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return err
	}
	s.reset()
	if payload.Runs != nil {
		s.runs = payload.Runs
	}
	if payload.Attempts != nil {
		s.attempts = payload.Attempts
	}
	if payload.Cursors != nil {
		s.cursors = payload.Cursors
	}
	if payload.Verifications != nil {
		s.verifications = payload.Verifications
	}
	if payload.OperatorActions != nil {
		s.operatorActions = payload.OperatorActions
	}
	return nil
}

func (s *FileSourceLifecycleStore) reset() {
	s.runs = make(map[string]SourceRun)
	s.attempts = make(map[string][]SourceAttempt)
	s.cursors = make(map[string]SourceCursor)
	s.verifications = make(map[string]SourceVerificationEvidence)
	s.operatorActions = make(map[string][]SourceOperatorActionRecord)
}

func (s *FileSourceLifecycleStore) withLockedState(ctx context.Context, fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *FileSourceLifecycleStore) writeLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := fileSourceLifecyclePayload{
		Runs:            s.runs,
		Attempts:        s.attempts,
		Cursors:         s.cursors,
		Verifications:   s.verifications,
		OperatorActions: s.operatorActions,
	}
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".connector-source-lifecycle.*.tmp")
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

func normalizeSourceRun(run SourceRun) (SourceRun, error) {
	run.SourceID = strings.TrimSpace(run.SourceID)
	run.Connector = strings.TrimSpace(run.Connector)
	run.AdapterRef = strings.TrimSpace(run.AdapterRef)
	run.Status = strings.TrimSpace(run.Status)
	run.BlockedReasonCode = strings.TrimSpace(run.BlockedReasonCode)
	run.BlockedReason = strings.TrimSpace(run.BlockedReason)
	switch {
	case run.SourceID == "":
		return SourceRun{}, errors.New("source run source id is required")
	case run.Connector == "":
		return SourceRun{}, errors.New("source run connector is required")
	case run.AdapterRef == "":
		return SourceRun{}, errors.New("source run adapter ref is required")
	case !validSourceRunStatus(run.Status):
		return SourceRun{}, errors.New("source run status is invalid")
	case run.BlockedReasonCode != "" && !validSourceReadinessReasonCode(run.BlockedReasonCode):
		return SourceRun{}, errors.New("source run blocked reason code is invalid")
	}
	now := time.Now().UTC()
	if run.StartedAt.IsZero() {
		run.StartedAt = now
	}
	if run.Status == SourceRunStatusReady && run.LastReadyAt.IsZero() {
		run.LastReadyAt = now
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = now
	}
	return run, nil
}

func normalizeSourceAttempt(attempt SourceAttempt) (SourceAttempt, error) {
	attempt.AttemptID = strings.TrimSpace(attempt.AttemptID)
	attempt.SourceRunID = strings.TrimSpace(attempt.SourceRunID)
	attempt.Outcome = strings.TrimSpace(attempt.Outcome)
	attempt.FailureRef = strings.TrimSpace(attempt.FailureRef)
	switch {
	case attempt.SourceRunID == "":
		return SourceAttempt{}, errors.New("source attempt source run id is required")
	case !validSourceAttemptOutcome(attempt.Outcome):
		return SourceAttempt{}, errors.New("source attempt outcome is invalid")
	}
	now := time.Now().UTC()
	if attempt.StartedAt.IsZero() {
		attempt.StartedAt = now
	}
	if attempt.EndedAt.IsZero() {
		attempt.EndedAt = now
	}
	if attempt.AttemptID == "" {
		attempt.AttemptID = stableOpaqueID("source_attempt", attempt.SourceRunID, attempt.Outcome, attempt.StartedAt.Format(time.RFC3339Nano), attempt.EndedAt.Format(time.RFC3339Nano), attempt.FailureRef)
	}
	return attempt, nil
}

func normalizeSourceCursor(cursor SourceCursor) (SourceCursor, error) {
	cursor.SourceID = strings.TrimSpace(cursor.SourceID)
	cursor.CursorKind = strings.TrimSpace(cursor.CursorKind)
	cursor.CursorValue = strings.TrimSpace(cursor.CursorValue)
	if cursor.SourceID == "" || cursor.CursorKind == "" {
		return SourceCursor{}, errors.New("source cursor source id and kind are required")
	}
	if cursor.CursorValue == "" {
		return SourceCursor{}, errors.New("source cursor value is required")
	}
	if cursor.UpdatedAt.IsZero() {
		cursor.UpdatedAt = time.Now().UTC()
	}
	return cursor, nil
}

func normalizeSourceVerificationEvidence(evidence SourceVerificationEvidence) (SourceVerificationEvidence, error) {
	evidence.SourceEventRef = strings.TrimSpace(evidence.SourceEventRef)
	evidence.SourceBatchRef = strings.TrimSpace(evidence.SourceBatchRef)
	evidence.SourceID = strings.TrimSpace(evidence.SourceID)
	evidence.Connector = strings.TrimSpace(evidence.Connector)
	evidence.ValidationStatus = strings.TrimSpace(evidence.ValidationStatus)
	evidence.EvidenceKind = strings.TrimSpace(evidence.EvidenceKind)
	evidence.EvidenceRef = strings.TrimSpace(evidence.EvidenceRef)
	evidence.AdapterRef = strings.TrimSpace(evidence.AdapterRef)
	switch {
	case evidence.SourceEventRef == "" && evidence.SourceBatchRef == "":
		return SourceVerificationEvidence{}, errors.New("source verification event or batch ref is required")
	case evidence.SourceID == "":
		return SourceVerificationEvidence{}, errors.New("source verification source id is required")
	case evidence.Connector == "":
		return SourceVerificationEvidence{}, errors.New("source verification connector is required")
	case evidence.AdapterRef == "":
		return SourceVerificationEvidence{}, errors.New("source verification adapter ref is required")
	case evidence.ValidationStatus == "":
		return SourceVerificationEvidence{}, errors.New("source verification status is required")
	case evidence.ValidationStatus == SourceValidationVerified && (evidence.EvidenceKind == "" || evidence.EvidenceRef == ""):
		return SourceVerificationEvidence{}, errors.New("verified source event requires evidence kind and ref")
	case evidence.EvidenceKind != "" && !validSourceEvidenceKind(evidence.EvidenceKind):
		return SourceVerificationEvidence{}, errors.New("source verification evidence kind is invalid")
	case evidence.ValidationStatus != SourceValidationVerified && evidence.ValidationStatus != SourceValidationUnchecked && evidence.ValidationStatus != SourceValidationRejected:
		return SourceVerificationEvidence{}, errors.New("source verification status is invalid")
	}
	if evidence.CheckedAt.IsZero() {
		evidence.CheckedAt = time.Now().UTC()
	}
	return evidence, nil
}

func sourceVerificationKey(evidence SourceVerificationEvidence) string {
	if strings.TrimSpace(evidence.SourceEventRef) != "" {
		return "event:" + strings.TrimSpace(evidence.SourceEventRef)
	}
	return "batch:" + strings.TrimSpace(evidence.SourceBatchRef)
}

func sourceCursorKey(sourceID string, cursorKind string) string {
	return strings.TrimSpace(sourceID) + "\x00" + strings.TrimSpace(cursorKind)
}

func validSourceRunStatus(status string) bool {
	switch status {
	case SourceRunStatusStarting, SourceRunStatusReady, SourceRunStatusDegraded, SourceRunStatusBlocked, SourceRunStatusStopped:
		return true
	default:
		return false
	}
}

func validSourceAttemptOutcome(outcome string) bool {
	switch outcome {
	case SourceAttemptOutcomeReady, SourceAttemptOutcomeFailed, SourceAttemptOutcomeBlocked, SourceAttemptOutcomeStopped:
		return true
	default:
		return false
	}
}

func validSourceEvidenceKind(kind string) bool {
	switch kind {
	case SourceEvidenceKindWebhookSignature, SourceEvidenceKindProviderEventSignature, SourceEvidenceKindTrustedLocalAdapterAttestation:
		return true
	default:
		return false
	}
}

func ValidSourceReadinessReasonCode(code string) bool {
	return validSourceReadinessReasonCode(code)
}

func validSourceReadinessReasonCode(code string) bool {
	switch code {
	case SourceReadinessReasonMissingProfile,
		SourceReadinessReasonProfileExpired,
		SourceReadinessReasonPermissionDenied,
		SourceReadinessReasonRefreshRequired,
		SourceReadinessReasonOperatorActionRequired,
		SourceReadinessReasonSourceCommandInvalid,
		SourceReadinessReasonSourceRuntimeFailed:
		return true
	default:
		return false
	}
}
