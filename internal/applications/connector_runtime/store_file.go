package connectorruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type OutboxStore interface {
	EnqueueCommand(context.Context, AppCommand, time.Time) (ConnectorOutboxItem, bool, error)
	GetOutboxItem(context.Context, string) (ConnectorOutboxItem, error)
	ClaimNextOutboxItem(context.Context, time.Time, string, time.Duration) (ConnectorOutboxItem, bool, error)
	ClaimOutboxItem(context.Context, string, time.Time, string, time.Duration) (ConnectorOutboxItem, bool, error)
	RecordDelivery(context.Context, ConnectorOutboxItem, DeliveryReceipt) error
	RequeueOutboxItem(context.Context, string, string, time.Time) (ConnectorOutboxItem, DeliveryReceipt, error)
	ListOutbox(context.Context) ([]ConnectorOutboxItem, error)
	ListReceipts(context.Context, string) ([]DeliveryReceipt, error)
}

type FileOutboxStore struct {
	path     string
	mu       sync.Mutex
	items    map[string]ConnectorOutboxItem
	byDedupe map[string]string
	receipts map[string][]DeliveryReceipt
}

type fileOutboxPayload struct {
	Items    map[string]ConnectorOutboxItem `json:"items"`
	ByDedupe map[string]string              `json:"by_dedupe"`
	Receipts map[string][]DeliveryReceipt   `json:"receipts"`
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
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileOutboxStore) EnqueueCommand(_ context.Context, command AppCommand, now time.Time) (ConnectorOutboxItem, bool, error) {
	if err := command.validate(); err != nil {
		return ConnectorOutboxItem{}, false, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	dedupeKey := command.outboxDedupeKey()
	s.mu.Lock()
	defer s.mu.Unlock()
	if existingID, ok := s.byDedupe[dedupeKey]; ok {
		return s.items[existingID], true, nil
	}
	item := command.toOutboxItem(now)
	s.items[item.OutboxID] = item
	s.byDedupe[dedupeKey] = item.OutboxID
	if err := s.writeLocked(); err != nil {
		delete(s.items, item.OutboxID)
		delete(s.byDedupe, dedupeKey)
		return ConnectorOutboxItem{}, false, err
	}
	return item, false, nil
}

func (s *FileOutboxStore) GetOutboxItem(_ context.Context, outboxID string) (ConnectorOutboxItem, error) {
	if outboxID == "" {
		return ConnectorOutboxItem{}, errors.New("outbox id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[outboxID]
	if !ok {
		return ConnectorOutboxItem{}, errors.New("outbox item not found")
	}
	return item, nil
}

func (s *FileOutboxStore) ClaimNextOutboxItem(_ context.Context, now time.Time, owner string, leaseDuration time.Duration) (ConnectorOutboxItem, bool, error) {
	if owner == "" {
		return ConnectorOutboxItem{}, false, errors.New("delivery lease owner is required")
	}
	if leaseDuration <= 0 {
		return ConnectorOutboxItem{}, false, errors.New("delivery lease duration must be positive")
	}
	if now.IsZero() {
		now = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
			return ConnectorOutboxItem{}, false, err
		}
		return item, true, nil
	}
	return ConnectorOutboxItem{}, false, nil
}

func (s *FileOutboxStore) ClaimOutboxItem(_ context.Context, outboxID string, now time.Time, owner string, leaseDuration time.Duration) (ConnectorOutboxItem, bool, error) {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[outboxID]
	if !ok {
		return ConnectorOutboxItem{}, false, errors.New("outbox item not found")
	}
	if !deliveryEligible(item, now) {
		return item, false, nil
	}
	item.LeaseID = stableOpaqueID("lease", item.OutboxID, owner, now.Format(time.RFC3339Nano))
	item.LeaseOwner = owner
	item.LeaseExpiresAt = now.Add(leaseDuration)
	item.UpdatedAt = now
	s.items[item.OutboxID] = item
	if err := s.writeLocked(); err != nil {
		return ConnectorOutboxItem{}, false, err
	}
	return item, true, nil
}

func (s *FileOutboxStore) RecordDelivery(_ context.Context, item ConnectorOutboxItem, receipt DeliveryReceipt) error {
	if item.OutboxID == "" || receipt.ReceiptID == "" || receipt.OutboxID == "" {
		return errors.New("outbox item and receipt ids are required")
	}
	if item.OutboxID != receipt.OutboxID {
		return errors.New("receipt outbox id does not match item")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[item.OutboxID]; !ok {
		return errors.New("outbox item not found")
	}
	s.items[item.OutboxID] = item
	s.receipts[item.OutboxID] = append(s.receipts[item.OutboxID], receipt)
	return s.writeLocked()
}

func (s *FileOutboxStore) RequeueOutboxItem(_ context.Context, outboxID string, reason string, now time.Time) (ConnectorOutboxItem, DeliveryReceipt, error) {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[outboxID]
	if !ok {
		return ConnectorOutboxItem{}, DeliveryReceipt{}, errors.New("outbox item not found")
	}
	if item.Status != OutboxStatusDeadLetter {
		return ConnectorOutboxItem{}, DeliveryReceipt{}, errors.New("outbox item is not dead-lettered")
	}
	item.Status = OutboxStatusQueued
	item.NextAttemptAt = time.Time{}
	item.LeaseID = ""
	item.LeaseOwner = ""
	item.LeaseExpiresAt = time.Time{}
	item.UpdatedAt = now
	receipt := DeliveryReceipt{
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
	if err := s.writeLocked(); err != nil {
		return ConnectorOutboxItem{}, DeliveryReceipt{}, err
	}
	return item, receipt, nil
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

func (s *FileOutboxStore) ListOutbox(_ context.Context) ([]ConnectorOutboxItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]ConnectorOutboxItem, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, nil
}

func (s *FileOutboxStore) ListReceipts(_ context.Context, outboxID string) ([]DeliveryReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	receipts := append([]DeliveryReceipt(nil), s.receipts[outboxID]...)
	sort.Slice(receipts, func(i, j int) bool {
		return receipts[i].RecordedAt.Before(receipts[j].RecordedAt)
	})
	return receipts, nil
}

func (s *FileOutboxStore) load() error {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var payload fileOutboxPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return err
	}
	if payload.Items != nil {
		s.items = payload.Items
	}
	if payload.ByDedupe != nil {
		s.byDedupe = payload.ByDedupe
	}
	if payload.Receipts != nil {
		s.receipts = payload.Receipts
	}
	return nil
}

func (s *FileOutboxStore) writeLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := fileOutboxPayload{
		Items:    s.items,
		ByDedupe: s.byDedupe,
		Receipts: s.receipts,
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
	return os.Rename(tmpPath, s.path)
}
