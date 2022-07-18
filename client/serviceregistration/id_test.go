package serviceregistration

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func Test_MakeAllocServiceID(t *testing.T) {
	testCases := []struct {
		name    string
		allocID string
		label   string
		service *structs.Service
		exp     string
	}{
		{
			name:    "no label - no port - no tags",
			allocID: "7ac7c672-1824-6f06-644c-4c249e1578b9",
			service: &structs.Service{
				Name: "redis",
			},
			exp: "_nomad-task-7ac7c672-1824-6f06-644c-4c249e1578b9-group-redis",
		},
		{
			name:    "with label - no port - no tags",
			allocID: "7ac7c672-1824-6f06-644c-4c249e1578b9",
			label:   "cache",
			service: &structs.Service{
				Name: "redis",
			},
			exp: "_nomad-task-7ac7c672-1824-6f06-644c-4c249e1578b9-cache-redis",
		},
		{
			name:    "with label - with port - no tags",
			allocID: "7ac7c672-1824-6f06-644c-4c249e1578b9",
			label:   "cache",
			service: &structs.Service{
				Name:      "redis",
				PortLabel: "db",
			},
			exp: "_nomad-task-7ac7c672-1824-6f06-644c-4c249e1578b9-cache-redis-db",
		},
		{
			name:    "with label - with port - one tag",
			allocID: "7ac7c672-1824-6f06-644c-4c249e1578b9",
			label:   "cache",
			service: &structs.Service{
				Name:      "redis",
				PortLabel: "db",
				Tags:      []string{"one"},
			},
			exp: "_nomad-task-7ac7c672-1824-6f06-644c-4c249e1578b9-cache-redis-db-f97c5d",
		},
		{
			name:    "with label - with port - two tags",
			allocID: "7ac7c672-1824-6f06-644c-4c249e1578b9",
			label:   "cache",
			service: &structs.Service{
				Name:      "redis",
				PortLabel: "db",
				Tags:      []string{"one", "two"},
			},
			exp: "_nomad-task-7ac7c672-1824-6f06-644c-4c249e1578b9-cache-redis-db-5b9164",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := MakeAllocServiceID(tc.allocID, tc.label, tc.service)
			must.Eq(t, tc.exp, actualOutput)
		})
	}
}
