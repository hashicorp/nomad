package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestFormatRoundedFloat(t *testing.T) {
	testutil.Parallel(t)

	cases := []struct {
		input    float64
		expected string
	}{
		{
			1323,
			"1323",
		},
		{
			10.321,
			"10.321",
		},
		{
			100000.31324324,
			"100000.313",
		},
		{
			100000.3,
			"100000.3",
		},
		{
			0.7654321,
			"0.765",
		},
	}

	for _, c := range cases {
		must.Eq(t, c.expected, formatFloat(c.input, 3))
	}
}

func Test_PointerOf(t *testing.T) {
	s := "hello"
	sPtr := pointerOf(s)

	must.Eq(t, s, *sPtr)

	b := "bye"
	sPtr = &b
	must.NotEq(t, s, *sPtr)
}
