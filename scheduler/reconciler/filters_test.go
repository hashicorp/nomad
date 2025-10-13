// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestReconciler_filterServerTerminalAllocs(t *testing.T) {
	makeSet := func() allocSet {
		set := make(allocSet)
		for _ = range 5 {
			alloc := mock.Alloc()
			set[alloc.ID] = alloc
		}

		return set
	}

	t.Run("none", func(t *testing.T) {
		set := makeSet()
		filtered := set.filterServerTerminalAllocs()
		must.Eq(t, filtered, set)
	})

	t.Run("with stop", func(t *testing.T) {
		set := makeSet()
		alloc := mock.Alloc()
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		set[alloc.ID] = alloc

		filtered := set.filterServerTerminalAllocs()
		must.NotEq(t, filtered, set)
		must.MapLen(t, 5, filtered)
	})

	t.Run("with evict", func(t *testing.T) {
		set := makeSet()
		alloc := mock.Alloc()
		alloc.DesiredStatus = structs.AllocDesiredStatusEvict
		set[alloc.ID] = alloc

		filtered := set.filterServerTerminalAllocs()
		must.NotEq(t, filtered, set)
		must.MapLen(t, 5, filtered)
	})

	t.Run("with stop batch", func(t *testing.T) {
		set := makeSet()
		alloc := mock.BatchAlloc()
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		set[alloc.ID] = alloc

		filtered := set.filterServerTerminalAllocs()
		must.Eq(t, filtered, set)
		must.MapLen(t, 6, filtered)
	})

	t.Run("with evict batch", func(t *testing.T) {
		set := makeSet()
		alloc := mock.BatchAlloc()
		alloc.DesiredStatus = structs.AllocDesiredStatusEvict
		set[alloc.ID] = alloc

		filtered := set.filterServerTerminalAllocs()
		must.Eq(t, filtered, set)
		must.MapLen(t, 6, filtered)
	})
}
