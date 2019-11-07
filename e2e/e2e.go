package e2e

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/e2e/framework"
)

func RunE2ETests(t *testing.T) {
	if os.Getenv("NOMAD_E2E") == "" {
		t.Skip("Skipping e2e tests, NOMAD_E2E not set")
	} else {
		framework.Run(t)
	}
}
