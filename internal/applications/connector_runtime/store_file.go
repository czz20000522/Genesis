package connectorruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	fileConnectorStoreLockTimeout  = 10 * time.Second
	fileConnectorStoreLockPoll     = 10 * time.Millisecond
	fileConnectorStoreLockStaleAge = 2 * time.Minute
)

type OutboxStore interface {
	EnqueueCommand(context.Context, AppCommand, time.Time) (ConnectorOutboxItem, bool, error)
	GetOutboxItem(context.Context, string) (ConnectorOutboxItem, error)
	ClaimNextOutboxItem(context.Context, time.Time, string, time.Duration) (ConnectorOutboxItem, bool, error)
	ClaimOutboxItem(context.Context, string, time.Time, string, time.Duration) (ConnectorOutboxItem, bool, error)
	RecordDelivery(context.Context, ConnectorOutboxItem, DeliveryReceipt) error
	RequeueOutboxItem(context.Context, string, string, time.Time) (ConnectorOutboxItem, DeliveryReceipt, error)
	ResolveRecoveryRequiredOutboxItem(context.Context, string, string, string, string, time.Time) (ConnectorOutboxItem, DeliveryReceipt, error)
	RecordReconciliationEvidence(context.Context, ReconciliationEvidence) error
	ListReconciliationEvidence(context.Context, string) ([]ReconciliationEvidence, error)
	ListOutbox(context.Context) ([]ConnectorOutboxItem, error)
	ListReceipts(context.Context, string) ([]DeliveryReceipt, error)
}

type FileOutboxStore struct {
	path     string
	mu       sync.Mutex
	items    map[string]ConnectorOutboxItem
	byDedupe map[string]string
	receipts map[string][]DeliveryReceipt
	probes   map[string][]ReconciliationEvidence
}

type fileOutboxPayload struct {
	Items    map[string]ConnectorOutboxItem      `json:"items"`
	ByDedupe map[string]string                   `json:"by_dedupe"`
	Receipts map[string][]DeliveryReceipt        `json:"receipts"`
	Probes   map[string][]ReconciliationEvidence `json:"reconciliation_evidence,omitempty"`
}

func NewFileOutboxStore(path string) (*FileOutboxStore, error) {
	if path == "" {
		return nil, errors.New("outbox store path is required")
	}
	store := &FileOutboxStore{
		path:     path,
		items:    make(map[string]ConnectorOutboxItem),
		byDedupe: make(map[string]string),
		receipts: make(map[string][]DeliveryReceipt),
		probes:   make(map[string][]ReconciliationEvidence),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileOutboxStore) EnqueueCommand(ctx context.Context, command AppCommand, now time.Time) (ConnectorOutboxItem, bool, error) {
	if err := command.validate(); err != nil {
		return ConnectorOutboxItem{}, false, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	dedupeKey := command.outboxDedupeKey()
	var item ConnectorOutboxItem
	var duplicate bool
	err := s.withLockedState(ctx, func() error {
		if existingID, ok := s.byDedupe[dedupeKey]; ok {
			item = s.items[existingID]
			duplicate = true
			return nil
		}
		item = command.toOutboxItem(now)
		s.items[item.OutboxID] = item
		s.byDedupe[dedupeKey] = item.OutboxID
		if err := s.writeLocked(); err != nil {
			delete(s.items, item.OutboxID)
			delete(s.byDedupe, dedupeKey)
			return err
		}
		return nil
	})
	return item, duplicate, err
}

func (s *FileOutboxStore) GetOutboxItem(ctx context.Context, outboxID string) (ConnectorOutboxItem, error) {
	if outboxID == "" {
		return ConnectorOutboxItem{}, errors.New("outbox id is required")
	}
	var item ConnectorOutboxItem
	err := s.withLockedState(ctx, func() error {
		var ok bool
		item, ok = s.items[outboxID]
		if !ok {
			return errors.New("outbox item not found")
		}
		return nil
	})
	return item, err
}

func (s *FileOutboxStore) ClaimNextOutboxItem(ctx context.Context, now time.Time, owner string, leaseDuration time.Duration) (ConnectorOutboxItem, bool, error) {
	if owner == "" {
		return ConnectorOutboxItem{}, false, errors.New("delivery lease owner is required")
	}
	if leaseDuration <= 0 {
		return ConnectorOutboxItem{}, false, errors.New("delivery lease duration must be positive")
	}
	if now.IsZero() {
		now = time.Now()
	}
	var claimed ConnectorOutboxItem
	var ok bool
	err := s.withLockedState(ctx, func() error {
		items := make([]ConnectorOutboxItem, 0, len(s.items))
		for _, item := range s.items {
			items = append(items, item)
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		})
		for _, item := range items {
			if !deliveryEligible(item, now) {
				continue
			}
			item.LeaseID = stableOpaqueID("lease", item.OutboxID, owner, now.Format(time.RFC3339Nano))
			item.LeaseOwner = owner
			item.LeaseExpiresAt = now.Add(leaseDuration)
			item.UpdatedAt = now
			s.items[item.OutboxID] = item
			if err := s.writeLocked(); err != nil {
				return err
			}
			claimed = item
			ok = true
			return nil
		}
		return nil
	})
	return claimed, ok, err
}

func (s *FileOutboxStore) ClaimOutboxItem(ctx context.Context, outboxID string, now time.Time, owner string, leaseDuration time.Duration) (ConnectorOutboxItem, bool, error) {
	if outboxID == "" {
		return ConnectorOutboxItem{}, false, errors.New("outbox id is required")
	}
	if owner == "" {
		return ConnectorOutboxItem{}, false, errors.New("delivery lease owner is required")
	}
	if leaseDuration <= 0 {
		return ConnectorOutboxItem{}, false, errors.New("delivery lease duration must be positive")
	}
	if now.IsZero() {
		now = time.Now()
	}
	var item ConnectorOutboxItem
	var claimed bool
	err := s.withLockedState(ctx, func() error {
		var ok bool
		item, ok = s.items[outboxID]
		if !ok {
			return errors.New("outbox item not found")
		}
		if !deliveryEligible(item, now) {
			return nil
		}
		item.LeaseID = stableOpaqueID("lease", item.OutboxID, owner, now.Format(time.RFC3339Nano))
		item.LeaseOwner = owner
		item.LeaseExpiresAt = now.Add(leaseDuration)
		item.UpdatedAt = now
		s.items[item.OutboxID] = item
		if err := s.writeLocked(); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	return item, claimed, err
}

func (s *FileOutboxStore) RecordDelivery(ctx context.Context, item ConnectorOutboxItem, receipt DeliveryReceipt) error {
	if item.OutboxID == "" || receipt.ReceiptID == "" || receipt.OutboxID == "" {
		return errors.New("outbox item and receipt ids are required")
	}
	if item.OutboxID != receipt.OutboxID {
		return errors.New("receipt outbox id does not match item")
	}
	return s.withLockedState(ctx, func() error {
		if _, ok := s.items[item.OutboxID]; !ok {
			return errors.New("outbox item not found")
		}
		s.items[item.OutboxID] = item
		s.receipts[item.OutboxID] = append(s.receipts[item.OutboxID], receipt)
		return s.writeLocked()
	})
}

func (s *FileOutboxStore) RequeueOutboxItem(ctx context.Context, outboxID string, reason string, now time.Time) (ConnectorOutboxItem, DeliveryReceipt, error) {
	if outboxID == "" {
		return ConnectorOutboxItem{}, DeliveryReceipt{}, errors.New("outbox id is required")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "operator_requeued"
	}
	if !safeConnectorCommandReason(reason) {
		return ConnectorOutboxItem{}, DeliveryReceipt{}, errors.New("operator recovery reason is unsafe")
	}
	if now.IsZero() {
		now = time.Now()
	}
	var item ConnectorOutboxItem
	var receipt DeliveryReceipt
	err := s.withLockedState(ctx, func() error {
		var ok bool
		item, ok = s.items[outboxID]
		if !ok {
			return errors.New("outbox item not found")
		}
		if item.Status != OutboxStatusDeadLetter {
			return errors.New("outbox item is not dead-lettered")
		}
		item.Status = OutboxStatusQueued
		item.NextAttemptAt = time.Time{}
		item.LeaseID = ""
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		item.UpdatedAt = now
		receipt = DeliveryReceipt{
			ReceiptID:  stableOpaqueID("receipt", item.OutboxID, DeliveryStatusRetrying, reason, fmt.Sprint(item.AttemptCount), now.Format(time.RFC3339Nano)),
			OutboxID:   item.OutboxID,
			Connector:  item.Connector,
			Status:     DeliveryStatusRetrying,
			Reason:     reason,
			Attempt:    item.AttemptCount,
			RecordedAt: now,
		}
		item.LastReceiptID = receipt.ReceiptID
		s.items[item.OutboxID] = item
		s.receipts[item.OutboxID] = append(s.receipts[item.OutboxID], receipt)
		return s.writeLocked()
	})
	return item, receipt, err
}

func (s *FileOutboxStore) ResolveRecoveryRequiredOutboxItem(ctx context.Context, outboxID string, outcome string, reason string, externalActionRef string, now time.Time) (ConnectorOutboxItem, DeliveryReceipt, error) {
	if outboxID == "" {
		return ConnectorOutboxItem{}, DeliveryReceipt{}, errors.New("outbox id is required")
	}
	outcome = strings.TrimSpace(outcome)
	if outcome != DeliveryStatusSent && outcome != DeliveryStatusDeadLettered {
		return ConnectorOutboxItem{}, DeliveryReceipt{}, errors.New("operator recovery outcome must be sent or dead_lettered")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "operator_resolved_" + outcome
	}
	if !safeConnectorCommandReason(reason) {
		return ConnectorOutboxItem{}, DeliveryReceipt{}, errors.New("operator recovery reason is unsafe")
	}
	externalActionRef = strings.TrimSpace(externalActionRef)
	if externalActionRef != "" && !safeExternalActionRef(externalActionRef) {
		return ConnectorOutboxItem{}, DeliveryReceipt{}, errors.New("operator recovery external action ref is unsafe")
	}
	if now.IsZero() {
		now = time.Now()
	}
	var item ConnectorOutboxItem
	var receipt DeliveryReceipt
	err := s.withLockedState(ctx, func() error {
		var ok bool
		item, ok = s.items[outboxID]
		if !ok {
			return errors.New("outbox item not found")
		}
		if item.Status != OutboxStatusRecoveryRequired {
			return errors.New("outbox item is not recovery-required")
		}
		item.NextAttemptAt = time.Time{}
		item.LeaseID = ""
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		item.UpdatedAt = now
		switch outcome {
		case DeliveryStatusSent:
			item.Status = OutboxStatusSent
		case DeliveryStatusDeadLettered:
			item.Status = OutboxStatusDeadLetter
		}
		receipt = DeliveryReceipt{
			ReceiptID:         stableOpaqueID("receipt", item.OutboxID, outcome, externalActionRef, reason, fmt.Sprint(item.AttemptCount), now.Format(time.RFC3339Nano)),
			OutboxID:          item.OutboxID,
			Connector:         item.Connector,
			ExternalActionRef: externalActionRef,
			Status:            outcome,
			Reason:            reason,
			Attempt:           item.AttemptCount,
			RecordedAt:        now,
		}
		item.LastReceiptID = receipt.ReceiptID
		s.items[item.OutboxID] = item
		s.receipts[item.OutboxID] = append(s.receipts[item.OutboxID], receipt)
		return s.writeLocked()
	})
	return item, receipt, err
}

func (s *FileOutboxStore) RecordReconciliationEvidence(ctx context.Context, evidence ReconciliationEvidence) error {
	evidence, err := normalizeReconciliationEvidence(evidence)
	if err != nil {
		return err
	}
	return s.withLockedState(ctx, func() error {
		if _, ok := s.items[evidence.OutboxID]; !ok {
			return errors.New("outbox item not found")
		}
		s.probes[evidence.OutboxID] = append(s.probes[evidence.OutboxID], evidence)
		return s.writeLocked()
	})
}

func (s *FileOutboxStore) ListReconciliationEvidence(ctx context.Context, outboxID string) ([]ReconciliationEvidence, error) {
	var evidence []ReconciliationEvidence
	err := s.withLockedState(ctx, func() error {
		if outboxID != "" {
			evidence = append([]ReconciliationEvidence(nil), s.probes[outboxID]...)
		} else {
			for _, entries := range s.probes {
				evidence = append(evidence, entries...)
			}
		}
		sort.Slice(evidence, func(i, j int) bool {
			return evidence[i].CheckedAt.Before(evidence[j].CheckedAt)
		})
		return nil
	})
	return evidence, err
}

func deliveryEligible(item ConnectorOutboxItem, now time.Time) bool {
	if deliveryLeaseActive(item, now) {
		return false
	}
	return item.Status == OutboxStatusQueued || (item.Status == OutboxStatusRetrying && !item.NextAttemptAt.After(now))
}

func deliveryLeaseActive(item ConnectorOutboxItem, now time.Time) bool {
	return item.LeaseID != "" && item.LeaseExpiresAt.After(now)
}

func (s *FileOutboxStore) ListOutbox(ctx context.Context) ([]ConnectorOutboxItem, error) {
	var items []ConnectorOutboxItem
	err := s.withLockedState(ctx, func() error {
		items = make([]ConnectorOutboxItem, 0, len(s.items))
		for _, item := range s.items {
			items = append(items, item)
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		})
		return nil
	})
	return items, err
}

func (s *FileOutboxStore) ListReceipts(ctx context.Context, outboxID string) ([]DeliveryReceipt, error) {
	var receipts []DeliveryReceipt
	err := s.withLockedState(ctx, func() error {
		receipts = append([]DeliveryReceipt(nil), s.receipts[outboxID]...)
		sort.Slice(receipts, func(i, j int) bool {
			return receipts[i].RecordedAt.Before(receipts[j].RecordedAt)
		})
		return nil
	})
	return receipts, err
}

func (s *FileOutboxStore) load() error {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.reset()
		return nil
	}
	if err != nil {
		return err
	}
	var payload fileOutboxPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return err
	}
	s.reset()
	if payload.Items != nil {
		s.items = payload.Items
	}
	if payload.ByDedupe != nil {
		s.byDedupe = payload.ByDedupe
	}
	if payload.Receipts != nil {
		s.receipts = payload.Receipts
	}
	if payload.Probes != nil {
		s.probes = payload.Probes
	}
	return nil
}

func (s *FileOutboxStore) reset() {
	s.items = make(map[string]ConnectorOutboxItem)
	s.byDedupe = make(map[string]string)
	s.receipts = make(map[string][]DeliveryReceipt)
	s.probes = make(map[string][]ReconciliationEvidence)
}

func (s *FileOutboxStore) withLockedState(ctx context.Context, fn func() error) error {
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

func (s *FileOutboxStore) writeLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := fileOutboxPayload{
		Items:    s.items,
		ByDedupe: s.byDedupe,
		Receipts: s.receipts,
		Probes:   s.probes,
	}
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".connector-outbox.*.tmp")
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

func acquireConnectorStateFileLock(ctx context.Context, path string) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, fileConnectorStoreLockTimeout)
		defer cancel()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	for {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(file, "pid=%d\ncreated_at=%s\n", os.Getpid(), time.Now().Format(time.RFC3339Nano))
			if closeErr := file.Close(); closeErr != nil {
				_ = os.Remove(path)
				return nil, closeErr
			}
			return func() {
				_ = os.Remove(path)
			}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		recovered, recoverErr := recoverStaleConnectorStateFileLock(path, time.Now())
		if recoverErr != nil {
			return nil, recoverErr
		}
		if recovered {
			continue
		}
		timer := time.NewTimer(fileConnectorStoreLockPoll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("connector state lock unavailable: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

type connectorStateFileLockRecord struct {
	PID       int
	CreatedAt time.Time
}

func recoverStaleConnectorStateFileLock(path string, now time.Time) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	record := parseConnectorStateFileLockRecord(content)
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = info.ModTime()
	}
	if createdAt.IsZero() || now.Sub(createdAt) < fileConnectorStoreLockStaleAge {
		return false, nil
	}
	if record.PID > 0 {
		live, known := connectorProcessLiveness(record.PID)
		if !known || live {
			return false, nil
		}
	}
	current, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if !bytes.Equal(content, current) {
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func parseConnectorStateFileLockRecord(content []byte) connectorStateFileLockRecord {
	var record connectorStateFileLockRecord
	for _, line := range strings.Split(string(content), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "pid":
			pid, err := strconv.Atoi(strings.TrimSpace(value))
			if err == nil {
				record.PID = pid
			}
		case "created_at":
			createdAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
			if err == nil {
				record.CreatedAt = createdAt
			}
		}
	}
	return record
}
