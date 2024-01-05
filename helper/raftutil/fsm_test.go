// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func Test_dummyFSM(t *testing.T) {
	ci.Parallel(t)

	dummyNomadFSM, err := dummyFSM(hclog.NewNullLogger())
	must.NotNil(t, dummyNomadFSM)
	must.NoError(t, err)
}
