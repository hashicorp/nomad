// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package feasible

func NewQuotaIterator(_ Context, source FeasibleIterator) FeasibleIterator {
	return source
}
