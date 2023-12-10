// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package scheduler

func NewQuotaIterator(_ Context, source FeasibleIterator) FeasibleIterator {
	return source
}
