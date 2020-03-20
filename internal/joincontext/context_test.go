package joincontext

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestCancelFirst(t *testing.T) {
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2 := context.Background()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatal("context cancelled")
	default:
	}
	cancel1()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Fatalf("unexpected error %v, expected %v", ctx.Err(), context.Canceled)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("context not cancelled")
	}
}

func TestCancelSecond(t *testing.T) {
	ctx1 := context.Background()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatal("context cancelled")
	default:
	}
	cancel2()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Fatalf("unexpected error %v, expected %v", ctx.Err(), context.Canceled)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("context not cancelled")
	}
}

func TestDeadlineEmpty(t *testing.T) {
	ctx1 := context.Background()
	ctx2 := context.Background()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()

	_, ok := ctx.Deadline()
	if ok {
		t.Fatal("context has deadline")
	}
}

func TestDeadlineSecondEmpty(t *testing.T) {
	ctx1, cancel1 := context.WithDeadline(context.Background(), time.Date(2086, time.January, 0, 0, 0, 0, 0, time.UTC))
	defer cancel1()
	ctx2 := context.Background()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()

	d, ok := ctx.Deadline()
	if !ok {
		t.Fatal("context has no deadline")
	}

	d1, _ := ctx1.Deadline()
	if d != d1 {
		t.Fatalf("unexpected deadline %v, expected %v", d, d1)
	}
}

func TestDeadlineFirstEmpty(t *testing.T) {
	ctx1 := context.Background()
	ctx2, cancel2 := context.WithDeadline(context.Background(), time.Date(2086, time.January, 0, 0, 0, 0, 0, time.UTC))
	defer cancel2()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()

	d, ok := ctx.Deadline()
	if !ok {
		t.Fatal("context has no deadline")
	}

	d2, _ := ctx2.Deadline()
	if d != d2 {
		t.Fatalf("unexpected deadline %v, expected %v", d, d2)
	}
}

func TestDeadlineFirstEarly(t *testing.T) {
	ctx1, cancel1 := context.WithDeadline(context.Background(), time.Date(2085, time.January, 0, 0, 0, 0, 0, time.UTC))
	defer cancel1()
	ctx2, cancel2 := context.WithDeadline(context.Background(), time.Date(2086, time.January, 0, 0, 0, 0, 0, time.UTC))
	defer cancel2()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()

	d, ok := ctx.Deadline()
	if !ok {
		t.Fatal("context has no deadline")
	}

	d1, _ := ctx1.Deadline()
	if d != d1 {
		t.Fatalf("unexpected deadline %v, expected %v", d, d1)
	}
}

func TestDeadlineSecondEarly(t *testing.T) {
	ctx1, cancel1 := context.WithDeadline(context.Background(), time.Date(2086, time.January, 0, 0, 0, 0, 0, time.UTC))
	defer cancel1()
	ctx2, cancel2 := context.WithDeadline(context.Background(), time.Date(2085, time.January, 0, 0, 0, 0, 0, time.UTC))
	defer cancel2()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()

	d, ok := ctx.Deadline()
	if !ok {
		t.Fatal("context has no deadline")
	}

	d2, _ := ctx2.Deadline()
	if d != d2 {
		t.Fatalf("unexpected deadline %v, expected %v", d, d2)
	}
}

func TestTimeoutFirst(t *testing.T) {
	ctx1, cancel1 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel1()
	ctx2 := context.Background()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Fatalf("unexpected error %v, expected %v", ctx.Err(), context.DeadlineExceeded)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("context not cancelled")
	}
}

func TestTimeoutSecond(t *testing.T) {
	ctx1 := context.Background()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Fatalf("unexpected error %v, expected %v", ctx.Err(), context.DeadlineExceeded)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("context not cancelled")
	}
}

func TestCancel(t *testing.T) {
	ctx1 := context.Background()
	ctx2 := context.Background()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Fatal("context cancelled")
	default:
	}
	cancel()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Fatalf("unexpected error %v, expected %v", ctx.Err(), context.Canceled)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("context not cancelled")
	}
}

func TestCancelAsParent(t *testing.T) {
	ctx1 := context.Background()
	ctx2 := context.Background()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()

	child, cancelChild := context.WithCancel(ctx)
	defer cancelChild()

	cancel()

	select {
	case <-child.Done():
		if child.Err() != context.Canceled {
			t.Fatalf("unexpected error in child context %v, expected %v", child.Err(), context.Canceled)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("child context not cancelled")
	}
}

func TestConcurrency(t *testing.T) {
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()

	child, childCancel := context.WithCancel(ctx)
	defer childCancel()

	spawn := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < 64; i++ {
		wg.Add(5)
		go func() {
			defer wg.Done()
			<-spawn
			<-ctx.Done()
		}()
		go func() {
			defer wg.Done()
			<-spawn
			<-child.Done()
		}()
		go func() {
			defer wg.Done()
			<-spawn
			cancel()
		}()
		go func() {
			defer wg.Done()
			<-spawn
			cancel1()
		}()
		go func() {
			defer wg.Done()
			<-spawn
			cancel2()
		}()
	}
	close(spawn)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("concurrency test failed with timeout")
	}
}

func TestGoroutineLeak(t *testing.T) {
	origNum := runtime.NumGoroutine()
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2 := context.Background()

	ctx, cancel := Join(ctx1, ctx2)
	defer cancel()

	cancel1()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Fatalf("unexpected error %v, expected %v", ctx.Err(), context.Canceled)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("context not cancelled")
	}
	time.Sleep(500 * time.Millisecond)

	// run GC, so it won't spawn accident goroutines
	runtime.GC()

	newNum := runtime.NumGoroutine()

	if newNum > origNum {
		t.Fatalf("there was goroutine leak")
	}
}
