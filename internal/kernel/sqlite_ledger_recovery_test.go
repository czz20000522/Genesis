package kernel

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteLedgerRecoveryDoesNotDeleteLockWhileAnotherProcessRecovers(t *testing.T) {
	dir := testTempDir(t)
	lockPath := filepath.Join(dir, "events.sqlite.lock")
	readyPath := filepath.Join(dir, "recovery-ready")
	releasePath := filepath.Join(dir, "recovery-release")
	staleCreatedAt := time.Now().Add(-2 * time.Hour)
	if err := os.WriteFile(lockPath, []byte("pid=0\ncreated_at="+staleCreatedAt.Format(time.RFC3339Nano)+"\n"), 0o644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	child := exec.Command(os.Args[0], "-test.run=^TestSQLiteLedgerRecoveryGuardHelper$")
	child.Env = append(os.Environ(),
		"GENESIS_SQLITE_LEDGER_RECOVERY_GUARD_HELPER=1",
		"GENESIS_SQLITE_LEDGER_RECOVERY_LOCK_PATH="+lockPath,
		"GENESIS_SQLITE_LEDGER_RECOVERY_READY_PATH="+readyPath,
		"GENESIS_SQLITE_LEDGER_RECOVERY_RELEASE_PATH="+releasePath,
	)
	if err := child.Start(); err != nil {
		t.Fatalf("start recovery guard helper: %v", err)
	}
	defer func() {
		_ = os.WriteFile(releasePath, []byte("release"), 0o644)
		_ = child.Wait()
	}()
	waitForSQLiteLedgerTestFile(t, readyPath)

	recovered, err := recoverStaleSQLiteLedgerLock(lockPath, time.Now())
	if err != nil {
		t.Fatalf("recover stale lock: %v", err)
	}
	if recovered {
		t.Fatal("recovered = true while another process holds the recovery guard")
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("stale lock was removed while recovery guard was held: %v", err)
	}
}

func TestSQLiteLedgerRecoveryGuardHelper(t *testing.T) {
	if os.Getenv("GENESIS_SQLITE_LEDGER_RECOVERY_GUARD_HELPER") != "1" {
		return
	}
	lockPath := os.Getenv("GENESIS_SQLITE_LEDGER_RECOVERY_LOCK_PATH")
	readyPath := os.Getenv("GENESIS_SQLITE_LEDGER_RECOVERY_READY_PATH")
	releasePath := os.Getenv("GENESIS_SQLITE_LEDGER_RECOVERY_RELEASE_PATH")
	guard, acquired, err := acquireSQLiteLedgerRecoveryGuard(lockPath)
	if err != nil {
		t.Fatalf("acquire recovery guard: %v", err)
	}
	if !acquired {
		t.Fatal("acquire recovery guard returned acquired=false")
	}
	defer guard.Close()
	if err := os.WriteFile(readyPath, []byte("ready"), 0o644); err != nil {
		t.Fatalf("write recovery readiness: %v", err)
	}
	for {
		if _, err := os.Stat(releasePath); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat recovery release: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForSQLiteLedgerTestFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat %s: %v", path, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}
