package structs

import (
	"fmt"
	"testing"
	"time"
)

func TestBatchFuture(t *testing.T) {
	t.Parallel()
	bf := NewBatchFuture()

	// Async respond to the future
	expect := fmt.Errorf("testing")
	go func() {
		time.Sleep(10 * time.Millisecond)
		bf.Respond(1000, expect)
	}()

	// Block for the result
	start := time.Now()
	err := bf.Wait()
	diff := time.Since(start)
	if diff < 5*time.Millisecond {
		t.Fatalf("too fast")
	}

	// Check the results
	if err != expect {
		t.Fatalf("bad: %s", err)
	}
	if bf.Index() != 1000 {
		t.Fatalf("bad: %d", bf.Index())
	}
}
