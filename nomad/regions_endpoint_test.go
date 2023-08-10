// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestRegionList(t *testing.T) {
	ci.Parallel(t)

	// Make the servers
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Region = "region1"
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.Region = "region2"
	})
	defer cleanupS2()

	// Join the servers
	s2Addr := fmt.Sprintf("127.0.0.1:%d",
		s2.config.SerfConfig.MemberlistConfig.BindPort)
	if n, err := s1.Join([]string{s2Addr}); err != nil || n != 1 {
		t.Fatalf("Failed joining: %v (%d joined)", err, n)
	}

	// Query the regions list
	testutil.WaitForResult(func() (bool, error) {
		var arg structs.GenericRequest
		var out []string
		if err := msgpackrpc.CallWithCodec(codec, "Region.List", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(out) != 2 || out[0] != "region1" || out[1] != "region2" {
			t.Fatalf("unexpected regions: %v", out)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}
