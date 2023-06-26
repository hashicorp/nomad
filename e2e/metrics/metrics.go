package metrics

import (
	"testing"

	"github.com/hashicorp/nomad/e2e/v3/cluster3"
)

func TestMetricsLinux(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
	)
}

/* TODO
func TestMetricsWindows(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
		cluster3.WindowsClients(1),
	)
}
*/
