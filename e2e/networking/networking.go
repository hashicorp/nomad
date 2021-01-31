package networking

import (
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
)

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Networking",
		CanRunLocal: true,
		Cases:       []framework.TestCase{e2eutil.NewE2EJob("networking/inputs/basic.nomad")},
	})
}
