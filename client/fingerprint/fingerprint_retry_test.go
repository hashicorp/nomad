// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// mockFingerprinter is a mock implementation of the Fingerprint interface that
// allows controlling behavior for testing the RetryWrapper.
type mockFingerprinter struct {
	callCount     atomic.Int32
	errorSequence []error
	StaticFingerprinter
}

func (m *mockFingerprinter) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	callNum := int(m.callCount.Add(1)) - 1

	if callNum < len(m.errorSequence) {
		if err := m.errorSequence[callNum]; err != nil {
			return err
		}
	}

	return nil
}

func (m *mockFingerprinter) getCallCount() int {
	return int(m.callCount.Load())
}

func newMockFingerprinter(errorSequence []error) *mockFingerprinter {
	return &mockFingerprinter{
		errorSequence: errorSequence,
	}
}

func TestRetryWrapper_Fingerprint(t *testing.T) {
	ci.Parallel(t)

	genericError := errors.New("test error")
	probeError := wrapProbeError(genericError)

	testCases := []struct {
		name              string
		errorSequence     []error
		fpConfig          *config.Fingerprint
		expectedErr       error
		expectedCallCount int
	}{
		{
			name:              "success on first attempt",
			errorSequence:     []error{nil},
			fpConfig:          nil,
			expectedErr:       nil,
			expectedCallCount: 1,
		},
		{
			name:              "no config probe error",
			errorSequence:     []error{probeError},
			fpConfig:          nil,
			expectedErr:       nil,
			expectedCallCount: 1,
		},
		{
			name:          "exit on failure probe error",
			errorSequence: []error{probeError},
			fpConfig: &config.Fingerprint{
				Name:          "test",
				ExitOnFailure: pointer.Of(true),
			},
			expectedErr:       probeError,
			expectedCallCount: 1,
		},
		{
			name:          "no exit on failure probe error",
			errorSequence: []error{probeError},
			fpConfig: &config.Fingerprint{
				Name:          "test",
				ExitOnFailure: pointer.Of(false),
			},
			expectedErr:       nil,
			expectedCallCount: 1,
		},
		{
			name: "no config with error",
			errorSequence: []error{
				genericError,
			},
			fpConfig:          nil,
			expectedErr:       genericError,
			expectedCallCount: 1,
		},
		{
			name: "retry attempts 0 with error",
			errorSequence: []error{
				genericError,
			},
			fpConfig: &config.Fingerprint{
				Name:          "test",
				RetryAttempts: 0,
			},
			expectedErr:       genericError,
			expectedCallCount: 1,
		},
		{
			name: "retry attempts 1 fails twice",
			errorSequence: []error{
				genericError,
				genericError,
			},
			fpConfig: &config.Fingerprint{
				Name:          "test",
				RetryAttempts: 1,
				RetryInterval: 10 * time.Millisecond,
			},
			expectedErr:       genericError,
			expectedCallCount: 2,
		},
		{
			name: "retry attempts 1 succeeds on second try",
			errorSequence: []error{
				genericError,
				nil,
			},
			fpConfig: &config.Fingerprint{
				Name:          "test",
				RetryAttempts: 1,
				RetryInterval: 10 * time.Millisecond,
			},
			expectedErr:       nil,
			expectedCallCount: 2,
		},
		{
			name: "retry attempts 3 succeeds on third try",
			errorSequence: []error{
				genericError,
				genericError,
				nil,
			},
			fpConfig: &config.Fingerprint{
				Name:          "test",
				RetryAttempts: 3,
				RetryInterval: 10 * time.Millisecond,
			},
			expectedErr:       nil,
			expectedCallCount: 3,
		},
		{
			name: "retry attempts 2 fails all attempts",
			errorSequence: []error{
				genericError,
				genericError,
				genericError,
			},
			fpConfig: &config.Fingerprint{
				Name:          "test",
				RetryAttempts: 2,
				RetryInterval: 10 * time.Millisecond,
			},
			expectedErr:       genericError,
			expectedCallCount: 3,
		},
		{
			name: "retry attempts -1 retries indefinitely until success",
			errorSequence: []error{
				genericError,
				genericError,
				genericError,
				genericError,
				genericError,
				genericError,
				genericError,
				nil,
			},
			fpConfig: &config.Fingerprint{
				Name:          "test",
				RetryAttempts: -1,
				RetryInterval: 10 * time.Millisecond,
			},
			expectedErr:       nil,
			expectedCallCount: 8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			mock := newMockFingerprinter(tc.errorSequence)

			wrapper := NewRetryWrapper(mock, testlog.HCLogger(t), "test")

			cfg := &config.Config{
				Fingerprinters: map[string]*config.Fingerprint{
					"test": tc.fpConfig,
				},
			}

			req := &FingerprintRequest{Config: cfg, Node: &structs.Node{}}
			var resp FingerprintResponse

			startTime := time.Now()
			err := wrapper.Fingerprint(req, &resp)
			elapsed := time.Since(startTime)

			// Ensure the correct error response has been obtained. This is the
			// final result of the finerprinter run.
			if tc.expectedErr != nil {
				must.Error(t, err)
				must.Eq(t, tc.expectedErr, err)
			} else {
				must.NoError(t, err)
			}

			// Check the retry logic was triggered the expected number of times.
			must.Eq(t, tc.expectedCallCount, mock.getCallCount())

			// Attempt to verify that the fingerprinter took at least the
			// expected amount of time, accounting for retries. This gives us
			// further confidence that the retry logic was executed correctly.
			if tc.fpConfig != nil && tc.expectedCallCount > 1 {
				expectedMinTime := tc.fpConfig.RetryInterval
				if expectedMinTime == 0 {
					expectedMinTime = 2 * time.Second
				}
				minExpectedDuration := time.Duration(tc.expectedCallCount-1) * expectedMinTime
				must.GreaterEq(t, minExpectedDuration-100*time.Millisecond, elapsed)
			}
		})
	}
}

func Test_wrapProbeError(t *testing.T) {
	ci.Parallel(t)

	baseErr := errors.New("base error")
	wrappedErr := wrapProbeError(baseErr)

	must.Error(t, wrappedErr)
	must.ErrorIs(t, wrappedErr, errEnvProbeQueryFailed)
	must.ErrorIs(t, wrappedErr, baseErr)
	must.StrContains(t, wrappedErr.Error(), "fingerprint initial probe failed")
	must.StrContains(t, wrappedErr.Error(), "base error")
}

func Test_shouldSkipEnvFingerprinter(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name           string
		inputCfg       *config.Fingerprint
		inputError     error
		expectedOutput bool
	}{
		{
			name:           "nil config and nil error",
			inputCfg:       nil,
			inputError:     nil,
			expectedOutput: false,
		},
		{
			name:           "nil config and initial error",
			inputCfg:       nil,
			inputError:     wrapProbeError(errors.New("initial error")),
			expectedOutput: true,
		},
		{
			name:           "nil config and non-initial error",
			inputCfg:       nil,
			inputError:     errors.New("initial error"),
			expectedOutput: false,
		},
		{
			name:           "exit on failure not set and initial error",
			inputCfg:       &config.Fingerprint{},
			inputError:     wrapProbeError(errors.New("initial error")),
			expectedOutput: true,
		},
		{
			name: "exit on failure false and initial error",
			inputCfg: &config.Fingerprint{
				ExitOnFailure: pointer.Of(false),
			},
			inputError:     wrapProbeError(errors.New("initial error")),
			expectedOutput: true,
		},
		{
			name: "exit on failure true and initial error",
			inputCfg: &config.Fingerprint{
				ExitOnFailure: pointer.Of(true),
			},
			inputError:     wrapProbeError(errors.New("initial error")),
			expectedOutput: false,
		},
		{
			name: "exit on failure false and non-initial error",
			inputCfg: &config.Fingerprint{
				ExitOnFailure: pointer.Of(false),
			},
			inputError:     errors.New("initial error"),
			expectedOutput: true,
		},
		{
			name: "exit on failure true and non-initial error",
			inputCfg: &config.Fingerprint{
				ExitOnFailure: pointer.Of(true),
			},
			inputError:     errors.New("initial error"),
			expectedOutput: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expectedOutput, shouldSkipEnvFingerprinter(tc.inputCfg, tc.inputError))
		})
	}
}
