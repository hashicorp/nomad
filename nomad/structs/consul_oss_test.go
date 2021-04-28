// +build !ent

package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJob_ConfigEntries(t *testing.T) {
	t.Parallel()

	ingress := &ConsulConnect{
		Gateway: &ConsulGateway{
			Ingress: new(ConsulIngressConfigEntry),
		},
	}

	terminating := &ConsulConnect{
		Gateway: &ConsulGateway{
			Terminating: new(ConsulTerminatingConfigEntry),
		},
	}

	j := &Job{
		TaskGroups: []*TaskGroup{{
			Name:   "group1",
			Consul: nil,
			Services: []*Service{{
				Name:    "group1-service1",
				Connect: ingress,
			}, {
				Name:    "group1-service2",
				Connect: nil,
			}, {
				Name:    "group1-service3",
				Connect: terminating,
			}},
		}, {
			Name:   "group2",
			Consul: nil,
			Services: []*Service{{
				Name:    "group2-service1",
				Connect: ingress,
			}},
		}, {
			Name:   "group3",
			Consul: &Consul{Namespace: "apple"},
			Services: []*Service{{
				Name:    "group3-service1",
				Connect: ingress,
			}},
		}, {
			Name:   "group4",
			Consul: &Consul{Namespace: "apple"},
			Services: []*Service{{
				Name:    "group4-service1",
				Connect: ingress,
			}, {
				Name:    "group4-service2",
				Connect: terminating,
			}},
		}, {
			Name:   "group5",
			Consul: &Consul{Namespace: "banana"},
			Services: []*Service{{
				Name:    "group5-service1",
				Connect: ingress,
			}},
		}},
	}

	exp := map[string]*ConsulConfigEntries{
		// in OSS, consul namespace is not supported
		"": &ConsulConfigEntries{
			Ingress: map[string]*ConsulIngressConfigEntry{
				"group1-service1": new(ConsulIngressConfigEntry),
				"group2-service1": new(ConsulIngressConfigEntry),
				"group3-service1": new(ConsulIngressConfigEntry),
				"group4-service1": new(ConsulIngressConfigEntry),
				"group5-service1": new(ConsulIngressConfigEntry),
			},
			Terminating: map[string]*ConsulTerminatingConfigEntry{
				"group1-service3": new(ConsulTerminatingConfigEntry),
				"group4-service2": new(ConsulTerminatingConfigEntry),
			},
		},
	}

	entries := j.ConfigEntries()
	require.EqualValues(t, exp, entries)
}
