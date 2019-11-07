package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatRoundedFloat(t *testing.T) {
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
		require.Equal(t, c.expected, formatFloat(c.input, 3))
	}
}
