package driver

import (
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestDriver_getTaskKillSignal(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("Linux only test")
	}

	// Test that the default is SIGINT
	{
		sig, err := getTaskKillSignal("")
		assert.Nil(err)
		assert.Equal(sig, os.Interrupt)
	}

	// Test that unsupported signals return an error
	{
		_, err := getTaskKillSignal("ABCDEF")
		assert.NotNil(err)
		assert.Contains(err.Error(), "Signal ABCDEF is not supported")
	}

	// Test that supported signals return that signal
	{
		sig, err := getTaskKillSignal("SIGKILL")
		assert.Nil(err)
		assert.Equal(sig, syscall.SIGKILL)
	}
}
