// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	cstructs "github.com/hashicorp/nomad/client/structs"
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

func TestMonitor_External(t *testing.T) {

	ci.Parallel(t)
	const (
		expectedText   = "log log log log log"
		goldenFilePath = "../testdata/monitor-external.golden"
	)
	goldenFileContents, err := os.ReadFile(goldenFilePath)
	must.NoError(t, err)

	testFile, err := os.CreateTemp("", "nomadtests-tshot-")
	must.NoError(t, err)

	_, err = testFile.Write([]byte(expectedText))
	must.NoError(t, err)
	inlineFilePath := testFile.Name()

	logger := log.NewInterceptLogger(&log.LoggerOptions{
		Level: log.Error,
	})

	monitor := New(512, logger, &log.LoggerOptions{})
	cases := []struct {
		name     string
		opts     cstructs.MonitorExternalRequest
		expected string
	}{
		{
			name: "happy_path_logpath_golden",
			opts: cstructs.MonitorExternalRequest{
				LogSince:     "72",
				OnDisk:       true,
				NomadLogPath: goldenFilePath,
			},
			expected: string(goldenFileContents),
		},
		{
			name: "happy_path_logpath_inline",
			opts: cstructs.MonitorExternalRequest{
				LogSince:     "72",
				OnDisk:       true,
				NomadLogPath: inlineFilePath,
			},
			expected: expectedText,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			opts := cstructs.MonitorExternalRequest{
				LogSince:     tc.opts.LogSince,
				ServiceName:  tc.opts.ServiceName,
				Follow:       tc.opts.Follow,
				NomadLogPath: tc.opts.NomadLogPath,
			}
			logCh := monitor.MonitorExternal(&opts)

			dir := t.TempDir()

			filename := filepath.Join(dir, "just-some-file")
			f, err := os.Create(filename)
			must.NoError(t, err)
			defer f.Close()
			var builder strings.Builder
			go func() {
			TEST:
				for {
					select {
					case log, ok := <-logCh:
						if !ok {
							break TEST
						}
						builder.Grow(len(log))
						builder.Write(log)

					default:
						continue
					}

				}
				received := builder.String()
				must.Eq(t, tc.expected, received)
			}()
		})
	}
}
