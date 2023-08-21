// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cgroupslib

import (
	"github.com/hashicorp/nomad/client/lib/idset"
)

type Partition interface {
	Reserve(*idset.Set[idset.CoreID]) error
	Release(*idset.Set[idset.CoreID]) error
}
