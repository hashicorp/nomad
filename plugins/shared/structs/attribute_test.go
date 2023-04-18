package structs

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/stretchr/testify/require"
)

func TestAttribute_Validate(t *testing.T) {
	cases := []struct {
		Input *Attribute
		Fail  bool
	}{
		{
			Input: &Attribute{
				Bool: pointer.Of(true),
			},
		},
		{
			Input: &Attribute{
				String: pointer.Of("foo"),
			},
		},
		{
			Input: &Attribute{
				Int: pointer.Of(int64(123)),
			},
		},
		{
			Input: &Attribute{
				Float: pointer.Of(float64(123.2)),
			},
		},
		{
			Input: &Attribute{
				Bool: pointer.Of(true),
				Unit: "MB",
			},
			Fail: true,
		},
		{
			Input: &Attribute{
				String: pointer.Of("foo"),
				Unit:   "MB",
			},
			Fail: true,
		},
		{
			Input: &Attribute{
				Int:  pointer.Of(int64(123)),
				Unit: "lolNO",
			},
			Fail: true,
		},
		{
			Input: &Attribute{
				Float: pointer.Of(float64(123.2)),
				Unit:  "lolNO",
			},
			Fail: true,
		},
		{
			Input: &Attribute{
				Int:   pointer.Of(int64(123)),
				Float: pointer.Of(float64(123.2)),
				Unit:  "mW",
			},
			Fail: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Input.GoString(), func(t *testing.T) {
			if err := c.Input.Validate(); err != nil && !c.Fail {
				require.NoError(t, err)
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
				Bool: pointer.Of(true),
			},
			B: &Attribute{
				Bool: pointer.Of(true),
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Bool: pointer.Of(true),
			},
			B: &Attribute{
				Bool: pointer.Of(false),
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Bool: pointer.Of(true),
			},
			B: &Attribute{
				String: pointer.Of("foo"),
			},
			NotComparable: true,
		},
		{
			A: &Attribute{
				Bool: pointer.Of(true),
			},
			B: &Attribute{
				Int: pointer.Of(int64(123)),
			},
			NotComparable: true,
		},
		{
			A: &Attribute{
				Bool: pointer.Of(true),
			},
			B: &Attribute{
				Float: pointer.Of(float64(123.2)),
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
				String: pointer.Of("a"),
			},
			B: &Attribute{
				String: pointer.Of("b"),
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				String: pointer.Of("hello"),
			},
			B: &Attribute{
				String: pointer.Of("hello"),
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				String: pointer.Of("b"),
			},
			B: &Attribute{
				String: pointer.Of("a"),
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				String: pointer.Of("hello"),
			},
			B: &Attribute{
				Bool: pointer.Of(true),
			},
			NotComparable: true,
		},
		{
			A: &Attribute{
				String: pointer.Of("hello"),
			},
			B: &Attribute{
				Int: pointer.Of(int64(123)),
			},
			NotComparable: true,
		},
		{
			A: &Attribute{
				String: pointer.Of("hello"),
			},
			B: &Attribute{
				Float: pointer.Of(float64(123.2)),
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
				Float: pointer.Of(float64(101.5)),
			},
			B: &Attribute{
				Float: pointer.Of(float64(100001.5)),
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Float: pointer.Of(float64(100001.5)),
			},
			B: &Attribute{
				Float: pointer.Of(float64(100001.5)),
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Float: pointer.Of(float64(999999999.5)),
			},
			B: &Attribute{
				Float: pointer.Of(float64(101.5)),
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Float: pointer.Of(float64(101.5)),
			},
			B: &Attribute{
				Bool: pointer.Of(true),
			},
			NotComparable: true,
		},
		{
			A: &Attribute{
				Float: pointer.Of(float64(101.5)),
			},
			B: &Attribute{
				String: pointer.Of("hello"),
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
				Int: pointer.Of(int64(3)),
			},
			B: &Attribute{
				Int: pointer.Of(int64(10)),
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Int: pointer.Of(int64(10)),
			},
			B: &Attribute{
				Int: pointer.Of(int64(10)),
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Int: pointer.Of(int64(100)),
			},
			B: &Attribute{
				Int: pointer.Of(int64(10)),
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Int: pointer.Of(int64(10)),
			},
			B: &Attribute{
				Bool: pointer.Of(true),
			},
			NotComparable: true,
		},
		{
			A: &Attribute{
				Int: pointer.Of(int64(10)),
			},
			B: &Attribute{
				String: pointer.Of("hello"),
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
				Int:  pointer.Of(int64(3)),
				Unit: "MB",
			},
			B: &Attribute{
				Int:  pointer.Of(int64(10)),
				Unit: "MB",
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Int:  pointer.Of(int64(10)),
				Unit: "MB",
			},
			B: &Attribute{
				Int:  pointer.Of(int64(10)),
				Unit: "MB",
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Int:  pointer.Of(int64(100)),
				Unit: "MB",
			},
			B: &Attribute{
				Int:  pointer.Of(int64(10)),
				Unit: "MB",
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Int:  pointer.Of(int64(3)),
				Unit: "GB",
			},
			B: &Attribute{
				Int:  pointer.Of(int64(3)),
				Unit: "MB",
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Int:  pointer.Of(int64(1)),
				Unit: "GiB",
			},
			B: &Attribute{
				Int:  pointer.Of(int64(1024)),
				Unit: "MiB",
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Int:  pointer.Of(int64(1)),
				Unit: "GiB",
			},
			B: &Attribute{
				Int:  pointer.Of(int64(1025)),
				Unit: "MiB",
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Int:  pointer.Of(int64(1000)),
				Unit: "mW",
			},
			B: &Attribute{
				Int:  pointer.Of(int64(1)),
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
				Float: pointer.Of(float64(3.0)),
				Unit:  "MB",
			},
			B: &Attribute{
				Float: pointer.Of(float64(10.0)),
				Unit:  "MB",
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Float: pointer.Of(float64(10.0)),
				Unit:  "MB",
			},
			B: &Attribute{
				Float: pointer.Of(float64(10.0)),
				Unit:  "MB",
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Float: pointer.Of(float64(100.0)),
				Unit:  "MB",
			},
			B: &Attribute{
				Float: pointer.Of(float64(10.0)),
				Unit:  "MB",
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Float: pointer.Of(float64(3.0)),
				Unit:  "GB",
			},
			B: &Attribute{
				Float: pointer.Of(float64(3.0)),
				Unit:  "MB",
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Float: pointer.Of(float64(1.0)),
				Unit:  "GiB",
			},
			B: &Attribute{
				Float: pointer.Of(float64(1024.0)),
				Unit:  "MiB",
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Float: pointer.Of(float64(1.0)),
				Unit:  "GiB",
			},
			B: &Attribute{
				Float: pointer.Of(float64(1025.0)),
				Unit:  "MiB",
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Float: pointer.Of(float64(1000.0)),
				Unit:  "mW",
			},
			B: &Attribute{
				Float: pointer.Of(float64(1.0)),
				Unit:  "W",
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Float: pointer.Of(float64(1.5)),
				Unit:  "GiB",
			},
			B: &Attribute{
				Float: pointer.Of(float64(1400.0)),
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
				Int: pointer.Of(int64(3)),
			},
			B: &Attribute{
				Float: pointer.Of(float64(10.0)),
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Int: pointer.Of(int64(10)),
			},
			B: &Attribute{
				Float: pointer.Of(float64(10.0)),
			},
			Expected: 0,
		},
		{
			A: &Attribute{
				Int: pointer.Of(int64(10)),
			},
			B: &Attribute{
				Float: pointer.Of(float64(10.1)),
			},
			Expected: -1,
		},
		{
			A: &Attribute{
				Int: pointer.Of(int64(100)),
			},
			B: &Attribute{
				Float: pointer.Of(float64(10.0)),
			},
			Expected: 1,
		},
		{
			A: &Attribute{
				Int: pointer.Of(int64(100)),
			},
			B: &Attribute{
				Float: pointer.Of(float64(100.00001)),
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
				require.Equal(t, c.Expected, v)
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
				Bool: pointer.Of(true),
			},
		},
		{
			Input: "false",
			Expected: &Attribute{
				Bool: pointer.Of(false),
			},
		},
		{
			Input: "1",
			Expected: &Attribute{
				Int: pointer.Of(int64(1)),
			},
		},
		{
			Input: "100",
			Expected: &Attribute{
				Int: pointer.Of(int64(100)),
			},
		},
		{
			Input: "-100",
			Expected: &Attribute{
				Int: pointer.Of(int64(-100)),
			},
		},
		{
			Input: "-1.0",
			Expected: &Attribute{
				Float: pointer.Of(float64(-1.0)),
			},
		},
		{
			Input: "-100.25",
			Expected: &Attribute{
				Float: pointer.Of(float64(-100.25)),
			},
		},
		{
			Input: "1.01",
			Expected: &Attribute{
				Float: pointer.Of(float64(1.01)),
			},
		},
		{
			Input: "100.25",
			Expected: &Attribute{
				Float: pointer.Of(float64(100.25)),
			},
		},
		{
			Input: "foobar",
			Expected: &Attribute{
				String: pointer.Of("foobar"),
			},
		},
		{
			Input: "foo123bar",
			Expected: &Attribute{
				String: pointer.Of("foo123bar"),
			},
		},
		{
			Input: "100MB",
			Expected: &Attribute{
				Int:  pointer.Of(int64(100)),
				Unit: "MB",
			},
		},
		{
			Input: "-100MHz",
			Expected: &Attribute{
				Int:  pointer.Of(int64(-100)),
				Unit: "MHz",
			},
		},
		{
			Input: "-1.0MB/s",
			Expected: &Attribute{
				Float: pointer.Of(float64(-1.0)),
				Unit:  "MB/s",
			},
		},
		{
			Input: "-100.25GiB/s",
			Expected: &Attribute{
				Float: pointer.Of(float64(-100.25)),
				Unit:  "GiB/s",
			},
		},
		{
			Input: "1.01TB",
			Expected: &Attribute{
				Float: pointer.Of(float64(1.01)),
				Unit:  "TB",
			},
		},
		{
			Input: "100.25mW",
			Expected: &Attribute{
				Float: pointer.Of(float64(100.25)),
				Unit:  "mW",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Input, func(t *testing.T) {
			a := ParseAttribute(c.Input)
			require.Equal(t, c.Expected, a)
			require.NoError(t, a.Validate())
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
