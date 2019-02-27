package docker

import (
	"testing"

	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/drivers"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// TestDockerDriver_FingerprintHealth asserts that docker reports healthy
// whenever Docker is supported.
//
// In Linux CI and AppVeyor Windows environment, it should be enabled.
func TestDockerDriver_FingerprintHealth(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	d := NewDockerDriver(testlog.HCLogger(t)).(*Driver)

	fp := d.buildFingerprint()
	require.Equal(t, drivers.HealthStateHealthy, fp.Health)
}
