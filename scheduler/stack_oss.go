// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package scheduler

func NewQuotaIterator(_ Context, source FeasibleIterator) FeasibleIterator {
	return source
}
