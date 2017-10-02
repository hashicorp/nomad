// +build pro

package state

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func testInitState(state *StateStore) error {
	if err := testInitDefaultNamespace(state); err != nil {
		return err
	}

	return nil
}

func TestStateStore_UpsertNamespaces_BadQuota(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns1 := mock.Namespace()
	ns1.Quota = "foo"
	assert.NotNil(state.UpsertNamespaces(1000, []*structs.Namespace{ns1}))
}
