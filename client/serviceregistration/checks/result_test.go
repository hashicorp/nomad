// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package checks

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestChecks_GetCheckQuery(t *testing.T) {
	cases := []struct {
		name        string
		cType       string
		protocol    string
		onUpdate    string
		expMode     structs.CheckMode
		expProtocol string
	}{
		{
			name:        "http check and http set",
			cType:       "http",
			protocol:    "http",
			onUpdate:    "checks",
			expMode:     structs.Healthiness,
			expProtocol: "http",
		},
		{
			name:        "http check and https set",
			cType:       "http",
			protocol:    "https",
			onUpdate:    "checks",
			expMode:     structs.Healthiness,
			expProtocol: "https",
		},
		{
			name:        "http check and protocol unset",
			cType:       "http",
			protocol:    "",
			onUpdate:    "checks",
			expMode:     structs.Healthiness,
			expProtocol: "http", // inherit default
		},
		{
			name:        "tcp check and protocol unset",
			cType:       "tcp",
			protocol:    "",
			onUpdate:    "checks",
			expMode:     structs.Healthiness,
			expProtocol: "",
		},
		{
			name:        "http check and http set",
			cType:       "http",
			protocol:    "http",
			onUpdate:    "checks",
			expMode:     structs.Healthiness,
			expProtocol: "http",
		},
		{
			name:        "on-update ignore",
			cType:       "http",
			protocol:    "http",
			onUpdate:    structs.OnUpdateIgnore,
			expMode:     structs.Readiness,
			expProtocol: "http",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serviceCheck := &structs.ServiceCheck{
				Type:        tc.cType,
				Path:        "/",
				Protocol:    tc.protocol,
				PortLabel:   "web",
				AddressMode: "host",
				Interval:    10 * time.Second,
				Timeout:     2 * time.Second,
				Method:      "GET",
				OnUpdate:    tc.onUpdate,
			}
			query := GetCheckQuery(serviceCheck)
			must.Eq(t, tc.expMode, query.Mode)
			must.Eq(t, tc.expProtocol, query.Protocol)
		})
	}
}

func TestChecks_Stub(t *testing.T) {
	now := time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC).Unix()
	result := Stub(
		"abc123",            // check id
		structs.Healthiness, // kind
		now,                 // timestamp
		"group", "task", "service", "check",
	)
	must.Eq(t, &structs.CheckQueryResult{
		ID:        "abc123",
		Mode:      structs.Healthiness,
		Status:    structs.CheckPending,
		Output:    "nomad: waiting to run",
		Timestamp: now,
		Group:     "group",
		Task:      "task",
		Service:   "service",
		Check:     "check",
	}, result)
}

func TestChecks_ClientResults_Insert(t *testing.T) {
	cr := make(ClientResults)
	cr.Insert("alloc1", &structs.CheckQueryResult{ID: "qr1", Check: "c1"})
	cr.Insert("alloc2", &structs.CheckQueryResult{ID: "qr2", Check: "c2"})
	cr.Insert("alloc3", &structs.CheckQueryResult{ID: "qr3", Check: "c3"})
	cr.Insert("alloc2", &structs.CheckQueryResult{ID: "qr2", Check: "c4"}) // overwrite
	cr.Insert("alloc3", &structs.CheckQueryResult{ID: "qr4", Check: "c5"})
	exp := ClientResults{
		"alloc1": {
			"qr1": &structs.CheckQueryResult{ID: "qr1", Check: "c1"},
		},
		"alloc2": {
			"qr2": &structs.CheckQueryResult{ID: "qr2", Check: "c4"},
		},
		"alloc3": {
			"qr3": &structs.CheckQueryResult{ID: "qr3", Check: "c3"},
			"qr4": &structs.CheckQueryResult{ID: "qr4", Check: "c5"},
		},
	}
	must.Eq(t, exp, cr)
}
