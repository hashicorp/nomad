// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hclspecutils

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

type testConversions struct {
	Name          string
	Input         *hclspec.Spec
	Expected      hcldec.Spec
	ExpectedError string
}

func testSpecConversions(t *testing.T, cases []testConversions) {
	t.Helper()

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			act, diag := Convert(c.Input)
			if diag.HasErrors() {
				if c.ExpectedError == "" {
					t.Fatalf("Convert %q failed: %v", c.Name, diag.Error())
				}

				require.Contains(t, diag.Error(), c.ExpectedError)
			} else if c.ExpectedError != "" {
				t.Fatalf("Expected error %q", c.ExpectedError)
			}

			require.EqualValues(t, c.Expected, act)
		})
	}
}

func TestDec_Convert_Object(t *testing.T) {
	ci.Parallel(t)

	tests := []testConversions{
		{
			Name: "Object w/ only attributes",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_Object{
					Object: &hclspec.Object{
						Attributes: map[string]*hclspec.Spec{
							"foo": {
								Block: &hclspec.Spec_Attr{
									Attr: &hclspec.Attr{
										Type:     "string",
										Required: false,
									},
								},
							},
							"bar": {
								Block: &hclspec.Spec_Attr{
									Attr: &hclspec.Attr{
										Type:     "number",
										Required: true,
									},
								},
							},
							"baz": {
								Block: &hclspec.Spec_Attr{
									Attr: &hclspec.Attr{
										Type: "bool",
									},
								},
							},
						},
					},
				},
			},
			Expected: hcldec.ObjectSpec(map[string]hcldec.Spec{
				"foo": &hcldec.AttrSpec{
					Name:     "foo",
					Type:     cty.String,
					Required: false,
				},
				"bar": &hcldec.AttrSpec{
					Name:     "bar",
					Type:     cty.Number,
					Required: true,
				},
				"baz": &hcldec.AttrSpec{
					Name:     "baz",
					Type:     cty.Bool,
					Required: false,
				},
			}),
		},
	}

	testSpecConversions(t, tests)
}

func TestDec_Convert_Array(t *testing.T) {
	ci.Parallel(t)

	tests := []testConversions{
		{
			Name: "array basic",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_Array{
					Array: &hclspec.Array{
						Values: []*hclspec.Spec{
							{
								Block: &hclspec.Spec_Attr{
									Attr: &hclspec.Attr{
										Name:     "foo",
										Required: true,
										Type:     "string",
									},
								},
							},
							{
								Block: &hclspec.Spec_Attr{
									Attr: &hclspec.Attr{
										Name:     "bar",
										Required: true,
										Type:     "string",
									},
								},
							},
						},
					},
				},
			},
			Expected: hcldec.TupleSpec{
				&hcldec.AttrSpec{
					Name:     "foo",
					Type:     cty.String,
					Required: true,
				},
				&hcldec.AttrSpec{
					Name:     "bar",
					Type:     cty.String,
					Required: true,
				},
			},
		},
	}

	testSpecConversions(t, tests)
}

func TestDec_Convert_Attr(t *testing.T) {
	ci.Parallel(t)

	tests := []testConversions{
		{
			Name: "attr basic type",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_Attr{
					Attr: &hclspec.Attr{
						Name:     "foo",
						Required: true,
						Type:     "string",
					},
				},
			},
			Expected: &hcldec.AttrSpec{
				Name:     "foo",
				Type:     cty.String,
				Required: true,
			},
		},
		{
			Name: "attr object type",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_Attr{
					Attr: &hclspec.Attr{
						Name:     "foo",
						Required: true,
						Type:     "object({name1 = string, name2 = bool})",
					},
				},
			},
			Expected: &hcldec.AttrSpec{
				Name: "foo",
				Type: cty.Object(map[string]cty.Type{
					"name1": cty.String,
					"name2": cty.Bool,
				}),
				Required: true,
			},
		},
		{
			Name: "attr no name",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_Attr{
					Attr: &hclspec.Attr{
						Required: true,
						Type:     "string",
					},
				},
			},
			ExpectedError: "Missing name in attribute spec",
		},
	}

	testSpecConversions(t, tests)
}

func TestDec_Convert_Block(t *testing.T) {
	ci.Parallel(t)

	tests := []testConversions{
		{
			Name: "block with attr",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_BlockValue{
					BlockValue: &hclspec.Block{
						Name:     "test",
						Required: true,
						Nested: &hclspec.Spec{
							Block: &hclspec.Spec_Attr{
								Attr: &hclspec.Attr{
									Name: "foo",
									Type: "string",
								},
							},
						},
					},
				},
			},
			Expected: &hcldec.BlockSpec{
				TypeName: "test",
				Required: true,
				Nested: &hcldec.AttrSpec{
					Name:     "foo",
					Type:     cty.String,
					Required: false,
				},
			},
		},
		{
			Name: "block with nested block",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_BlockValue{
					BlockValue: &hclspec.Block{
						Name:     "test",
						Required: true,
						Nested: &hclspec.Spec{
							Block: &hclspec.Spec_BlockValue{
								BlockValue: &hclspec.Block{
									Name:     "test",
									Required: true,
									Nested: &hclspec.Spec{
										Block: &hclspec.Spec_Attr{
											Attr: &hclspec.Attr{
												Name: "foo",
												Type: "string",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			Expected: &hcldec.BlockSpec{
				TypeName: "test",
				Required: true,
				Nested: &hcldec.BlockSpec{
					TypeName: "test",
					Required: true,
					Nested: &hcldec.AttrSpec{
						Name:     "foo",
						Type:     cty.String,
						Required: false,
					},
				},
			},
		},
	}

	testSpecConversions(t, tests)
}

func TestDec_Convert_BlockAttrs(t *testing.T) {
	ci.Parallel(t)

	tests := []testConversions{
		{
			Name: "block attr",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_BlockAttrs{
					BlockAttrs: &hclspec.BlockAttrs{
						Name:     "test",
						Type:     "string",
						Required: true,
					},
				},
			},
			Expected: &hcldec.BlockAttrsSpec{
				TypeName:    "test",
				ElementType: cty.String,
				Required:    true,
			},
		},
		{
			Name: "block list no name",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_BlockAttrs{
					BlockAttrs: &hclspec.BlockAttrs{
						Type:     "string",
						Required: true,
					},
				},
			},
			ExpectedError: "Missing name in block_attrs spec",
		},
	}

	testSpecConversions(t, tests)
}

func TestDec_Convert_BlockList(t *testing.T) {
	ci.Parallel(t)

	tests := []testConversions{
		{
			Name: "block list with attr",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_BlockList{
					BlockList: &hclspec.BlockList{
						Name:     "test",
						MinItems: 1,
						MaxItems: 3,
						Nested: &hclspec.Spec{
							Block: &hclspec.Spec_Attr{
								Attr: &hclspec.Attr{
									Name: "foo",
									Type: "string",
								},
							},
						},
					},
				},
			},
			Expected: &hcldec.BlockListSpec{
				TypeName: "test",
				MinItems: 1,
				MaxItems: 3,
				Nested: &hcldec.AttrSpec{
					Name:     "foo",
					Type:     cty.String,
					Required: false,
				},
			},
		},
		{
			Name: "block list no name",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_BlockList{
					BlockList: &hclspec.BlockList{
						MinItems: 1,
						MaxItems: 3,
						Nested: &hclspec.Spec{
							Block: &hclspec.Spec_Attr{
								Attr: &hclspec.Attr{
									Name: "foo",
									Type: "string",
								},
							},
						},
					},
				},
			},
			ExpectedError: "Missing name in block_list spec",
		},
	}

	testSpecConversions(t, tests)
}

func TestDec_Convert_BlockSet(t *testing.T) {
	ci.Parallel(t)

	tests := []testConversions{
		{
			Name: "block set with attr",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_BlockSet{
					BlockSet: &hclspec.BlockSet{
						Name:     "test",
						MinItems: 1,
						MaxItems: 3,
						Nested: &hclspec.Spec{
							Block: &hclspec.Spec_Attr{
								Attr: &hclspec.Attr{
									Name: "foo",
									Type: "string",
								},
							},
						},
					},
				},
			},
			Expected: &hcldec.BlockSetSpec{
				TypeName: "test",
				MinItems: 1,
				MaxItems: 3,
				Nested: &hcldec.AttrSpec{
					Name:     "foo",
					Type:     cty.String,
					Required: false,
				},
			},
		},
		{
			Name: "block set missing name",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_BlockSet{
					BlockSet: &hclspec.BlockSet{
						MinItems: 1,
						MaxItems: 3,
						Nested: &hclspec.Spec{
							Block: &hclspec.Spec_Attr{
								Attr: &hclspec.Attr{
									Name: "foo",
									Type: "string",
								},
							},
						},
					},
				},
			},
			ExpectedError: "Missing name in block_set spec",
		},
	}

	testSpecConversions(t, tests)
}

func TestDec_Convert_BlockMap(t *testing.T) {
	ci.Parallel(t)

	tests := []testConversions{
		{
			Name: "block map with attr",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_BlockMap{
					BlockMap: &hclspec.BlockMap{
						Name:   "test",
						Labels: []string{"key1", "key2"},
						Nested: &hclspec.Spec{
							Block: &hclspec.Spec_Attr{
								Attr: &hclspec.Attr{
									Name: "foo",
									Type: "string",
								},
							},
						},
					},
				},
			},
			Expected: &hcldec.BlockMapSpec{
				TypeName:   "test",
				LabelNames: []string{"key1", "key2"},
				Nested: &hcldec.AttrSpec{
					Name:     "foo",
					Type:     cty.String,
					Required: false,
				},
			},
		},
		{
			Name: "block map missing name",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_BlockMap{
					BlockMap: &hclspec.BlockMap{
						Labels: []string{"key1", "key2"},
						Nested: &hclspec.Spec{
							Block: &hclspec.Spec_Attr{
								Attr: &hclspec.Attr{
									Name: "foo",
									Type: "string",
								},
							},
						},
					},
				},
			},
			ExpectedError: "Missing name in block_map spec",
		},
		{
			Name: "block map missing labels",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_BlockMap{
					BlockMap: &hclspec.BlockMap{
						Name: "foo",
						Nested: &hclspec.Spec{
							Block: &hclspec.Spec_Attr{
								Attr: &hclspec.Attr{
									Name: "foo",
									Type: "string",
								},
							},
						},
					},
				},
			},
			ExpectedError: "Invalid block label name list",
		},
	}

	testSpecConversions(t, tests)
}

func TestDec_Convert_Default(t *testing.T) {
	ci.Parallel(t)

	tests := []testConversions{
		{
			Name: "default attr",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_Default{
					Default: &hclspec.Default{
						Primary: &hclspec.Spec{
							Block: &hclspec.Spec_Attr{
								Attr: &hclspec.Attr{
									Name:     "foo",
									Type:     "string",
									Required: true,
								},
							},
						},
						Default: &hclspec.Spec{
							Block: &hclspec.Spec_Literal{
								Literal: &hclspec.Literal{
									Value: "\"hi\"",
								},
							},
						},
					},
				},
			},
			Expected: &hcldec.DefaultSpec{
				Primary: &hcldec.AttrSpec{
					Name:     "foo",
					Type:     cty.String,
					Required: true,
				},
				Default: &hcldec.LiteralSpec{
					Value: cty.StringVal("hi"),
				},
			},
		},
	}

	testSpecConversions(t, tests)
}

func TestDec_Convert_Literal(t *testing.T) {
	ci.Parallel(t)

	tests := []testConversions{
		{
			Name: "bool: true",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_Literal{
					Literal: &hclspec.Literal{
						Value: "true",
					},
				},
			},
			Expected: &hcldec.LiteralSpec{
				Value: cty.BoolVal(true),
			},
		},
		{
			Name: "bool: false",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_Literal{
					Literal: &hclspec.Literal{
						Value: "false",
					},
				},
			},
			Expected: &hcldec.LiteralSpec{
				Value: cty.BoolVal(false),
			},
		},
		{
			Name: "string",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_Literal{
					Literal: &hclspec.Literal{
						Value: "\"hi\"",
					},
				},
			},
			Expected: &hcldec.LiteralSpec{
				Value: cty.StringVal("hi"),
			},
		},
		{
			Name: "string w/ func",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_Literal{
					Literal: &hclspec.Literal{
						Value: "reverse(\"hi\")",
					},
				},
			},
			Expected: &hcldec.LiteralSpec{
				Value: cty.StringVal("ih"),
			},
		},
		{
			Name: "list string",
			Input: &hclspec.Spec{
				Block: &hclspec.Spec_Literal{
					Literal: &hclspec.Literal{
						Value: "[\"hi\", \"bye\"]",
					},
				},
			},
			Expected: &hcldec.LiteralSpec{
				Value: cty.TupleVal([]cty.Value{cty.StringVal("hi"), cty.StringVal("bye")}),
			},
		},
	}

	testSpecConversions(t, tests)
}
