package taskrunner

import (
	"testing"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// Statically assert the stats hook implements the expected interfaces
var _ interfaces.TaskPoststartHook = (*serviceHook)(nil)
var _ interfaces.TaskExitedHook = (*serviceHook)(nil)
var _ interfaces.TaskPreKillHook = (*serviceHook)(nil)
var _ interfaces.TaskUpdateHook = (*serviceHook)(nil)

// TestTaskRunner_ServiceHook_InterpolateServices asserts that all service
// and check fields are properly interpolated.
func TestTaskRunner_ServiceHook_InterpolateServices(t *testing.T) {
	t.Parallel()
	services := []*structs.Service{
		{
			Name:      "${name}",
			PortLabel: "${portlabel}",
			Tags:      []string{"${tags}"},
			Checks: []*structs.ServiceCheck{
				{
					Name:          "${checkname}",
					Type:          "${checktype}",
					Command:       "${checkcmd}",
					Args:          []string{"${checkarg}"},
					Path:          "${checkstr}",
					Protocol:      "${checkproto}",
					PortLabel:     "${checklabel}",
					InitialStatus: "${checkstatus}",
					Method:        "${checkmethod}",
					Header: map[string][]string{
						"${checkheaderk}": {"${checkheaderv}"},
					},
				},
			},
		},
	}

	env := &taskenv.TaskEnv{
		EnvMap: map[string]string{
			"name":         "name",
			"portlabel":    "portlabel",
			"tags":         "tags",
			"checkname":    "checkname",
			"checktype":    "checktype",
			"checkcmd":     "checkcmd",
			"checkarg":     "checkarg",
			"checkstr":     "checkstr",
			"checkpath":    "checkpath",
			"checkproto":   "checkproto",
			"checklabel":   "checklabel",
			"checkstatus":  "checkstatus",
			"checkmethod":  "checkmethod",
			"checkheaderk": "checkheaderk",
			"checkheaderv": "checkheaderv",
		},
	}

	interpolated := interpolateServices(env, services)

	exp := []*structs.Service{
		{
			Name:      "name",
			PortLabel: "portlabel",
			Tags:      []string{"tags"},
			Checks: []*structs.ServiceCheck{
				{
					Name:          "checkname",
					Type:          "checktype",
					Command:       "checkcmd",
					Args:          []string{"checkarg"},
					Path:          "checkstr",
					Protocol:      "checkproto",
					PortLabel:     "checklabel",
					InitialStatus: "checkstatus",
					Method:        "checkmethod",
					Header: map[string][]string{
						"checkheaderk": {"checkheaderv"},
					},
				},
			},
		},
	}

	require.Equal(t, exp, interpolated)
}
