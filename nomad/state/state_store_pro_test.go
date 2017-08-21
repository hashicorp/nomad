// +build pro ent

package state

import (
	"sort"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestStateStore_UpsertNamespace(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns := mock.Namespace()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NamespaceByName(ws, ns.Name)
	assert.Nil(err)

	assert.Nil(state.UpsertNamespace(1000, ns))
	assert.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NamespaceByName(ws, ns.Name)
	assert.Nil(err)
	assert.Equal(ns, out)

	index, err := state.Index(TableNamespaces)
	assert.Nil(err)
	assert.EqualValues(1000, index)
	assert.False(watchFired(ws))
}

// TODO test deleting when there are jobs/etc
func TestStateStore_DeleteNamespace(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns := mock.Namespace()

	assert.Nil(state.UpsertNamespace(1000, ns))

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NamespaceByName(ws, ns.Name)
	assert.Nil(err)

	assert.Nil(state.DeleteNamespace(1001, ns.Name))
	assert.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NamespaceByName(ws, ns.Name)
	assert.Nil(err)
	assert.Nil(out)

	index, err := state.Index(TableNamespaces)
	assert.Nil(err)
	assert.EqualValues(1001, index)
	assert.False(watchFired(ws))
}

func TestStateStore_Namespaces(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	var namespaces []*structs.Namespace

	for i := 0; i < 10; i++ {
		ns := mock.Namespace()
		namespaces = append(namespaces, ns)

		assert.Nil(state.UpsertNamespace(1000+uint64(i), ns))
	}

	// Create a watchset so we can test that getters don't cause it to fire
	ws := memdb.NewWatchSet()
	iter, err := state.Namespaces(ws)
	assert.Nil(err)

	var out []*structs.Namespace
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Namespace))
	}

	namespaceSort(namespaces)
	namespaceSort(out)
	assert.Equal(namespaces, out)
	assert.False(watchFired(ws))
}

func TestStateStore_NamespaceByNamePrefix(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns := mock.Namespace()

	ns.Name = "foobar"
	assert.Nil(state.UpsertNamespace(1000, ns))

	// Create a watchset so we can test that getters don't cause it to fire
	ws := memdb.NewWatchSet()
	iter, err := state.NamespacesByNamePrefix(ws, ns.Name)
	assert.Nil(err)

	gatherNamespaces := func(iter memdb.ResultIterator) []*structs.Namespace {
		var namespaces []*structs.Namespace
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			ns := raw.(*structs.Namespace)
			namespaces = append(namespaces, ns)
		}
		return namespaces
	}

	namespaces := gatherNamespaces(iter)
	assert.Len(namespaces, 1)
	assert.False(watchFired(ws))

	iter, err = state.NamespacesByNamePrefix(ws, "foo")
	assert.Nil(err)

	namespaces = gatherNamespaces(iter)
	assert.Len(namespaces, 1)

	ns = mock.Namespace()
	ns.Name = "foozip"
	err = state.UpsertNamespace(1001, ns)
	assert.Nil(err)
	assert.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	iter, err = state.NamespacesByNamePrefix(ws, "foo")
	assert.Nil(err)

	namespaces = gatherNamespaces(iter)
	assert.Len(namespaces, 2)

	iter, err = state.NamespacesByNamePrefix(ws, "foob")
	assert.Nil(err)

	namespaces = gatherNamespaces(iter)
	assert.Len(namespaces, 1)
	assert.False(watchFired(ws))
}

func TestStateStore_RestoreNamespace(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns := mock.Namespace()

	restore, err := state.Restore()
	assert.Nil(err)

	assert.Nil(restore.NamespaceRestore(ns))
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.NamespaceByName(ws, ns.Name)
	assert.Nil(err)
	assert.Equal(out, ns)
}

// namespaceSort is used to sort namespaces by name
func namespaceSort(namespaces []*structs.Namespace) {
	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})
}
