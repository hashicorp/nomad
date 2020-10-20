package taskenv

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// TestInterpolateServices asserts that all service
// and check fields are properly interpolated.
func TestInterpolateServices(t *testing.T) {
	t.Parallel()
	services := []*structs.Service{
		{
			Name:      "${name}",
			PortLabel: "${portlabel}",
			Tags:      []string{"${tags}"},
			Meta: map[string]string{
				"meta-key": "${meta}",
			},
			CanaryMeta: map[string]string{
				"canarymeta-key": "${canarymeta}",
			},
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

	env := &TaskEnv{
		EnvMap: map[string]string{
			"name":         "name",
			"portlabel":    "portlabel",
			"tags":         "tags",
			"meta":         "meta-value",
			"canarymeta":   "canarymeta-value",
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

	interpolated := InterpolateServices(env, services)

	exp := []*structs.Service{
		{
			Name:      "name",
			PortLabel: "portlabel",
			Tags:      []string{"tags"},
			Meta: map[string]string{
				"meta-key": "meta-value",
			},
			CanaryMeta: map[string]string{
				"canarymeta-key": "canarymeta-value",
			},
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
