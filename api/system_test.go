package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestSystem_GarbageCollect(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.System()
	err := e.GarbageCollect()
	must.NoError(t, err)
}
