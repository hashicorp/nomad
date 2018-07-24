package example

import (
	"testing"

	"github.com/hashicorp/nomad/e2e/framework"
)

func TestE2E(t *testing.T) {
	framework.New().AddSuites(&framework.TestSuite{
		Component:   "simple",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(SimpleExampleTestCase),
		},
	}).Run(t)
}
