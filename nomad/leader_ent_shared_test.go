// +build pro ent

package nomad

import (
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestLeader_InitializeNamespaces(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer s1.Shutdown()

	// Wait for the evaluation to marked as cancelled
	state := s1.fsm.State()
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		out, err := state.NamespaceByName(ws, structs.DefaultNamespace)
		if err != nil {
			return false, err
		}
		return out != nil && out.Description == structs.DefaultNamespaceDescription, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}
