package monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	log "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestMonitor_Start(t *testing.T) {
	t.Parallel()

	logger := log.NewInterceptLogger(&log.LoggerOptions{
		Level: log.Error,
	})

	m := New(512, logger, &log.LoggerOptions{
		Level: log.Debug,
	})

	closeCh := make(chan struct{})
	defer close(closeCh)

	logCh := m.Start(closeCh)
	go func() {
		for {
			select {
			case log := <-logCh:
				require.Contains(t, string(log), "[DEBUG] test log")
			case <-time.After(1 * time.Second):
				t.Fatal("Expected to receive from log channel")
			}
		}
	}()
	logger.Debug("test log")
}

func TestMonitor_DroppedMessages(t *testing.T) {
	t.Parallel()

	logger := log.NewInterceptLogger(&log.LoggerOptions{
		Level: log.Warn,
	})

	m := New(5, logger, &log.LoggerOptions{
		Level: log.Debug,
	})

	doneCh := make(chan struct{})
	defer close(doneCh)

	m.Start(doneCh)

	for i := 0; i <= 9; i++ {
		logger.Debug("test message")
	}

	assert.Greater(t, m.droppedCount, 0)
}
