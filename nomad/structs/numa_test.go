// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/shoenig/test/must"
)

func TestNUMA_Equal(t *testing.T) {
	ci.Parallel(t)

	must.Equal[*NUMA](t, nil, nil)
	must.NotEqual[*NUMA](t, nil, new(NUMA))

	must.StructEqual(t, &NUMA{
		Affinity: "none",
	}, []must.Tweak[*NUMA]{{
		Field: "Affinity",
		Apply: func(n *NUMA) { n.Affinity = "require" },
	}})
}

func TestNUMA_Validate(t *testing.T) {
	ci.Parallel(t)

	err := errors.New("numa affinity must be one of none, prefer, or require")

	cases := []struct {
		name     string
		affinity string
		exp      error
	}{
		{
			name:     "affinity unset",
			affinity: "",
			exp:      err,
		},
		{
			name:     "affinity none",
			affinity: "none",
			exp:      nil,
		},
		{
			name:     "affinity prefer",
			affinity: "prefer",
			exp:      nil,
		},
		{
			name:     "affinity require",
			affinity: "require",
			exp:      nil,
		},
		{
			name:     "affinity invalid",
			affinity: "invalid",
			exp:      err,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			numa := &NUMA{
				tc.affinity,
			}
			result := numa.Validate()
			must.Eq(t, tc.exp, result)
		})
	}
}

func TestNUMA_Copy(t *testing.T) {
	ci.Parallel(t)

	n := &NUMA{Affinity: "require"}
	c := n.Copy()
	must.Equal(t, n, c)

	n.Affinity = "prefer"
	must.NotEqual(t, n, c)
}

func makeLegacyCore(id hw.CoreID) numalib.Core {
	return numalib.Core{
		SocketID:   0,
		NodeID:     0,
		ID:         id,
		Grade:      numalib.Performance,
		Disable:    false,
		GuessSpeed: 3200,
	}
}
