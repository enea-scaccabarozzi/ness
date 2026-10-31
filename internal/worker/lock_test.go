package worker_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

func TestLockRefusesASecondWorkerNamingThePid(t *testing.T) {
	// covers: R-WRK-001
	dir := t.TempDir()
	release, err := worker.Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = worker.Lock(dir)
	if !errors.Is(err, worker.ErrLocked) || !strings.Contains(err.Error(), "pid "+strconv.Itoa(os.Getpid())) {
		t.Fatalf("second Lock = %v", err)
	}
	release()
	release2, err := worker.Lock(dir)
	if err != nil {
		t.Fatalf("Lock after release: %v", err)
	}
	release2()
}

func TestLockPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "install.lock")
	release, err := worker.LockPath(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.LockPath(p); !errors.Is(err, worker.ErrLocked) {
		t.Fatalf("second = %v, want ErrLocked", err)
	}
	release()
	again, err := worker.LockPath(p)
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	again()
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("lock file removed: %v", err)
	}
}
