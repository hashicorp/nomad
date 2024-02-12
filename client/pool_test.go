// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package anonymous

import (
	"fmt"
	"slices"
	"testing"

	"github.com/shoenig/test/must"
)

func TestPool_Release_unused(t *testing.T) {
	p := New(200, 209)

	cases := []struct {
		id UGID
	}{
		{id: 0},
		{id: 200},
		{id: 205},
		{id: 209},
		{id: 210},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("id%s", tc.id), func(t *testing.T) {
			err := p.Release(tc.id)
			must.ErrorIs(t, ErrReleaseUnused, err)
		})
	}
}

func TestPool_Acquire_exhausted(t *testing.T) {
	p := New(200, 209)

	// consume all 10 ugids
	for i := 200; i <= 209; i++ {
		v, err := p.Acquire()
		must.NoError(t, err)
		must.Between[UGID](t, 200, v, 209)
	}

	// next acquire should fail
	v, err := p.Acquire()
	must.Eq(t, none, v)
	must.ErrorIs(t, ErrPoolExhausted, err)

	// let go of one ugid
	err2 := p.Release(204)
	must.NoError(t, err2)

	// now an acquire should succeed
	v2, err3 := p.Acquire()
	must.NoError(t, err3)
	must.Eq(t, 204, v2)
}

func TestPool_Acquire_random(t *testing.T) {
	run1 := make([]UGID, 10)
	run2 := make([]UGID, 10)

	p1 := New(100, 109)
	p2 := New(100, 109)

	// acquire all 10 UGIDs and record the order of each
	for i := 0; i < 10; i++ {
		v1, err1 := p1.Acquire()
		must.NoError(t, err1)

		v2, err2 := p2.Acquire()
		must.NoError(t, err2)

		run1[i] = v1
		run2[i] = v2
	}

	// ensure the order is different (i.e. randomness)
	must.NotEq(t, run1, run2)

	// ensure both runs contain the expected ugids
	exp := []UGID{100, 101, 102, 103, 104, 105, 106, 107, 108, 109}
	must.SliceContainsAll(t, exp, run1)
	must.SliceContainsAll(t, exp, run2)
}

func TestPool_Restore(t *testing.T) {
	p := New(500, 505) // 6 GUIDs

	// restore 501, 502, 504
	p.Restore(501)
	p.Restore(502)
	p.Restore(504)

	v1, err1 := p.Acquire()
	must.NoError(t, err1)

	v2, err2 := p.Acquire()
	must.NoError(t, err2)

	v3, err3 := p.Acquire()
	must.NoError(t, err3)

	// ensure the next 3 are the UGIDs that were not already consumed
	// and set via Restore
	ids := []UGID{v1, v2, v3}
	slices.Sort(ids)
	must.Eq(t, 500, ids[0])
	must.Eq(t, 503, ids[1])
	must.Eq(t, 505, ids[2])
}

