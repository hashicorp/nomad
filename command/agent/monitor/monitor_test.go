// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package monitor

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestMonitor_Start(t *testing.T) {
	ci.Parallel(t)

	logger := log.NewInterceptLogger(&log.LoggerOptions{
		Level: log.Error,
	})

	m := New(512, logger, &log.LoggerOptions{
		Level: log.Debug,
	})

	logCh := m.Start()
	defer m.Stop()

	go func() {
		logger.Debug("test log")
		time.Sleep(10 * time.Millisecond)
	}()

	for {
		select {
		case log := <-logCh:
			require.Contains(t, string(log), "[DEBUG] test log")
			return
		case <-time.After(3 * time.Second):
			t.Fatal("Expected to receive from log channel")
		}
	}
}

// Ensure number of dropped messages are logged
func TestMonitor_DroppedMessages(t *testing.T) {
	ci.Parallel(t)

	logger := log.NewInterceptLogger(&log.LoggerOptions{
		Level: log.Warn,
	})

	m := new(5, logger, &log.LoggerOptions{
		Level: log.Debug,
	})
	m.droppedDuration = 5 * time.Millisecond

	doneCh := make(chan struct{})
	defer close(doneCh)

	logCh := m.Start()

	for i := 0; i <= 100; i++ {
		logger.Debug(fmt.Sprintf("test message %d", i))
	}

	received := ""
	passed := make(chan struct{})
	go func() {
		for {
			select {
			case recv := <-logCh:
				received += string(recv)
				if strings.Contains(received, "[WARN] Monitor dropped") {
					close(passed)
				}
			}
		}
	}()

TEST:
	for {
		select {
		case <-passed:
			break TEST
		case <-time.After(2 * time.Second):
			require.Fail(t, "expected to see warn dropped messages")
		}
	}
}

func TestMonitor_Export(t *testing.T) {
	ci.Parallel(t)
	const (
		expectedText = "log log log log log"
	)

	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "log")
	must.NoError(t, err)
	for range 1000 {
		_, _ = f.WriteString(fmt.Sprintf("%v [INFO] it's log, it's log, it's big it's heavy it's wood", time.Now()))
	}
	f.Close()
	goldenFilePath := f.Name()
	goldenFileContents, err := os.ReadFile(goldenFilePath)
	must.NoError(t, err)

	testFile, err := os.CreateTemp("", "nomadtest")
	must.NoError(t, err)

	_, err = testFile.Write([]byte(expectedText))
	must.NoError(t, err)
	inlineFilePath := testFile.Name()

	logger := log.NewInterceptLogger(&log.LoggerOptions{
		Level: log.Error,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cases := []struct {
		name        string
		opts        MonitorExportOpts
		expected    string
		expectClose bool
	}{
		{
			name: "happy_path_logpath_long_file",
			opts: MonitorExportOpts{
				Context:      ctx,
				Logger:       logger,
				OnDisk:       true,
				NomadLogPath: goldenFilePath,
			},
			expected: string(goldenFileContents),
		},
		{
			name: "happy_path_logpath_short_file",
			opts: MonitorExportOpts{
				Context:      ctx,
				Logger:       logger,
				OnDisk:       true,
				NomadLogPath: inlineFilePath,
			},
			expected: expectedText,
		},
		{
			name: "close context",
			opts: MonitorExportOpts{
				Context:      ctx,
				Logger:       logger,
				OnDisk:       true,
				NomadLogPath: inlineFilePath,
			},
			expected: expectedText,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			monitor, err := NewExportMonitor(tc.opts)
			must.NoError(t, err)
			logCh := monitor.Start()
			if tc.expectClose {
				cancel()
			}
			var (
				builder strings.Builder
				wg      sync.WaitGroup
			)
			wg.Add(1)
			go func() {
				defer wg.Done()
			TEST:
				for {
					select {
					case log, ok := <-logCh:
						if !ok {
							break TEST
						}
						builder.Write(log)
						time.Sleep(15 * time.Millisecond)
					default:
						continue
					}

				}
			}()
			wg.Wait()
			if !tc.expectClose {
				must.Eq(t, strings.TrimSpace(tc.expected), strings.TrimSpace(builder.String()))
			} else {
				must.Eq(t, builder.String(), "")
			}

		})
	}
}
