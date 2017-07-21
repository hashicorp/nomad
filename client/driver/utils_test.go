package driver

import (
	"testing"
	"time"
)

func TestDriver_KillTimeout(t *testing.T) {
	t.Parallel()
	expected := 1 * time.Second
	max := 10 * time.Second

	if actual := GetKillTimeout(expected, max); expected != actual {
		t.Fatalf("GetKillTimeout() returned %v; want %v", actual, expected)
	}

	expected = 10 * time.Second
	input := 11 * time.Second

	if actual := GetKillTimeout(input, max); expected != actual {
		t.Fatalf("KillTimeout() returned %v; want %v", actual, expected)
	}
}
