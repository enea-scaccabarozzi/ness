package worker_test

import (
	"errors"
	"os"
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
