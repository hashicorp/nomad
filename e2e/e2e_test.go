package e2e

import (
	"testing"

	_ "github.com/hashicorp/nomad/e2e/example"
)

func TestE2E(t *testing.T) {
	RunE2ETests(t)
}
