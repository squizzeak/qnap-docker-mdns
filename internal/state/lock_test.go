package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockAcquireRelease(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.lock")

	lock := NewLock(path)
	acquired, err := lock.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if !acquired {
		t.Fatal("expected to acquire lock")
	}

	owner, err := lock.ReadOwner()
	if err != nil {
		t.Fatal(err)
	}
	if owner <= 0 {
		t.Errorf("expected positive PID, got %d", owner)
	}

	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestLockDuplicateFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.lock")

	lock1 := NewLock(path)
	acquired, err := lock1.Acquire()
	if err != nil || !acquired {
		t.Fatal("first lock should succeed")
	}
	defer lock1.Release()

	lock2 := NewLock(path)
	acquired, err = lock2.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if acquired {
		t.Fatal("second lock should fail")
	}
}

func TestLockReleaseAllowsReacquire(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.lock")

	lock1 := NewLock(path)
	lock1.Acquire()
	lock1.Release()

	lock2 := NewLock(path)
	acquired, err := lock2.Acquire()
	if err != nil || !acquired {
		t.Fatal("should reacquire after release")
	}
	lock2.Release()
}

func TestReadOwnerStaleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.lock")

	if err := os.WriteFile(path, []byte("12345\n"), 0644); err != nil {
		t.Fatal(err)
	}

	lock := NewLock(path)
	pid, err := lock.ReadOwner()
	if err != nil {
		t.Fatal(err)
	}
	if pid != 12345 {
		t.Errorf("expected 12345, got %d", pid)
	}
}
