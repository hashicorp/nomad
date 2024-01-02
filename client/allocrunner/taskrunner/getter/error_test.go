// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package getter

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestError_Error(t *testing.T) {
	cases := []struct {
		name string
		err  *Error
		exp  string
	}{
		{"object nil", nil, "<nil>"},
		{"error nil", new(Error), "<nil>"},
		{"has error", &Error{Err: errors.New("oops")}, "oops"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := Error{Err: tc.err}
			result := e.Error()
			must.Eq(t, tc.exp, result)
		})
	}
}

func TestError_IsRecoverable(t *testing.T) {
	var _ structs.Recoverable = (*Error)(nil)
	must.True(t, (&Error{Recoverable: true}).IsRecoverable())
	must.False(t, (&Error{Recoverable: false}).IsRecoverable())
}

func TestError_Equal(t *testing.T) {
	cases := []struct {
		name string
		a    *Error
		b    *Error
		exp  bool
	}{
		{name: "both nil", a: nil, b: nil, exp: true},
		{name: "one nil", a: new(Error), b: nil, exp: false},
		{
			name: "different url",
			a:    &Error{URL: "example.com/a"},
			b:    &Error{URL: "example.com/b"},
			exp:  false,
		},
		{
			name: "different err",
			a:    &Error{URL: "example.com/z", Err: errors.New("b")},
			b:    &Error{URL: "example.com/z", Err: errors.New("a")},
			exp:  false,
		},
		{
			name: "nil vs not nil err",
			a:    &Error{URL: "example.com/z", Err: errors.New("b")},
			b:    &Error{URL: "example.com/z", Err: nil},
			exp:  false,
		},
		{
			name: "different recoverable",
			a:    &Error{URL: "example.com", Err: errors.New("a"), Recoverable: false},
			b:    &Error{URL: "example.com", Err: errors.New("b"), Recoverable: true},
			exp:  false,
		},
		{
			name: "same no error",
			a:    &Error{URL: "example.com", Err: nil, Recoverable: true},
			b:    &Error{URL: "example.com", Err: nil, Recoverable: true},
			exp:  true,
		},
		{
			name: "same with error",
			a:    &Error{URL: "example.com", Err: errors.New("a"), Recoverable: true},
			b:    &Error{URL: "example.com", Err: errors.New("a"), Recoverable: true},
			exp:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.a.Equal(tc.b)
			must.Eq(t, tc.exp, result)
		})
	}
}
