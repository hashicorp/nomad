package example

import (
	"time"

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

func (tc *SimpleExampleTestCase) TestPassExample(f *framework.F) {
	f.T().Log("all good here")
}

type ExampleLongSetupCase struct {
	framework.TC
}

func (tc *ExampleLongSetupCase) BeforeEach(f *framework.F) {
	time.Sleep(5 * time.Second)
}

func (tc *ExampleLongSetupCase) TestPass(f *framework.F) {

}
