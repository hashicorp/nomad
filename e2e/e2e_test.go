// This package exists to wrap our e2e provisioning and test framework so that it
// can be run via 'go test ./e2e'. See './framework/framework.go'
package e2e

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/e2e/framework"

	_ "github.com/hashicorp/nomad/e2e/affinities"
	_ "github.com/hashicorp/nomad/e2e/clientstate"
	_ "github.com/hashicorp/nomad/e2e/connect"
	_ "github.com/hashicorp/nomad/e2e/consul"
	_ "github.com/hashicorp/nomad/e2e/consultemplate"
	_ "github.com/hashicorp/nomad/e2e/csi"
	_ "github.com/hashicorp/nomad/e2e/deployment"
	_ "github.com/hashicorp/nomad/e2e/example"
	_ "github.com/hashicorp/nomad/e2e/hostvolumes"
	_ "github.com/hashicorp/nomad/e2e/metrics"
	_ "github.com/hashicorp/nomad/e2e/nomad09upgrade"
	_ "github.com/hashicorp/nomad/e2e/nomadexec"
	_ "github.com/hashicorp/nomad/e2e/spread"
	_ "github.com/hashicorp/nomad/e2e/systemsched"
	_ "github.com/hashicorp/nomad/e2e/taskevents"
)

func TestE2E(t *testing.T) {
	if os.Getenv("NOMAD_E2E") == "" {
		t.Skip("Skipping e2e tests, NOMAD_E2E not set")
	} else {
		framework.Run(t)
	}
}
