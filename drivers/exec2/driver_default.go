// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package exec2

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/drivers"
)

func New(hclog.Logger) drivers.DriverPlugin {
	panic("exec2 only supported on linux")
}
