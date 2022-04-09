package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
)

func TestSystem_GarbageCollect(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.System()
	if err := e.GarbageCollect(); err != nil {
		t.Fatal(err)
	}
}
