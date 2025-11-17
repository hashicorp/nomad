// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestCustomResourcesMath(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name      string
		have      *CustomResource
		delta     *CustomResource
		expect    *CustomResource
		method    func(*CustomResource, *CustomResource) error
		expectErr string
	}{
		{
			name:      "incompatible",
			have:      &CustomResource{Name: "foo"},
			delta:     &CustomResource{Name: "bar"},
			method:    (*CustomResource).Add,
			expectErr: `custom resources could not be compared: resource names "foo" and "bar" mismatch`,
		},
		{
			name: "countable add",
			have: &CustomResource{
				Name:     "disk",
				Type:     CustomResourceTypeCountable,
				Quantity: 100_000,
			},
			delta: &CustomResource{
				Name:     "disk",
				Type:     CustomResourceTypeCountable,
				Quantity: 10_000,
			},
			method: (*CustomResource).Add,
			expect: &CustomResource{
				Name:     "disk",
				Type:     CustomResourceTypeCountable,
				Quantity: 110_000,
			},
		},
		{
			name: "countable subtract floor",
			have: &CustomResource{
				Name:     "disk",
				Type:     CustomResourceTypeCountable,
				Quantity: 10_000,
			},
			delta: &CustomResource{
				Name:     "disk",
				Type:     CustomResourceTypeCountable,
				Quantity: 20_000,
			},
			method: (*CustomResource).Subtract,
			expect: &CustomResource{
				Name:     "disk",
				Type:     CustomResourceTypeCountable,
				Quantity: 0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			got := tc.have.Copy()
			delta := tc.delta.Copy()
			err := tc.method(got, delta)

			if tc.expectErr == "" {
				must.NoError(t, err)
				must.Eq(t, tc.expect, got)
			} else {
				must.EqError(t, err, tc.expectErr)
				must.Eq(t, got, tc.have, must.Sprint("expected unchanged on error"))
			}
			must.Eq(t, tc.delta, delta, must.Sprint("expected delta to be unchanged"))
		})
	}

}

func TestCustomResourcesSuperset(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name      string
		have      *CustomResource
		want      *CustomResource
		expectOk  bool
		expectMsg string
	}{
		{
			name:      "incompatible",
			have:      &CustomResource{Name: "foo"},
			want:      &CustomResource{Name: "bar"},
			expectMsg: `custom resources could not be compared: resource names "foo" and "bar" mismatch`,
		},
		{
			name: "countable ok",
			have: &CustomResource{
				Name:     "disk",
				Type:     CustomResourceTypeCountable,
				Quantity: 100_000,
			},
			want: &CustomResource{
				Name:     "disk",
				Type:     CustomResourceTypeCountable,
				Quantity: 10_000,
			},
			expectOk: true,
		},
		{
			name: "countable exhausted",
			have: &CustomResource{
				Name:     "disk",
				Type:     CustomResourceTypeCountable,
				Quantity: 100_000,
			},
			want: &CustomResource{
				Name:     "disk",
				Type:     CustomResourceTypeCountable,
				Quantity: 200_000,
			},
			expectMsg: "custom resource: disk",
		},
		{
			name: "dynamic ok",
			have: &CustomResource{
				Name:  "ports",
				Type:  CustomResourceTypeDynamicInstance,
				Items: []any{8001, 8002, 8003},
			},
			want: &CustomResource{
				Name:  "ports",
				Type:  CustomResourceTypeDynamicInstance,
				Items: []any{8002},
			},
			expectOk: true,
		},
		{
			name: "dynamic exhausted",
			have: &CustomResource{
				Name:  "ports",
				Type:  CustomResourceTypeDynamicInstance,
				Items: []any{8001, 8002, 8003},
			},
			want: &CustomResource{
				Name:  "ports",
				Type:  CustomResourceTypeDynamicInstance,
				Items: []any{8006},
			},
			expectMsg: "custom resource: ports",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok, msg := tc.have.Superset(tc.want)
			test.Eq(t, tc.expectOk, ok)
			test.Eq(t, tc.expectMsg, msg)
		})
	}

}

func TestCustomResourcesMerge(t *testing.T) {
	ci.Parallel(t)

	base := CustomResources{
		{ // should be updated
			Name:    "foo_dynamic",
			Version: 1,
			Type:    CustomResourceTypeDynamicInstance,
			Scope:   CustomResourceScopeTask,
			Range:   "1-3",
			Items:   []any{1, 2, 3},
		},
		{ // different version, should be ignored
			Name:     "bar_countable",
			Type:     CustomResourceTypeCountable,
			Scope:    CustomResourceScopeGroup,
			Quantity: 10_000,
		},
		{ // should be ignored
			Name:     "quuz_ratio",
			Type:     CustomResourceTypeRatio,
			Scope:    CustomResourceScopeTask,
			Quantity: 100,
		},
	}

	other := CustomResources{
		{ // should update
			Name:    "foo_dynamic",
			Version: 1,
			Type:    CustomResourceTypeDynamicInstance,
			Scope:   CustomResourceScopeTask,
			Range:   "2-4,7-8",
			Items:   []any{2, 3, 4, 7, 8},
		},
		{ // different version, should be added
			Name:     "bar_countable",
			Version:  2,
			Type:     CustomResourceTypeCountable,
			Scope:    CustomResourceScopeGroup,
			Quantity: 20_000,
		},
		{ // new resource, should be added
			Name:  "baz_static",
			Type:  CustomResourceTypeStaticInstance,
			Scope: CustomResourceScopeGroup,
			Items: []any{10, 20, 30},
		},
	}

	got := base.Copy()
	got.Merge(other)
	must.Eq(t,
		CustomResources{
			{
				Name:    "foo_dynamic",
				Version: 1,
				Type:    CustomResourceTypeDynamicInstance,
				Scope:   CustomResourceScopeTask,
				Range:   "2-4,7-8",
				Items:   []any{2, 3, 4, 7, 8},
			},
			{
				Name:     "bar_countable",
				Type:     CustomResourceTypeCountable,
				Scope:    CustomResourceScopeGroup,
				Quantity: 10_000,
			},
			{
				Name:     "quuz_ratio",
				Type:     CustomResourceTypeRatio,
				Scope:    CustomResourceScopeTask,
				Quantity: 100,
			},
			{
				Name:     "bar_countable",
				Version:  2,
				Type:     CustomResourceTypeCountable,
				Scope:    CustomResourceScopeGroup,
				Quantity: 20_000,
			},
			{
				Name:  "baz_static",
				Type:  CustomResourceTypeStaticInstance,
				Scope: CustomResourceScopeGroup,
				Items: []any{10, 20, 30},
			},
		},
		got)

}

func TestCustomResources_Select(t *testing.T) {

	ci.Parallel(t)

	available := CustomResources{
		{
			Name:     "foo_countable",
			Type:     CustomResourceTypeCountable,
			Scope:    CustomResourceScopeGroup,
			Quantity: 10_000,
		},
		{
			Name:    "bar_dynamic",
			Version: 1,
			Type:    CustomResourceTypeDynamicInstance,
			Scope:   CustomResourceScopeTask,
			Range:   "1-10",
			Items:   []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
	}

	request := CustomResources{
		{
			Name:     "foo_countable",
			Type:     CustomResourceTypeCountable,
			Scope:    CustomResourceScopeGroup,
			Quantity: 0,
		},
		{
			Name:     "bar_dynamic",
			Version:  1,
			Type:     CustomResourceTypeDynamicInstance,
			Scope:    CustomResourceScopeTask,
			Quantity: 0,
		},
	}

	request[0].Quantity = 10
	request[1].Quantity = 20
	err := request.Select(available)
	must.EqError(t, err, "custom resources exhausted: bar_dynamic")

	request[0].Quantity = 10
	request[1].Quantity = 4
	err = request.Select(available)
	must.NoError(t, err)
	must.Len(t, 4, request[1].Items)

	available[1].Items = []any{}
	for i := range 100 {
		available[1].Items = append(available[1].Items, i)
	}
	err = request.Select(available)
	must.NoError(t, err)
	must.Len(t, 4, request[1].Items)
}
