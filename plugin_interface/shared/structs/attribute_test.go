// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/plugin-interface/helper"
	"github.com/shoenig/test/must"
)

func TestAttribute_Validate(t *testing.T) {
	cases := []struct {
		Input *Attribute
		Fail  bool
	}{
		{
			Input: &Attribute{
				Bool: helper.PointerOf(true),
			},
		},
		{
			Input: &Attribute{
				String: helper.PointerOf("foo"),
			},
		},
		{
			Input: &Attribute{
				Int: helper.PointerOf(int64(123)),
			},
		},
		{
			Input: &Attribute{
				Float: helper.PointerOf(float64(123.2)),
			},
		},
		{
			Input: &Attribute{
				Bool: helper.PointerOf(true),
				Unit: "MB",
			},
			Fail: true,
		},
		{
			Input: &Attribute{
				String: helper.PointerOf("foo"),
				Unit:   "MB",
			},
			Fail: true,
		},
		{
			Input: &Attribute{
				Int:  helper.PointerOf(int64(123)),
				Unit: "lolNO",
			},
			Fail: true,
		},
		{
			Input: &Attribute{
				Float: helper.PointerOf(float64(123.2)),
				Unit:  "lolNO",
			},
			Fail: true,
		},
		{
			Input: &Attribute{
				Int:   helper.PointerOf(int64(123)),
				Float: helper.PointerOf(float64(123.2)),
				Unit:  "mW",
			},
			Fail: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Input.GoString(), func(t *testing.T) {
			if err := c.Input.Validate(); err != nil && !c.Fail {
				must.NoError(t, err)
			}
		})
	}
}

type compareTestCase struct {
	A             *Attribute
	B             *Attribute
	Expected      int
	NotComparable bool
}

func TestAttribute_Compare_Bool(t *testing.T) {
	cases := []*compareTestCase{
		{
			A: &Attribute{
				Bool: helper.PointerOf(true),
			},
			B: &Attribute{
				Bool: helper.PointerOf(true),
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Bool: helper.PointerOf(true),
			},
			B: &Attribute{
				Bool: helper.PointerOf(false),
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Bool: helper.PointerOf(true),
			},
			B: &Attribute{
				String: helper.PointerOf("foo"),
			},
			NotComparable: true,
		},
		{
			A: &Attribute{
				Bool: helper.PointerOf(true),
			},
			B: &Attribute{
				Int: helper.PointerOf(int64(123)),
			},
			NotComparable: true,
		},
		{
			A: &Attribute{
				Bool: helper.PointerOf(true),
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(123.2)),
			},
			NotComparable: true,
		},
	}
	testComparison(t, cases)
}

func TestAttribute_Compare_String(t *testing.T) {
	cases := []*compareTestCase{
		{
			A: &Attribute{
				String: helper.PointerOf("a"),
			},
			B: &Attribute{
				String: helper.PointerOf("b"),
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				String: helper.PointerOf("hello"),
			},
			B: &Attribute{
				String: helper.PointerOf("hello"),
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				String: helper.PointerOf("b"),
			},
			B: &Attribute{
				String: helper.PointerOf("a"),
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				String: helper.PointerOf("hello"),
			},
			B: &Attribute{
				Bool: helper.PointerOf(true),
			},
			NotComparable: true,
		},
		{
			A: &Attribute{
				String: helper.PointerOf("hello"),
			},
			B: &Attribute{
				Int: helper.PointerOf(int64(123)),
			},
			NotComparable: true,
		},
		{
			A: &Attribute{
				String: helper.PointerOf("hello"),
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(123.2)),
			},
			NotComparable: true,
		},
	}
	testComparison(t, cases)
}

func TestAttribute_Compare_Float(t *testing.T) {
	cases := []*compareTestCase{
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(101.5)),
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(100001.5)),
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(100001.5)),
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(100001.5)),
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(999999999.5)),
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(101.5)),
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(101.5)),
			},
			B: &Attribute{
				Bool: helper.PointerOf(true),
			},
			NotComparable: true,
		},
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(101.5)),
			},
			B: &Attribute{
				String: helper.PointerOf("hello"),
			},
			NotComparable: true,
		},
	}
	testComparison(t, cases)
}

func TestAttribute_Compare_Int(t *testing.T) {
	cases := []*compareTestCase{
		{
			A: &Attribute{
				Int: helper.PointerOf(int64(3)),
			},
			B: &Attribute{
				Int: helper.PointerOf(int64(10)),
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Int: helper.PointerOf(int64(10)),
			},
			B: &Attribute{
				Int: helper.PointerOf(int64(10)),
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Int: helper.PointerOf(int64(100)),
			},
			B: &Attribute{
				Int: helper.PointerOf(int64(10)),
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Int: helper.PointerOf(int64(10)),
			},
			B: &Attribute{
				Bool: helper.PointerOf(true),
			},
			NotComparable: true,
		},
		{
			A: &Attribute{
				Int: helper.PointerOf(int64(10)),
			},
			B: &Attribute{
				String: helper.PointerOf("hello"),
			},
			NotComparable: true,
		},
	}
	testComparison(t, cases)
}

func TestAttribute_Compare_Int_With_Units(t *testing.T) {
	cases := []*compareTestCase{
		{
			A: &Attribute{
				Int:  helper.PointerOf(int64(3)),
				Unit: "MB",
			},
			B: &Attribute{
				Int:  helper.PointerOf(int64(10)),
				Unit: "MB",
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Int:  helper.PointerOf(int64(10)),
				Unit: "MB",
			},
			B: &Attribute{
				Int:  helper.PointerOf(int64(10)),
				Unit: "MB",
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Int:  helper.PointerOf(int64(100)),
				Unit: "MB",
			},
			B: &Attribute{
				Int:  helper.PointerOf(int64(10)),
				Unit: "MB",
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Int:  helper.PointerOf(int64(3)),
				Unit: "GB",
			},
			B: &Attribute{
				Int:  helper.PointerOf(int64(3)),
				Unit: "MB",
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Int:  helper.PointerOf(int64(1)),
				Unit: "GiB",
			},
			B: &Attribute{
				Int:  helper.PointerOf(int64(1024)),
				Unit: "MiB",
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Int:  helper.PointerOf(int64(1)),
				Unit: "GiB",
			},
			B: &Attribute{
				Int:  helper.PointerOf(int64(1025)),
				Unit: "MiB",
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Int:  helper.PointerOf(int64(1000)),
				Unit: "mW",
			},
			B: &Attribute{
				Int:  helper.PointerOf(int64(1)),
				Unit: "W",
			},
			Expected: 0,
		},
	}
	testComparison(t, cases)
}

func TestAttribute_Compare_Float_With_Units(t *testing.T) {
	cases := []*compareTestCase{
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(3.0)),
				Unit:  "MB",
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(10.0)),
				Unit:  "MB",
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(10.0)),
				Unit:  "MB",
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(10.0)),
				Unit:  "MB",
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(100.0)),
				Unit:  "MB",
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(10.0)),
				Unit:  "MB",
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(3.0)),
				Unit:  "GB",
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(3.0)),
				Unit:  "MB",
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(1.0)),
				Unit:  "GiB",
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(1024.0)),
				Unit:  "MiB",
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(1.0)),
				Unit:  "GiB",
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(1025.0)),
				Unit:  "MiB",
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(1000.0)),
				Unit:  "mW",
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(1.0)),
				Unit:  "W",
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Float: helper.PointerOf(float64(1.5)),
				Unit:  "GiB",
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(1400.0)),
				Unit:  "MiB",
			},
			Expected: 1,
		},
	}
	testComparison(t, cases)
}

func TestAttribute_Compare_IntToFloat(t *testing.T) {
	cases := []*compareTestCase{
		{
			A: &Attribute{
				Int: helper.PointerOf(int64(3)),
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(10.0)),
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Int: helper.PointerOf(int64(10)),
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(10.0)),
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Int: helper.PointerOf(int64(10)),
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(10.1)),
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Int: helper.PointerOf(int64(100)),
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(10.0)),
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Int: helper.PointerOf(int64(100)),
			},
			B: &Attribute{
				Float: helper.PointerOf(float64(100.00001)),
			},
			Expected: -1,
		},
	}
	testComparison(t, cases)
}

func testComparison(t *testing.T, cases []*compareTestCase) {
	for _, c := range cases {
		t.Run(fmt.Sprintf("%#v vs %#v", c.A, c.B), func(t *testing.T) {
			v, ok := c.A.Compare(c.B)
			if !ok && !c.NotComparable {
				t.Fatal("should be comparable")
			} else if ok {
				must.Eq(t, c.Expected, v)
			}
		})
	}
}

func TestAttribute_ParseAndValidate(t *testing.T) {
	cases := []struct {
		Input    string
		Expected *Attribute
	}{
		{
			Input: "true",
			Expected: &Attribute{
				Bool: helper.PointerOf(true),
			},
		},
		{
			Input: "false",
			Expected: &Attribute{
				Bool: helper.PointerOf(false),
			},
		},
		{
			Input: "1",
			Expected: &Attribute{
				Int: helper.PointerOf(int64(1)),
			},
		},
		{
			Input: "100",
			Expected: &Attribute{
				Int: helper.PointerOf(int64(100)),
			},
		},
		{
			Input: "-100",
			Expected: &Attribute{
				Int: helper.PointerOf(int64(-100)),
			},
		},
		{
			Input: "-1.0",
			Expected: &Attribute{
				Float: helper.PointerOf(float64(-1.0)),
			},
		},
		{
			Input: "-100.25",
			Expected: &Attribute{
				Float: helper.PointerOf(float64(-100.25)),
			},
		},
		{
			Input: "1.01",
			Expected: &Attribute{
				Float: helper.PointerOf(float64(1.01)),
			},
		},
		{
			Input: "100.25",
			Expected: &Attribute{
				Float: helper.PointerOf(float64(100.25)),
			},
		},
		{
			Input: "foobar",
			Expected: &Attribute{
				String: helper.PointerOf("foobar"),
			},
		},
		{
			Input: "foo123bar",
			Expected: &Attribute{
				String: helper.PointerOf("foo123bar"),
			},
		},
		{
			Input: "100MB",
			Expected: &Attribute{
				Int:  helper.PointerOf(int64(100)),
				Unit: "MB",
			},
		},
		{
			Input: "-100MHz",
			Expected: &Attribute{
				Int:  helper.PointerOf(int64(-100)),
				Unit: "MHz",
			},
		},
		{
			Input: "-1.0MB/s",
			Expected: &Attribute{
				Float: helper.PointerOf(float64(-1.0)),
				Unit:  "MB/s",
			},
		},
		{
			Input: "-100.25GiB/s",
			Expected: &Attribute{
				Float: helper.PointerOf(float64(-100.25)),
				Unit:  "GiB/s",
			},
		},
		{
			Input: "1.01TB",
			Expected: &Attribute{
				Float: helper.PointerOf(float64(1.01)),
				Unit:  "TB",
			},
		},
		{
			Input: "100.25mW",
			Expected: &Attribute{
				Float: helper.PointerOf(float64(100.25)),
				Unit:  "mW",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Input, func(t *testing.T) {
			a := ParseAttribute(c.Input)
			must.Eq(t, c.Expected, a)
			must.NoError(t, a.Validate())
		})
	}
}

func BenchmarkParse(b *testing.B) {
	cases := []string{
		"true",
		"false",
		"100",
		"-100",
		"-1.0",
		"-100.25",
		"1.01",
		"100.25",
		"foobar",
		"foo123bar",
		"100MB",
		"-100MHz",
		"-1.0MB/s",
		"-100.25GiB/s",
		"1.01TB",
		"100.25mW",
	}

	for n := 0; n < b.N; n++ {
		for _, c := range cases {
			ParseAttribute(c)
		}
	}
}
