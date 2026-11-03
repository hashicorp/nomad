// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
	"github.com/shoenig/test/must"
)

func TestDriverNeedsLabeledSockets(t *testing.T) {
	ci.Parallel(t)

	must.False(t, DriverNeedsLabeledSockets(nil))
	must.True(t, DriverNeedsLabeledSockets(&drivers.Capabilities{FSIsolation: fsisolation.Image}))
	must.False(t, DriverNeedsLabeledSockets(&drivers.Capabilities{FSIsolation: fsisolation.Chroot}))
	must.False(t, DriverNeedsLabeledSockets(&drivers.Capabilities{FSIsolation: fsisolation.None}))
}

func TestAPIHook_LabelSocket(t *testing.T) {
	ci.Parallel(t)

	capsFor := func(mode fsisolation.Mode) func() (*drivers.Capabilities, error) {
		return func() (*drivers.Capabilities, error) {
			return &drivers.Capabilities{FSIsolation: mode}, nil
		}
	}

	cases := []struct {
		name   string
		option bool
		caps   func() (*drivers.Capabilities, error)
		exp    bool
	}{
		{name: "option off", option: false, caps: capsFor(fsisolation.Image), exp: false},
		{name: "image driver", option: true, caps: capsFor(fsisolation.Image), exp: true},
		{name: "chroot driver", option: true, caps: capsFor(fsisolation.Chroot), exp: false},
		{name: "no isolation driver", option: true, caps: capsFor(fsisolation.None), exp: false},
		{name: "no capabilities", option: true, caps: nil, exp: false},
		{
			name:   "capabilities error",
			option: true,
			caps: func() (*drivers.Capabilities, error) {
				return nil, errors.New("driver not found")
			},
			exp: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newAPIHook(context.Background(), testAPIListenerRegistrar{}, testlog.HCLogger(t), tc.option, tc.caps)
			must.Eq(t, tc.exp, h.labelSocket())
		})
	}
}
