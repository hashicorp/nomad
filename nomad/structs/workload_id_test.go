// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestWorkloadIdentity_Equal(t *testing.T) {
	ci.Parallel(t)

	var orig *WorkloadIdentity

	newWI := orig.Copy()
	must.Equal(t, orig, newWI)

	orig = &WorkloadIdentity{}
	must.NotEqual(t, orig, newWI)

	newWI = &WorkloadIdentity{}
	must.Equal(t, orig, newWI)

	orig.Env = true
	must.NotEqual(t, orig, newWI)

	newWI.Env = true
	must.Equal(t, orig, newWI)

	newWI.File = true
	must.NotEqual(t, orig, newWI)

	newWI.File = false
	must.Equal(t, orig, newWI)

	newWI.Name = "foo"
	must.NotEqual(t, orig, newWI)

	newWI.Name = ""
	must.Equal(t, orig, newWI)

	newWI.Audience = []string{"foo"}
	must.NotEqual(t, orig, newWI)
}

// TestWorkloadIdentity_Validate asserts that canonicalized workload identities
// validate and emit warnings as expected.
func TestWorkloadIdentity_Validate(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Desc string
		In   WorkloadIdentity
		Exp  WorkloadIdentity
		Err  string
		Warn string
	}{
		{
			Desc: "Empty",
			In:   WorkloadIdentity{},
			Exp: WorkloadIdentity{
				Name:     WorkloadIdentityDefaultName,
				Audience: []string{WorkloadIdentityDefaultAud},
			},
		},
		{
			Desc: "Default audience",
			In: WorkloadIdentity{
				Name: WorkloadIdentityDefaultName,
			},
			Exp: WorkloadIdentity{
				Name:     WorkloadIdentityDefaultName,
				Audience: []string{WorkloadIdentityDefaultAud},
			},
		},
		{
			Desc: "Ok",
			In: WorkloadIdentity{
				Name:     "foo-id",
				Audience: []string{"http://nomadproject.io/"},
				Env:      true,
				File:     true,
			},
			Exp: WorkloadIdentity{
				Name:     "foo-id",
				Audience: []string{"http://nomadproject.io/"},
				Env:      true,
				File:     true,
			},
		},
		{
			Desc: "Be reasonable",
			In: WorkloadIdentity{
				Name: strings.Repeat("x", 1025),
			},
			Err: "invalid name",
		},
		{
			Desc: "No hacks",
			In: WorkloadIdentity{
				Name: "../etc/passwd",
			},
			Err: "invalid name",
		},
		{
			Desc: "No Windows hacks",
			In: WorkloadIdentity{
				Name: `A:\hacks`,
			},
			Err: "invalid name",
		},
		{
			Desc: "Empty audience",
			In: WorkloadIdentity{
				Name:     "foo",
				Audience: []string{"ok", ""},
			},
			Err: "an empty string is an invalid audience (2)",
		},
		{
			Desc: "Warn audience",
			In: WorkloadIdentity{
				Name: "foo",
			},
			Exp: WorkloadIdentity{
				Name: "foo",
			},
			Warn: "identities without an audience are insecure",
		},
		{
			Desc: "Warn too many audiences",
			In: WorkloadIdentity{
				Name:     "foo",
				Audience: []string{"foo", "bar"},
			},
			Exp: WorkloadIdentity{
				Name:     "foo",
				Audience: []string{"foo", "bar"},
			},
			Warn: "while multiple audiences is allowed, it is more secure to use 1 audience per identity",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Desc, func(t *testing.T) {
			tc.In.Canonicalize()

			if err := tc.In.Validate(); err != nil {
				if tc.Err == "" {
					t.Fatalf("unexpected validation error: %s", err)
				}
				must.ErrorContains(t, err, tc.Err)
				return
			}

			// Only compare valid structs
			must.Eq(t, tc.Exp, tc.In)

			if err := tc.In.Warnings(); err != nil {
				if tc.Warn == "" {
					t.Fatalf("unexpected warnings: %s", err)
				}
				must.ErrorContains(t, err, tc.Warn)
				return
			}
		})
	}
}

func TestWorkloadIdentity_Nil(t *testing.T) {
	ci.Parallel(t)

	var nilWID *WorkloadIdentity

	nilWID = nilWID.Copy()
	must.Nil(t, nilWID)

	must.True(t, nilWID.Equal(nil))

	nilWID.Canonicalize()

	must.Error(t, nilWID.Validate())

	must.Error(t, nilWID.Warnings())
}
