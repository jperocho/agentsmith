package lock

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireUnlock(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".lock")
	l, err := Acquire(p)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := l.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	// Re-acquiring after release must succeed.
	l2, err := Acquire(p)
	if err != nil {
		t.Fatalf("re-Acquire: %v", err)
	}
	if err := l2.Unlock(); err != nil {
		t.Fatalf("second Unlock: %v", err)
	}
}

func TestUnlockNilSafe(t *testing.T) {
	var l *Lock
	if err := l.Unlock(); err != nil {
		t.Errorf("nil-receiver Unlock = %v, want nil", err)
	}
	if err := (&Lock{}).Unlock(); err != nil {
		t.Errorf("zero-value Unlock = %v, want nil", err)
	}
}

func TestExclusiveBlocksUntilRelease(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".lock")
	l1, err := Acquire(p)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	got := make(chan *Lock, 1)
	go func() {
		// This Acquire blocks (LOCK_EX) until l1 is released.
		l2, err := Acquire(p)
		if err != nil {
			t.Errorf("contending Acquire: %v", err)
			return
		}
		got <- l2
	}()

	// Held: the contender must not acquire.
	select {
	case <-got:
		t.Fatal("second Acquire succeeded while lock held")
	case <-time.After(150 * time.Millisecond):
	}

	if err := l1.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	// Released: the contender must now acquire promptly.
	select {
	case l2 := <-got:
		if err := l2.Unlock(); err != nil {
			t.Errorf("contender Unlock: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second Acquire did not proceed after release")
	}
}
