package connectorruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"genesis/internal/testsupport"
)

func TestFileOutboxStoreConcurrentInstancesPreserveIndependentEnqueues(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "outbox-concurrent-enqueue"), "connector-outbox.json")
	first, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("first NewFileOutboxStore returned error: %v", err)
	}
	second, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("second NewFileOutboxStore returned error: %v", err)
	}

	if _, _, err := first.EnqueueCommand(ctx, testSendMessageCommand(), time.Now()); err != nil {
		t.Fatalf("first EnqueueCommand returned error: %v", err)
	}
	secondCommand := testSendMessageCommand()
	secondCommand.CommandID = "cmd_second"
	secondCommand.DedupeKey = "dedupe_second"
	secondCommand.Body = "second message"
	if _, _, err := second.EnqueueCommand(ctx, secondCommand, time.Now().Add(time.Second)); err != nil {
		t.Fatalf("second EnqueueCommand returned error: %v", err)
	}

	reloaded, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("reload NewFileOutboxStore returned error: %v", err)
	}
	items, err := reloaded.ListOutbox(ctx)
	if err != nil {
		t.Fatalf("ListOutbox returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items after two independent writers = %+v, want both writes preserved", items)
	}
}

func TestFileOutboxStoreConcurrentInstancesPreserveReceiptHistory(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "outbox-concurrent-receipts"), "connector-outbox.json")
	seed, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("seed NewFileOutboxStore returned error: %v", err)
	}
	item, _, err := seed.EnqueueCommand(ctx, testSendMessageCommand(), time.Now())
	if err != nil {
		t.Fatalf("seed EnqueueCommand returned error: %v", err)
	}
	first, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("first NewFileOutboxStore returned error: %v", err)
	}
	second, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("second NewFileOutboxStore returned error: %v", err)
	}

	firstReceipt := DeliveryReceipt{
		ReceiptID:  "receipt_first",
		OutboxID:   item.OutboxID,
		Connector:  item.Connector,
		Status:     DeliveryStatusSent,
		Attempt:    1,
		RecordedAt: time.Now(),
	}
	if err := first.RecordDelivery(ctx, item, firstReceipt); err != nil {
		t.Fatalf("first RecordDelivery returned error: %v", err)
	}
	secondReceipt := DeliveryReceipt{
		ReceiptID:  "receipt_second",
		OutboxID:   item.OutboxID,
		Connector:  item.Connector,
		Status:     DeliveryStatusFailed,
		Attempt:    2,
		RecordedAt: time.Now().Add(time.Second),
	}
	if err := second.RecordDelivery(ctx, item, secondReceipt); err != nil {
		t.Fatalf("second RecordDelivery returned error: %v", err)
	}

	reloaded, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("reload NewFileOutboxStore returned error: %v", err)
	}
	receipts, err := reloaded.ListReceipts(ctx, item.OutboxID)
	if err != nil {
		t.Fatalf("ListReceipts returned error: %v", err)
	}
	if len(receipts) != 2 {
		t.Fatalf("receipts after two independent writers = %+v, want both receipts preserved", receipts)
	}
}

func TestFileOutboxStoreConcurrentInstancesDoNotClaimSameItem(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "outbox-concurrent-claim"), "connector-outbox.json")
	seed, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("seed NewFileOutboxStore returned error: %v", err)
	}
	if _, _, err := seed.EnqueueCommand(ctx, testSendMessageCommand(), time.Now()); err != nil {
		t.Fatalf("seed EnqueueCommand returned error: %v", err)
	}
	first, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("first NewFileOutboxStore returned error: %v", err)
	}
	second, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("second NewFileOutboxStore returned error: %v", err)
	}

	claimed, ok, err := first.ClaimNextOutboxItem(ctx, time.Now(), "worker-one", time.Minute)
	if err != nil {
		t.Fatalf("first ClaimNextOutboxItem returned error: %v", err)
	}
	if !ok {
		t.Fatal("first ClaimNextOutboxItem should claim the queued item")
	}
	secondClaim, ok, err := second.ClaimNextOutboxItem(ctx, time.Now().Add(time.Second), "worker-two", time.Minute)
	if err != nil {
		t.Fatalf("second ClaimNextOutboxItem returned error: %v", err)
	}
	if ok {
		t.Fatalf("second ClaimNextOutboxItem claimed %+v after first already claimed %+v", secondClaim, claimed)
	}
}

func TestFileOutboxStoreTakesOverStaleInvalidLockWithoutDataLoss(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "outbox-stale-lock-takeover"), "connector-outbox.json")
	seed, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("seed NewFileOutboxStore returned error: %v", err)
	}
	firstCommand := testSendMessageCommand()
	if _, _, err := seed.EnqueueCommand(ctx, firstCommand, time.Now()); err != nil {
		t.Fatalf("seed EnqueueCommand returned error: %v", err)
	}
	writeConnectorStateLockFile(t, path+".lock", -1, time.Now().Add(-time.Hour))

	recovered, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("recovered NewFileOutboxStore returned error: %v", err)
	}
	secondCommand := testSendMessageCommand()
	secondCommand.CommandID = "cmd_after_stale_lock"
	secondCommand.DedupeKey = "dedupe_after_stale_lock"
	secondCommand.Body = "after stale lock"
	if _, _, err := recovered.EnqueueCommand(ctx, secondCommand, time.Now().Add(time.Second)); err != nil {
		t.Fatalf("EnqueueCommand after stale lock returned error: %v", err)
	}

	reloaded, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("reload NewFileOutboxStore returned error: %v", err)
	}
	items, err := reloaded.ListOutbox(ctx)
	if err != nil {
		t.Fatalf("ListOutbox returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items after stale lock recovery = %+v, want original plus recovered write", items)
	}
	if _, err := os.Stat(path + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale lock should be removed after recovery, stat err=%v", err)
	}
}

func TestFileOutboxStoreDoesNotStealLiveLock(t *testing.T) {
	path := filepath.Join(testsupport.ProjectTempDir(t, "outbox-live-lock-retained"), "connector-outbox.json")
	store, err := NewFileOutboxStore(path)
	if err != nil {
		t.Fatalf("NewFileOutboxStore returned error: %v", err)
	}
	writeConnectorStateLockFile(t, path+".lock", os.Getpid(), time.Now())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, _, err = store.EnqueueCommand(ctx, testSendMessageCommand(), time.Now())
	if err == nil {
		t.Fatal("EnqueueCommand should not steal a live lock")
	}
	if !strings.Contains(err.Error(), "connector state lock unavailable") {
		t.Fatalf("error = %v, want connector state lock unavailable", err)
	}
	if _, statErr := os.Stat(path + ".lock"); statErr != nil {
		t.Fatalf("live lock should remain, stat err=%v", statErr)
	}
}

func writeConnectorStateLockFile(t *testing.T, path string, pid int, createdAt time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir lock dir: %v", err)
	}
	content := "pid=" + strconv.Itoa(pid) + "\ncreated_at=" + createdAt.Format(time.RFC3339Nano) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
}
