package drivers

import (
	"testing"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/stretchr/testify/require"
)

var _ DriverPlugin = (*MockDriver)(nil)

// Very simple test to ensure the test harness works as expected
func TestDriverHarness(t *testing.T) {
	handle := &TaskHandle{Config: &TaskConfig{Name: "mock"}}
	d := &MockDriver{
		StartTaskF: func(task *TaskConfig) (*TaskHandle, *cstructs.DriverNetwork, error) {
			return handle, nil, nil
		},
	}
	harness := NewDriverHarness(t, d)
	defer harness.Kill()
	actual, _, err := harness.StartTask(&TaskConfig{})
	require.NoError(t, err)
	require.Equal(t, handle.Config.Name, actual.Config.Name)
}
