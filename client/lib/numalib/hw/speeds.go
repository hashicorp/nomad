// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hw

import (
	"strconv"
)

type (
	MHz uint64
	KHz uint64
)

func (khz KHz) MHz() MHz {
	return MHz(khz / 1000)
}

func (khz KHz) String() string {
	return strconv.FormatUint(uint64(khz.MHz()), 10)
}
