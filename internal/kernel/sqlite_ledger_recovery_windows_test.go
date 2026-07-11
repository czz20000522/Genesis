//go:build windows

package kernel

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
)

func TestSQLiteLedgerWindowsProcessHandleTreatsExitedStillActiveCodeAsDead(t *testing.T) {
	child := exec.Command(os.Args[0], "-test.run=^TestSQLiteLedgerExitedStillActiveHelper$")
	child.Env = append(os.Environ(), "GENESIS_SQLITE_LEDGER_EXIT_STILL_ACTIVE_HELPER=1")
	if err := child.Start(); err != nil {
		t.Fatalf("start exit code 259 helper: %v", err)
	}
	defer child.Wait()
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(child.Process.Pid))
	if err != nil {
		t.Fatalf("open helper process: %v", err)
	}
	defer syscall.CloseHandle(handle)
	result, err := syscall.WaitForSingleObject(handle, 5000)
	if err != nil {
		t.Fatalf("wait for exit code 259 helper: %v", err)
	}
	if result != syscall.WAIT_OBJECT_0 {
		t.Fatalf("wait result = %d, want WAIT_OBJECT_0", result)
	}

	live, known := sqliteLedgerWindowsProcessHandleLiveness(handle)
	if !known || live {
		t.Fatalf("handle liveness after exit code 259 = live:%t known:%t, want live:false known:true", live, known)
	}
}

func TestSQLiteLedgerExitedStillActiveHelper(t *testing.T) {
	if os.Getenv("GENESIS_SQLITE_LEDGER_EXIT_STILL_ACTIVE_HELPER") == "1" {
		os.Exit(259)
	}
}
