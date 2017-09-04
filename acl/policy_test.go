package acl

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	type tcase struct {
		Raw    string
		ErrStr string
		Expect *Policy
	}
	tcases := []tcase{
		{
			`
			namespace "default" {
				policy = "read"
			}
			`,
			"",
			&Policy{
				Namespaces: []*NamespacePolicy{
					&NamespacePolicy{
						Name:   "default",
						Policy: PolicyRead,
						Capabilities: []string{
							NamespaceCapabilityListJobs,
							NamespaceCapabilityReadJob,
						},
					},
				},
			},
		},
		{
			`
			namespace "default" {
				policy = "read"
			}
			namespace "other" {
				policy = "write"
			}
			namespace "secret" {
				capabilities = ["deny", "read-logs"]
			}
			agent {
				policy = "read"
			}
			node {
				policy = "write"
			}
			operator {
				policy = "deny"
			}
			`,
			"",
			&Policy{
				Namespaces: []*NamespacePolicy{
					&NamespacePolicy{
						Name:   "default",
						Policy: PolicyRead,
						Capabilities: []string{
							NamespaceCapabilityListJobs,
							NamespaceCapabilityReadJob,
						},
					},
					&NamespacePolicy{
						Name:   "other",
						Policy: PolicyWrite,
						Capabilities: []string{
							NamespaceCapabilityListJobs,
							NamespaceCapabilityReadJob,
							NamespaceCapabilitySubmitJob,
							NamespaceCapabilityReadLogs,
							NamespaceCapabilityReadFS,
						},
					},
					&NamespacePolicy{
						Name: "secret",
						Capabilities: []string{
							NamespaceCapabilityDeny,
							NamespaceCapabilityReadLogs,
						},
					},
				},
				Agent: &AgentPolicy{
					Policy: PolicyRead,
				},
				Node: &NodePolicy{
					Policy: PolicyWrite,
				},
				Operator: &OperatorPolicy{
					Policy: PolicyDeny,
				},
			},
		},
		{
			`
			namespace "default" {
				policy = "foo"
			}
			`,
			"Invalid namespace policy",
			nil,
		},
		{
			`
			namespace "default" {
				capabilities = ["deny", "foo"]
			}
			`,
			"Invalid namespace capability",
			nil,
		},
		{
			`
			agent {
				policy = "foo"
			}
			`,
			"Invalid agent policy",
			nil,
		},
		{
			`
			node {
				policy = "foo"
			}
			`,
			"Invalid node policy",
			nil,
		},
		{
			`
			operator {
				policy = "foo"
			}
			`,
			"Invalid operator policy",
			nil,
		},
		{
			`
			namespace "has a space"{
				policy = "read"
			}
			`,
			"Invalid namespace name",
			nil,
		},
	}

	for idx, tc := range tcases {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			p, err := Parse(tc.Raw)
			if err != nil {
				if tc.ErrStr == "" {
					t.Fatalf("Unexpected err: %v", err)
				}
				if !strings.Contains(err.Error(), tc.ErrStr) {
					t.Fatalf("Unexpected err: %v", err)
				}
				return
			}
			if err == nil && tc.ErrStr != "" {
				t.Fatalf("Missing expected err")
			}
			tc.Expect.Raw = tc.Raw
			assert.EqualValues(t, tc.Expect, p)
		})
	}
}
