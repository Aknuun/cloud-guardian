package main

import (
	"sync/atomic"
	"testing"
	"time"
)

// A panicking task must not stop the loop (and must not kill the process).
func TestEverySurvivesPanic(t *testing.T) {
	var runs atomic.Int32
	every("test-panic", 10*time.Millisecond, func() {
		n := runs.Add(1)
		if n == 1 {
			panic("boom")
		}
	})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runs.Load() >= 3 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("loop stopped after panic, runs=%d", runs.Load())
}
