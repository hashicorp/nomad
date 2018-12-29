package hclspec

import (
	"testing"

	"github.com/hashicorp/hcl2/hcldec"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

type testConversions struct {
	Name          string
	Input         *Spec
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
	t.Parallel()

	tests := []testConversions{
		{
			Name: "Object w/ only attributes",
			Input: &Spec{
				Block: &Spec_Object{
					&Object{
						Attributes: map[string]*Spec{
							"foo": {
								Block: &Spec_Attr{
									&Attr{
										Type:     "string",
										Required: false,
									},
								},
							},
							"bar": {
								Block: &Spec_Attr{
									&Attr{
										Type:     "number",
										Required: true,
									},
								},
							},
							"baz": {
								Block: &Spec_Attr{
									&Attr{
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
	t.Parallel()

	tests := []testConversions{
		{
			Name: "array basic",
			Input: &Spec{
				Block: &Spec_Array{
					Array: &Array{
						Values: []*Spec{
							{
								Block: &Spec_Attr{
									&Attr{
										Name:     "foo",
										Required: true,
										Type:     "string",
									},
								},
							},
							{
								Block: &Spec_Attr{
									&Attr{
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
	t.Parallel()

	tests := []testConversions{
		{
			Name: "attr basic type",
			Input: &Spec{
				Block: &Spec_Attr{
					&Attr{
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
			Input: &Spec{
				Block: &Spec_Attr{
					&Attr{
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
			Input: &Spec{
				Block: &Spec_Attr{
					&Attr{
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
	t.Parallel()

	tests := []testConversions{
		{
			Name: "block with attr",
			Input: &Spec{
				Block: &Spec_BlockValue{
					BlockValue: &Block{
						Name:     "test",
						Required: true,
						Nested: &Spec{
							Block: &Spec_Attr{
								&Attr{
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
			Input: &Spec{
				Block: &Spec_BlockValue{
					BlockValue: &Block{
						Name:     "test",
						Required: true,
						Nested: &Spec{
							Block: &Spec_BlockValue{
								BlockValue: &Block{
									Name:     "test",
									Required: true,
									Nested: &Spec{
										Block: &Spec_Attr{
											&Attr{
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

func TestDec_Convert_BlockList(t *testing.T) {
	t.Parallel()

	tests := []testConversions{
		{
			Name: "block list with attr",
			Input: &Spec{
				Block: &Spec_BlockList{
					BlockList: &BlockList{
						Name:     "test",
						MinItems: 1,
						MaxItems: 3,
						Nested: &Spec{
							Block: &Spec_Attr{
								&Attr{
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
			Input: &Spec{
				Block: &Spec_BlockList{
					BlockList: &BlockList{
						MinItems: 1,
						MaxItems: 3,
						Nested: &Spec{
							Block: &Spec_Attr{
								&Attr{
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
	t.Parallel()

	tests := []testConversions{
		{
			Name: "block set with attr",
			Input: &Spec{
				Block: &Spec_BlockSet{
					BlockSet: &BlockSet{
						Name:     "test",
						MinItems: 1,
						MaxItems: 3,
						Nested: &Spec{
							Block: &Spec_Attr{
								&Attr{
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
			Input: &Spec{
				Block: &Spec_BlockSet{
					BlockSet: &BlockSet{
						MinItems: 1,
						MaxItems: 3,
						Nested: &Spec{
							Block: &Spec_Attr{
								&Attr{
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
	t.Parallel()

	tests := []testConversions{
		{
			Name: "block map with attr",
			Input: &Spec{
				Block: &Spec_BlockMap{
					BlockMap: &BlockMap{
						Name:   "test",
						Labels: []string{"key1", "key2"},
						Nested: &Spec{
							Block: &Spec_Attr{
								&Attr{
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
			Input: &Spec{
				Block: &Spec_BlockMap{
					BlockMap: &BlockMap{
						Labels: []string{"key1", "key2"},
						Nested: &Spec{
							Block: &Spec_Attr{
								&Attr{
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
			Input: &Spec{
				Block: &Spec_BlockMap{
					BlockMap: &BlockMap{
						Name: "foo",
						Nested: &Spec{
							Block: &Spec_Attr{
								&Attr{
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
	t.Parallel()

	tests := []testConversions{
		{
			Name: "default attr",
			Input: &Spec{
				Block: &Spec_Default{
					Default: &Default{
						Primary: &Spec{
							Block: &Spec_Attr{
								&Attr{
									Name:     "foo",
									Type:     "string",
									Required: true,
								},
							},
						},
						Default: &Spec{
							Block: &Spec_Literal{
								&Literal{
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
	t.Parallel()

	tests := []testConversions{
		{
			Name: "bool: true",
			Input: &Spec{
				Block: &Spec_Literal{
					Literal: &Literal{
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
			Input: &Spec{
				Block: &Spec_Literal{
					Literal: &Literal{
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
			Input: &Spec{
				Block: &Spec_Literal{
					Literal: &Literal{
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
			Input: &Spec{
				Block: &Spec_Literal{
					Literal: &Literal{
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
			Input: &Spec{
				Block: &Spec_Literal{
					Literal: &Literal{
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
