package helper

import (
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func TestCluster_RandomStagger(t *testing.T) {
	cases := []struct {
		name  string
		input time.Duration
	}{
		{name: "positive", input: 1 * time.Second},
		{name: "negative", input: -1 * time.Second},
		{name: "zero", input: 0},
	}

	abs := func(d time.Duration) time.Duration {
		return Max(d, -d)
	}

	for _, tc := range cases {
		result := RandomStagger(tc.input)
		must.GreaterEq(t, result, 0)
		must.LessEq(t, result, abs(tc.input))
	}
}
