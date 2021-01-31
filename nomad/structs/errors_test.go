package structs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRPCCodedErrors(t *testing.T) {
	cases := []struct {
		err     error
		code    int
		message string
	}{
		{
			NewErrRPCCoded(400, "a test message,here"),
			400,
			"a test message,here",
		},
		{
			NewErrRPCCodedf(500, "a test message,here %s %s", "and,here%s", "second"),
			500,
			"a test message,here and,here%s second",
		},
	}

	for _, c := range cases {
		t.Run(c.err.Error(), func(t *testing.T) {
			code, msg, ok := CodeFromRPCCodedErr(c.err)
			assert.True(t, ok)
			assert.Equal(t, c.code, code)
			assert.Equal(t, c.message, msg)
		})
	}

	negativeCases := []string{
		"random error",
		errRPCCodedErrorPrefix,
		errRPCCodedErrorPrefix + "123",
		errRPCCodedErrorPrefix + "qwer,asdf",
	}
	for _, c := range negativeCases {
		t.Run(c, func(t *testing.T) {
			_, _, ok := CodeFromRPCCodedErr(errors.New(c))
			assert.False(t, ok)
		})
	}
}
