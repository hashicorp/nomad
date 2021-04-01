package license

import (
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/stretchr/testify/require"
)

type LicenseE2ETest struct {
	framework.TC
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "License",
		CanRunLocal: true,
		Cases:       []framework.TestCase{new(LicenseE2ETest)},
	})
}

func (tc *LicenseE2ETest) TestLicenseGet(f *framework.F) {
	t := f.T()

	client := tc.Nomad()

	// Get the license and do not forward to the leader
	lic, _, err := client.Operator().LicenseGet(&api.QueryOptions{
		AllowStale: true,
	})

	require.NoError(t, err)
	require.NotEqual(t, "temporary-license", lic.License.LicenseID)
}
