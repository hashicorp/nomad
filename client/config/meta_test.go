package config

import (
	"testing"

	"github.com/hashicorp/nomad/client/structs"
	"github.com/stretchr/testify/require"
)

func TestComputeMetadataDiff(t *testing.T) {
	cases := []struct {
		Name         string
		Base         map[string]string
		Target       map[string]string
		ExpectedDiff []*structs.MetadataDiff
	}{
		{
			Name: "Basic Add",
			Base: map[string]string{},
			Target: map[string]string{
				"Foo": "bar",
			},
			ExpectedDiff: []*structs.MetadataDiff{
				{
					Type: structs.MetadataDiffTypeAdd,
					Key:  "Foo",
					To:   "bar",
				},
			},
		},
		{
			Name: "Basic Remove",
			Base: map[string]string{
				"Foo": "bar",
			},
			Target: map[string]string{},
			ExpectedDiff: []*structs.MetadataDiff{
				{
					Type: structs.MetadataDiffTypeRemove,
					Key:  "Foo",
					From: "bar",
				},
			},
		},
		{
			Name: "Basic Update",
			Base: map[string]string{
				"Foo": "bar",
			},
			Target: map[string]string{
				"Foo": "baz",
			},
			ExpectedDiff: []*structs.MetadataDiff{
				{
					Type: structs.MetadataDiffTypeUpdate,
					Key:  "Foo",
					From: "bar",
					To:   "baz",
				},
			},
		},
		{
			Name: "Basic No-op",
			Base: map[string]string{
				"Foo": "bar",
			},
			Target: map[string]string{
				"Foo": "bar",
			},
			ExpectedDiff: []*structs.MetadataDiff{},
		},
		{
			Name: "Complex",
			Base: map[string]string{
				"Foo":  "bar",
				"biz":  "baz",
				"fizz": "buzz",
			},
			Target: map[string]string{
				"Foo": "bar",
				"biz": "boz",
			},
			ExpectedDiff: []*structs.MetadataDiff{
				{
					Type: structs.MetadataDiffTypeUpdate,
					Key:  "biz",
					From: "baz",
					To:   "boz",
				},
				{
					Type: structs.MetadataDiffTypeRemove,
					Key:  "fizz",
					From: "buzz",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)

			diff := ComputeMetadataDiff(tc.Base, tc.Target)
			require.NotNil(diff)
			require.Equal(tc.ExpectedDiff, diff)
		})
	}
}

func TestApplyMetadataDiff(t *testing.T) {
	cases := []struct {
		Name   string
		Base   map[string]string
		Target map[string]string
		Diff   []*structs.MetadataDiff
	}{
		{
			Name: "Basic Add",
			Base: map[string]string{},
			Target: map[string]string{
				"Foo": "bar",
			},
			Diff: []*structs.MetadataDiff{
				{
					Type: structs.MetadataDiffTypeAdd,
					Key:  "Foo",
					To:   "bar",
				},
			},
		},
		{
			Name: "Basic Remove",
			Base: map[string]string{
				"Foo": "bar",
			},
			Target: map[string]string{},
			Diff: []*structs.MetadataDiff{
				{
					Type: structs.MetadataDiffTypeRemove,
					Key:  "Foo",
					From: "bar",
				},
			},
		},
		{
			Name: "Basic Update",
			Base: map[string]string{
				"Foo": "bar",
			},
			Target: map[string]string{
				"Foo": "baz",
			},
			Diff: []*structs.MetadataDiff{
				{
					Type: structs.MetadataDiffTypeUpdate,
					Key:  "Foo",
					From: "bar",
					To:   "baz",
				},
			},
		},
		{
			Name: "Basic No-op",
			Base: map[string]string{
				"Foo": "bar",
			},
			Target: map[string]string{
				"Foo": "bar",
			},
			Diff: []*structs.MetadataDiff{},
		},
		{
			Name: "Complex",
			Base: map[string]string{
				"Foo":  "bar",
				"biz":  "baz",
				"fizz": "buzz",
			},
			Target: map[string]string{
				"Foo": "bar",
				"biz": "boz",
			},
			Diff: []*structs.MetadataDiff{
				{
					Type: structs.MetadataDiffTypeUpdate,
					Key:  "biz",
					From: "baz",
					To:   "boz",
				},
				{
					Type: structs.MetadataDiffTypeRemove,
					Key:  "fizz",
					From: "buzz",
				},
			},
		},
		{
			Name: "Skipped Update",
			Base: map[string]string{
				"foo": "bar",
			},
			Target: map[string]string{
				"foo": "bar",
			},
			Diff: []*structs.MetadataDiff{
				{
					Type: structs.MetadataDiffTypeUpdate,
					Key:  "now-removed",
					From: "biz",
					To:   "boz",
				},
				{
					Type: structs.MetadataDiffTypeUpdate,
					Key:  "foo",
					From: "buzz",
					To:   "overruled by updated config",
				},
			},
		},
		{
			Name: "Skipped Delete",
			Base: map[string]string{
				"foo": "bar",
			},
			Target: map[string]string{
				"foo": "bar",
			},
			Diff: []*structs.MetadataDiff{
				{
					Type: structs.MetadataDiffTypeRemove,
					Key:  "foo",
					From: "overruled-by-updated-value",
				},
			},
		},
		{
			Name: "Skipped Addition",
			Base: map[string]string{
				"foo": "bar",
			},
			Target: map[string]string{
				"foo": "bar",
			},
			Diff: []*structs.MetadataDiff{
				{
					Type: structs.MetadataDiffTypeAdd,
					Key:  "foo",
					To:   "overruled-by-updated-value",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)

			result := ApplyMetadataDiff(nil, tc.Base, tc.Diff)
			require.NotNil(result)
			require.Equal(tc.Target, result)
		})
	}
}
