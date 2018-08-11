package base

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Very simple test to ensure the test harness works as expected
func TestDriverHarness(t *testing.T) {
	handle := &TaskHandle{Config: &TaskConfig{Name: "mock"}}
	d := &MockDriver{
		StartTaskF: func(task *TaskConfig) (*TaskHandle, error) {
			return handle, nil
		},
	}
	harness := NewDriverHarness(t, d)
	defer harness.Kill()
	actual, err := harness.StartTask(&TaskConfig{})
	require.NoError(t, err)
	require.Equal(t, handle.Config.Name, actual.Config.Name)
}
