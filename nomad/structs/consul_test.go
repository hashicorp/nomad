// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestConsul_Copy(t *testing.T) {
	tests := []struct {
		name string
		orig *Consul
	}{
		{"nil", nil},
		{"set namespace", &Consul{Namespace: "one"}},
		{"set namespace and identity", &Consul{
			Namespace:   "one",
			UseIdentity: true,
			ServiceIdentity: &WorkloadIdentity{
				Name:     "consul-service/test-80",
				Audience: []string{"consul.io", "nomad.dev"},
				Env:      false,
				File:     true,
			}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { must.Eq(t, tt.orig, tt.orig.Copy()) })
	}
}

func TestConsul_Equal(t *testing.T) {
	tests := []struct {
		name   string
		one    *Consul
		two    *Consul
		wantEq bool
	}{
		{"nil", nil, nil, true},
		{"nil and set", nil, &Consul{Namespace: "one"}, false},
		{"same", &Consul{
			Namespace:   "one",
			UseIdentity: true,
			ServiceIdentity: &WorkloadIdentity{
				Name:     "consul-service/test-80",
				Audience: []string{"consul.io", "nomad.dev"},
				Env:      false,
				File:     true,
			}},
			&Consul{
				Namespace:   "one",
				UseIdentity: true,
				ServiceIdentity: &WorkloadIdentity{
					Name:     "consul-service/test-80",
					Audience: []string{"consul.io", "nomad.dev"},
					Env:      false,
					File:     true,
				}},
			true,
		},
		{"different", &Consul{
			Namespace:   "one",
			UseIdentity: true,
			ServiceIdentity: &WorkloadIdentity{
				Name:     "consul-service/test-80",
				Audience: []string{"consul.io", "nomad.dev"},
				Env:      false,
				File:     true,
			}},
			&Consul{
				Namespace:   "one",
				UseIdentity: true,
				ServiceIdentity: &WorkloadIdentity{
					Name:     "consul-service/test-80",
					Audience: []string{"consul.io", "nomad.com"},
					Env:      false,
					File:     true,
				}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantEq {
				must.True(t, tt.one.Equal(tt.two))
			} else {
				must.False(t, tt.one.Equal(tt.two))
			}
		})
	}
}

func TestConsul_Validate(t *testing.T) {
	tests := []struct {
		name    string
		c       *Consul
		wantErr bool
	}{
		{"empty ns", &Consul{Namespace: ""}, false},
		{"use identity set but no identity", &Consul{UseIdentity: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				must.Error(t, tt.c.Validate())
			} else {
				must.NoError(t, tt.c.Validate())
			}
		})
	}
}
