package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestRegionsList(t *testing.T) {
	testutil.Parallel(t)
	c1, s1 := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.Region = "regionA"
	})
	defer s1.Stop()

	c2, s2 := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.Region = "regionB"
	})
	defer s2.Stop()

	// Join the servers
	_, err := c2.Agent().Join(s1.SerfAddr)
	must.NoError(t, err)

	f := func() error {
		regions, err := c1.Regions().List()
		if err != nil {
			return fmt.Errorf("failed to get regions: %w", err)
		}
		if n := len(regions); n != 2 {
			return fmt.Errorf("expected 2 regions, got %d", n)
		}
		if regions[0] != "regionA" {
			return fmt.Errorf("unexpected first region, got: %s", regions[0])
		}
		if regions[1] != "regionB" {
			return fmt.Errorf("unexpected second region, got: %s", regions[1])
		}
		return nil
	}
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(f),
		wait.Timeout(10*time.Second),
		wait.Gap(1*time.Second),
	))
}
