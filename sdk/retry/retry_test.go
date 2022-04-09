package retry

import (
	"fmt"
	"regexp"
	"runtime"
	"testing"
	"time"
)

// getDelta defines the time band a test run should complete in.
//
// macOS is *really* behind the curve in github actions
func getDelta() time.Duration {
	switch runtime.GOOS {
	case "darwin":
		return 250 * time.Millisecond
	default:
		return 25 * time.Millisecond
	}
}

func TestRetryer(t *testing.T) {
	delta := getDelta()

	tests := []struct {
		desc string
		r    Retryer
	}{
		{"counter", &Counter{Count: 3, Wait: 100 * time.Millisecond}},
		{"timer", &Timer{Timeout: 200 * time.Millisecond, Wait: 100 * time.Millisecond}},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var count int
			start := time.Now()
			for tt.r.Continue() {
				count++
			}
			dur := time.Since(start)
			if got, want := count, 3; got != want {
				t.Fatalf("got %d retries want %d", got, want)
			}
			// since the first iteration happens immediately
			// the Retryer waits only twice for three iterations.
			// order of events: (true, (wait) true, (wait) true, false)
			if got, want := dur, 200*time.Millisecond; got < (want-delta) || got > (want+delta) {
				t.Fatalf("loop took %v want %v (+/- %v)", got, want, delta)
			}
		})
	}
}

var (
	notYetRe       = regexp.MustCompile(`not yet\n`)
	doNotProceedRe = regexp.MustCompile(`do not proceed\n`)
)

func TestRunWith(t *testing.T) {
	t.Run("calls FailNow after exceeding retries", func(t *testing.T) {
		ft := &fakeT{}
		count := 0
		RunWith(&Counter{Count: 3, Wait: time.Millisecond}, ft, func(r *R) {
			count++
			r.FailNow()
		})

		if count != 3 {
			t.Fatalf("expected count to be 3, got: %d", count)
		}
		if ft.fails != 1 {
			t.Fatalf("expected fails to be 1, got: %d", ft.fails)
		}
	})

	t.Run("Stop ends the retrying", func(t *testing.T) {
		ft := &fakeT{}
		count := 0
		RunWith(&Counter{Count: 5, Wait: time.Millisecond}, ft, func(r *R) {
			count++
			if count == 2 {
				r.Stop(fmt.Errorf("do not proceed"))
			}
			r.Fatalf("not yet")
		})

		if count != 2 {
			t.Fatalf("expected count to be 2, got: %d", count)
		}
		if ft.fails != 1 {
			t.Fatalf("expected fails to be 1, got: %d", ft.fails)
		}
		if len(ft.out) != 1 {
			t.Fatalf("expected length to be 1, got: %d", len(ft.out))
		}
		if !notYetRe.MatchString(ft.out[0]) {
			t.Fatalf("expected output to contain 'not yet', got: %q", ft.out[0])
		}
		if !doNotProceedRe.MatchString(ft.out[0]) {
			t.Fatalf("expected output to contain 'do not proceed', got: %q", ft.out[0])
		}
	})
}

type fakeT struct {
	fails int
	out   []string
}

func (f *fakeT) Helper() {}

func (f *fakeT) Log(args ...interface{}) {
	f.out = append(f.out, fmt.Sprint(args...))
}

func (f *fakeT) FailNow() {
	f.fails++
}

var _ Failer = &fakeT{}
