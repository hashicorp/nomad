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
			new(ExampleLongSetupCase),
		},
	})
}

type SimpleExampleTestCase struct {
	framework.TC
}

func (tc *SimpleExampleTestCase) TestExample(f *framework.F) {
	f.T().Log("Logging foo")
	jobs, _, err := tc.Nomad().Jobs().List(nil)
	f.NoError(err)
	f.Empty(jobs)
}

func (tc *SimpleExampleTestCase) TestParallelExample(f *framework.F) {
	f.T().Log("this one can run in parallel with other tests")
	f.T().Parallel()
}

type ExampleLongSetupCase struct {
	framework.TC
}

func (tc *ExampleLongSetupCase) BeforeEach(f *framework.F) {
	f.T().Log("Logging before each")
}
