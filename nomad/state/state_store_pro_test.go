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

func TestStateStore_UpsertNamespaces(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NamespaceByName(ws, ns1.Name)
	assert.Nil(err)

	assert.Nil(state.UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2}))
	assert.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NamespaceByName(ws, ns1.Name)
	assert.Nil(err)
	assert.Equal(ns1, out)

	out, err = state.NamespaceByName(ws, ns2.Name)
	assert.Nil(err)
	assert.Equal(ns2, out)

	index, err := state.Index(TableNamespaces)
	assert.Nil(err)
	assert.EqualValues(1000, index)
	assert.False(watchFired(ws))
}

func TestStateStore_DeleteNamespaces(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()

	assert.Nil(state.UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2}))

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NamespaceByName(ws, ns1.Name)
	assert.Nil(err)

	assert.Nil(state.DeleteNamespaces(1001, []string{ns1.Name, ns2.Name}))
	assert.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NamespaceByName(ws, ns1.Name)
	assert.Nil(err)
	assert.Nil(out)

	out, err = state.NamespaceByName(ws, ns2.Name)
	assert.Nil(err)
	assert.Nil(out)

	index, err := state.Index(TableNamespaces)
	assert.Nil(err)
	assert.EqualValues(1001, index)
	assert.False(watchFired(ws))
}

func TestStateStore_DeleteNamespaces_NonTerminalJobs(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	ns := mock.Namespace()
	assert.Nil(state.UpsertNamespaces(1000, []*structs.Namespace{ns}))

	job := mock.Job()
	job.Namespace = ns.Name
	assert.Nil(state.UpsertJob(1001, job))

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NamespaceByName(ws, ns.Name)
	assert.Nil(err)

	err = state.DeleteNamespaces(1002, []string{ns.Name})
	assert.NotNil(err)
	assert.Contains(err.Error(), "one non-terminal")
	assert.False(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NamespaceByName(ws, ns.Name)
	assert.Nil(err)
	assert.NotNil(out)

	index, err := state.Index(TableNamespaces)
	assert.Nil(err)
	assert.EqualValues(1000, index)
	assert.False(watchFired(ws))
}

func TestStateStore_Namespaces(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	var namespaces []*structs.Namespace

	for i := 0; i < 10; i++ {
		ns := mock.Namespace()
		namespaces = append(namespaces, ns)
	}

	assert.Nil(state.UpsertNamespaces(1000, namespaces))

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
	assert.Nil(state.UpsertNamespaces(1000, []*structs.Namespace{ns}))

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
	err = state.UpsertNamespaces(1001, []*structs.Namespace{ns})
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
