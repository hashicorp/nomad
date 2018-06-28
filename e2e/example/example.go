package example

import (
	"github.com/hashicorp/nomad/e2e/framework"
)

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "simple",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(SimpleExampleTestCase),
		},
	})
}

type SimpleExampleTestCase struct {
	framework.TC
}

func (tc *SimpleExampleTestCase) TestExample() {
	jobs, _, err := tc.Nomad().Jobs().List(nil)
	tc.NoError(err)
	tc.Empty(jobs)
}
