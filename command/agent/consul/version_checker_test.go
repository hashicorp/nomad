// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/nomad/ci"
)

func TestConsulSupportsTLSSkipVerify(t *testing.T) {
	ci.Parallel(t)

	assertSupport := func(expected bool, blob string) {
		self := map[string]map[string]interface{}{}
		if err := json.Unmarshal([]byte("{"+blob+"}"), &self); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		actual := supportsTLSSkipVerify(self)
		if actual != expected {
			t.Errorf("expected %t but got %t for:\n%s\n", expected, actual, blob)
		}
	}

	// 0.6.4
	assertSupport(false, `"Member": {
        "Addr": "127.0.0.1",
        "DelegateCur": 4,
        "DelegateMax": 4,
        "DelegateMin": 2,
        "Name": "rusty",
        "Port": 8301,
        "ProtocolCur": 2,
        "ProtocolMax": 3,
        "ProtocolMin": 1,
        "Status": 1,
        "Tags": {
            "build": "0.6.4:26a0ef8c",
            "dc": "dc1",
            "port": "8300",
            "role": "consul",
            "vsn": "2",
            "vsn_max": "3",
            "vsn_min": "1"
        }}`)

	// 0.7.0
	assertSupport(false, `"Member": {
        "Addr": "127.0.0.1",
        "DelegateCur": 4,
        "DelegateMax": 4,
        "DelegateMin": 2,
        "Name": "rusty",
        "Port": 8301,
        "ProtocolCur": 2,
        "ProtocolMax": 4,
        "ProtocolMin": 1,
        "Status": 1,
        "Tags": {
            "build": "0.7.0:'a189091",
            "dc": "dc1",
            "port": "8300",
            "role": "consul",
            "vsn": "2",
            "vsn_max": "3",
            "vsn_min": "2"
        }}`)

	// 0.7.2
	assertSupport(true, `"Member": {
        "Addr": "127.0.0.1",
        "DelegateCur": 4,
        "DelegateMax": 4,
        "DelegateMin": 2,
        "Name": "rusty",
        "Port": 8301,
        "ProtocolCur": 2,
        "ProtocolMax": 5,
        "ProtocolMin": 1,
        "Status": 1,
        "Tags": {
            "build": "0.7.2:'a9afa0c",
            "dc": "dc1",
            "port": "8300",
            "role": "consul",
            "vsn": "2",
            "vsn_max": "3",
            "vsn_min": "2"
        }}`)

	// 0.8.1
	assertSupport(true, `"Member": {
        "Addr": "127.0.0.1",
        "DelegateCur": 4,
        "DelegateMax": 5,
        "DelegateMin": 2,
        "Name": "rusty",
        "Port": 8301,
        "ProtocolCur": 2,
        "ProtocolMax": 5,
        "ProtocolMin": 1,
        "Status": 1,
        "Tags": {
            "build": "0.8.1:'e9ca44d",
            "dc": "dc1",
            "id": "3ddc1b59-460e-a100-1d5c-ce3972122664",
            "port": "8300",
            "raft_vsn": "2",
            "role": "consul",
            "vsn": "2",
            "vsn_max": "3",
            "vsn_min": "2",
            "wan_join_port": "8302"
        }}`)
}
