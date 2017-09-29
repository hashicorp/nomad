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

func testInitDefaultNamespace(state *StateStore) error {
	d := mock.Namespace()
	d.Name = structs.DefaultNamespace
	return state.UpsertNamespaces(1, []*structs.Namespace{d})
}

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

func TestStateStore_DeleteNamespaces_Default(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	ns := mock.Namespace()
	ns.Name = structs.DefaultNamespace
	assert.Nil(state.UpsertNamespaces(1000, []*structs.Namespace{ns}))

	err := state.DeleteNamespaces(1002, []string{ns.Name})
	assert.NotNil(err)
	assert.Contains(err.Error(), "can not be deleted")
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
		ns := raw.(*structs.Namespace)
		if ns.Name == structs.DefaultNamespace {
			continue
		}
		out = append(out, ns)
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

func TestStateStore_UpsertAlloc_AllocsByNamespace(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	ns1 := mock.Namespace()
	ns1.Name = "namespaced"
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc1.Namespace = ns1.Name
	alloc1.Job.Namespace = ns1.Name
	alloc2.Namespace = ns1.Name
	alloc2.Job.Namespace = ns1.Name

	ns2 := mock.Namespace()
	ns2.Name = "new-namespace"
	alloc3 := mock.Alloc()
	alloc4 := mock.Alloc()
	alloc3.Namespace = ns2.Name
	alloc3.Job.Namespace = ns2.Name
	alloc4.Namespace = ns2.Name
	alloc4.Job.Namespace = ns2.Name

	assert.Nil(state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))
	assert.Nil(state.UpsertJob(999, alloc1.Job))
	assert.Nil(state.UpsertJob(1000, alloc3.Job))

	// Create watchsets so we can test that update fires the watch
	watches := []memdb.WatchSet{memdb.NewWatchSet(), memdb.NewWatchSet()}
	_, err := state.AllocsByNamespace(watches[0], ns1.Name)
	assert.Nil(err)
	_, err = state.AllocsByNamespace(watches[1], ns2.Name)
	assert.Nil(err)

	assert.Nil(state.UpsertAllocs(1001, []*structs.Allocation{alloc1, alloc2, alloc3, alloc4}))
	assert.True(watchFired(watches[0]))
	assert.True(watchFired(watches[1]))

	ws := memdb.NewWatchSet()
	iter1, err := state.AllocsByNamespace(ws, ns1.Name)
	assert.Nil(err)
	iter2, err := state.AllocsByNamespace(ws, ns2.Name)
	assert.Nil(err)

	var out1 []*structs.Allocation
	for {
		raw := iter1.Next()
		if raw == nil {
			break
		}
		out1 = append(out1, raw.(*structs.Allocation))
	}

	var out2 []*structs.Allocation
	for {
		raw := iter2.Next()
		if raw == nil {
			break
		}
		out2 = append(out2, raw.(*structs.Allocation))
	}

	assert.Len(out1, 2)
	assert.Len(out2, 2)

	for _, alloc := range out1 {
		assert.Equal(ns1.Name, alloc.Namespace)
	}
	for _, alloc := range out2 {
		assert.Equal(ns2.Name, alloc.Namespace)
	}

	index, err := state.Index("allocs")
	assert.Nil(err)
	assert.EqualValues(1001, index)
	assert.False(watchFired(ws))
}

func TestStateStore_Deployments_Namespace(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	ns1 := mock.Namespace()
	ns1.Name = "namespaced"
	deploy1 := mock.Deployment()
	deploy2 := mock.Deployment()
	deploy1.Namespace = ns1.Name
	deploy2.Namespace = ns1.Name

	ns2 := mock.Namespace()
	ns2.Name = "new-namespace"
	deploy3 := mock.Deployment()
	deploy4 := mock.Deployment()
	deploy3.Namespace = ns2.Name
	deploy4.Namespace = ns2.Name

	assert.Nil(state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))

	// Create watchsets so we can test that update fires the watch
	watches := []memdb.WatchSet{memdb.NewWatchSet(), memdb.NewWatchSet()}
	_, err := state.DeploymentsByNamespace(watches[0], ns1.Name)
	assert.Nil(err)
	_, err = state.DeploymentsByNamespace(watches[1], ns2.Name)
	assert.Nil(err)

	assert.Nil(state.UpsertDeployment(1001, deploy1))
	assert.Nil(state.UpsertDeployment(1002, deploy2))
	assert.Nil(state.UpsertDeployment(1003, deploy3))
	assert.Nil(state.UpsertDeployment(1004, deploy4))
	assert.True(watchFired(watches[0]))
	assert.True(watchFired(watches[1]))

	ws := memdb.NewWatchSet()
	iter1, err := state.DeploymentsByNamespace(ws, ns1.Name)
	assert.Nil(err)
	iter2, err := state.DeploymentsByNamespace(ws, ns2.Name)
	assert.Nil(err)

	var out1 []*structs.Deployment
	for {
		raw := iter1.Next()
		if raw == nil {
			break
		}
		out1 = append(out1, raw.(*structs.Deployment))
	}

	var out2 []*structs.Deployment
	for {
		raw := iter2.Next()
		if raw == nil {
			break
		}
		out2 = append(out2, raw.(*structs.Deployment))
	}

	assert.Len(out1, 2)
	assert.Len(out2, 2)

	for _, deploy := range out1 {
		assert.Equal(ns1.Name, deploy.Namespace)
	}
	for _, deploy := range out2 {
		assert.Equal(ns2.Name, deploy.Namespace)
	}

	index, err := state.Index("deployment")
	assert.Nil(err)
	assert.EqualValues(1004, index)
	assert.False(watchFired(ws))
}

func TestStateStore_JobsByNamespace(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns1 := mock.Namespace()
	ns1.Name = "new"
	job1 := mock.Job()
	job2 := mock.Job()
	job1.Namespace = ns1.Name
	job2.Namespace = ns1.Name

	ns2 := mock.Namespace()
	ns2.Name = "new-namespace"
	job3 := mock.Job()
	job4 := mock.Job()
	job3.Namespace = ns2.Name
	job4.Namespace = ns2.Name

	assert.Nil(state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))

	// Create watchsets so we can test that update fires the watch
	watches := []memdb.WatchSet{memdb.NewWatchSet(), memdb.NewWatchSet()}
	_, err := state.JobsByNamespace(watches[0], ns1.Name)
	assert.Nil(err)
	_, err = state.JobsByNamespace(watches[1], ns2.Name)
	assert.Nil(err)

	assert.Nil(state.UpsertJob(1001, job1))
	assert.Nil(state.UpsertJob(1002, job2))
	assert.Nil(state.UpsertJob(1003, job3))
	assert.Nil(state.UpsertJob(1004, job4))
	assert.True(watchFired(watches[0]))
	assert.True(watchFired(watches[1]))

	ws := memdb.NewWatchSet()
	iter1, err := state.JobsByNamespace(ws, ns1.Name)
	assert.Nil(err)
	iter2, err := state.JobsByNamespace(ws, ns2.Name)
	assert.Nil(err)

	var out1 []*structs.Job
	for {
		raw := iter1.Next()
		if raw == nil {
			break
		}
		out1 = append(out1, raw.(*structs.Job))
	}

	var out2 []*structs.Job
	for {
		raw := iter2.Next()
		if raw == nil {
			break
		}
		out2 = append(out2, raw.(*structs.Job))
	}

	assert.Len(out1, 2)
	assert.Len(out2, 2)

	for _, job := range out1 {
		assert.Equal(ns1.Name, job.Namespace)
	}
	for _, job := range out2 {
		assert.Equal(ns2.Name, job.Namespace)
	}

	index, err := state.Index("jobs")
	assert.Nil(err)
	assert.EqualValues(1004, index)
	assert.False(watchFired(ws))
}

func TestStateStore_UpsertEvals_Namespace(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns1 := mock.Namespace()
	ns1.Name = "new"
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval1.Namespace = ns1.Name
	eval2.Namespace = ns1.Name

	ns2 := mock.Namespace()
	ns2.Name = "new-namespace"
	eval3 := mock.Eval()
	eval4 := mock.Eval()
	eval3.Namespace = ns2.Name
	eval4.Namespace = ns2.Name

	assert.Nil(state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))

	// Create watchsets so we can test that update fires the watch
	watches := []memdb.WatchSet{memdb.NewWatchSet(), memdb.NewWatchSet()}
	_, err := state.EvalsByNamespace(watches[0], ns1.Name)
	assert.Nil(err)
	_, err = state.EvalsByNamespace(watches[1], ns2.Name)
	assert.Nil(err)

	assert.Nil(state.UpsertEvals(1001, []*structs.Evaluation{eval1, eval2, eval3, eval4}))
	assert.True(watchFired(watches[0]))
	assert.True(watchFired(watches[1]))

	ws := memdb.NewWatchSet()
	iter1, err := state.EvalsByNamespace(ws, ns1.Name)
	assert.Nil(err)
	iter2, err := state.EvalsByNamespace(ws, ns2.Name)
	assert.Nil(err)

	var out1 []*structs.Evaluation
	for {
		raw := iter1.Next()
		if raw == nil {
			break
		}
		out1 = append(out1, raw.(*structs.Evaluation))
	}

	var out2 []*structs.Evaluation
	for {
		raw := iter2.Next()
		if raw == nil {
			break
		}
		out2 = append(out2, raw.(*structs.Evaluation))
	}

	assert.Len(out1, 2)
	assert.Len(out2, 2)

	for _, eval := range out1 {
		assert.Equal(ns1.Name, eval.Namespace)
	}
	for _, eval := range out2 {
		assert.Equal(ns2.Name, eval.Namespace)
	}

	index, err := state.Index("evals")
	assert.Nil(err)
	assert.EqualValues(1001, index)
	assert.False(watchFired(ws))
}

func TestStateStore_EvalsByIDPrefix_Namespaces(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	eval1 := mock.Eval()
	eval1.ID = "aabbbbbb-7bfb-395d-eb95-0685af2176b2"
	eval2 := mock.Eval()
	eval2.ID = "aabbcbbb-7bfb-395d-eb95-0685af2176b2"
	sharedPrefix := "aabb"

	ns1 := mock.Namespace()
	ns1.Name = "namespace1"
	ns2 := mock.Namespace()
	ns2.Name = "namespace2"
	eval1.Namespace = ns1.Name
	eval2.Namespace = ns2.Name

	assert.Nil(state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))
	assert.Nil(state.UpsertEvals(1000, []*structs.Evaluation{eval1, eval2}))

	gatherEvals := func(iter memdb.ResultIterator) []*structs.Evaluation {
		var evals []*structs.Evaluation
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			evals = append(evals, raw.(*structs.Evaluation))
		}
		return evals
	}

	ws := memdb.NewWatchSet()
	iter1, err := state.EvalsByIDPrefix(ws, ns1.Name, sharedPrefix)
	assert.Nil(err)
	iter2, err := state.EvalsByIDPrefix(ws, ns2.Name, sharedPrefix)
	assert.Nil(err)

	evalsNs1 := gatherEvals(iter1)
	evalsNs2 := gatherEvals(iter2)
	assert.Len(evalsNs1, 1)
	assert.Len(evalsNs2, 1)

	iter1, err = state.EvalsByIDPrefix(ws, ns1.Name, eval1.ID[:8])
	assert.Nil(err)

	evalsNs1 = gatherEvals(iter1)
	assert.Len(evalsNs1, 1)
	assert.False(watchFired(ws))
}

func TestStateStore_DeploymentsByIDPrefix_Namespaces(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	deploy1 := mock.Deployment()
	deploy1.ID = "aabbbbbb-7bfb-395d-eb95-0685af2176b2"
	deploy2 := mock.Deployment()
	deploy2.ID = "aabbcbbb-7bfb-395d-eb95-0685af2176b2"
	sharedPrefix := "aabb"

	ns1 := mock.Namespace()
	ns1.Name = "namespace1"
	ns2 := mock.Namespace()
	ns2.Name = "namespace2"
	deploy1.Namespace = ns1.Name
	deploy2.Namespace = ns2.Name

	assert.Nil(state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))
	assert.Nil(state.UpsertDeployment(1000, deploy1))
	assert.Nil(state.UpsertDeployment(1001, deploy2))

	gatherDeploys := func(iter memdb.ResultIterator) []*structs.Deployment {
		var deploys []*structs.Deployment
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			deploy := raw.(*structs.Deployment)
			deploys = append(deploys, deploy)
		}
		return deploys
	}

	ws := memdb.NewWatchSet()
	iter1, err := state.DeploymentsByIDPrefix(ws, ns1.Name, sharedPrefix)
	assert.Nil(err)
	iter2, err := state.DeploymentsByIDPrefix(ws, ns2.Name, sharedPrefix)
	assert.Nil(err)

	deploysNs1 := gatherDeploys(iter1)
	deploysNs2 := gatherDeploys(iter2)
	assert.Len(deploysNs1, 1)
	assert.Len(deploysNs2, 1)

	iter1, err = state.DeploymentsByIDPrefix(ws, ns1.Name, deploy1.ID[:8])
	assert.Nil(err)

	deploysNs1 = gatherDeploys(iter1)
	assert.Len(deploysNs1, 1)
	assert.False(watchFired(ws))
}

func TestStateStore_JobsByIDPrefix_Namespaces(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	job1 := mock.Job()
	job2 := mock.Job()

	ns1 := mock.Namespace()
	ns1.Name = "namespace1"
	ns2 := mock.Namespace()
	ns2.Name = "namespace2"

	jobID := "redis"
	job1.ID = jobID
	job2.ID = jobID
	job1.Namespace = ns1.Name
	job2.Namespace = ns2.Name

	assert.Nil(state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))
	assert.Nil(state.UpsertJob(1000, job1))
	assert.Nil(state.UpsertJob(1001, job2))

	gatherJobs := func(iter memdb.ResultIterator) []*structs.Job {
		var jobs []*structs.Job
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			jobs = append(jobs, raw.(*structs.Job))
		}
		return jobs
	}

	// Try full match
	ws := memdb.NewWatchSet()
	iter1, err := state.JobsByIDPrefix(ws, ns1.Name, jobID)
	assert.Nil(err)
	iter2, err := state.JobsByIDPrefix(ws, ns2.Name, jobID)
	assert.Nil(err)

	jobsNs1 := gatherJobs(iter1)
	assert.Len(jobsNs1, 1)

	jobsNs2 := gatherJobs(iter2)
	assert.Len(jobsNs2, 1)

	// Try prefix
	iter1, err = state.JobsByIDPrefix(ws, ns1.Name, "re")
	assert.Nil(err)
	iter2, err = state.JobsByIDPrefix(ws, ns2.Name, "re")
	assert.Nil(err)

	jobsNs1 = gatherJobs(iter1)
	jobsNs2 = gatherJobs(iter2)
	assert.Len(jobsNs1, 1)
	assert.Len(jobsNs2, 1)

	job3 := mock.Job()
	job3.ID = "riak"
	job3.Namespace = ns1.Name
	assert.Nil(state.UpsertJob(1003, job3))
	assert.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	iter1, err = state.JobsByIDPrefix(ws, ns1.Name, "r")
	assert.Nil(err)
	iter2, err = state.JobsByIDPrefix(ws, ns2.Name, "r")
	assert.Nil(err)

	jobsNs1 = gatherJobs(iter1)
	jobsNs2 = gatherJobs(iter2)
	assert.Len(jobsNs1, 2)
	assert.Len(jobsNs2, 1)

	iter1, err = state.JobsByIDPrefix(ws, ns1.Name, "ri")
	assert.Nil(err)

	jobsNs1 = gatherJobs(iter1)
	assert.Len(jobsNs1, 1)
	assert.False(watchFired(ws))
}

func TestStateStore_AllocsByIDPrefix_Namespaces(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	alloc1 := mock.Alloc()
	alloc1.ID = "aabbbbbb-7bfb-395d-eb95-0685af2176b2"
	alloc2 := mock.Alloc()
	alloc2.ID = "aabbcbbb-7bfb-395d-eb95-0685af2176b2"
	sharedPrefix := "aabb"

	ns1 := mock.Namespace()
	ns1.Name = "namespace1"
	ns2 := mock.Namespace()
	ns2.Name = "namespace2"

	alloc1.Namespace = ns1.Name
	alloc2.Namespace = ns2.Name

	assert.Nil(state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))
	assert.Nil(state.UpsertAllocs(1000, []*structs.Allocation{alloc1, alloc2}))

	gatherAllocs := func(iter memdb.ResultIterator) []*structs.Allocation {
		var allocs []*structs.Allocation
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			alloc := raw.(*structs.Allocation)
			allocs = append(allocs, alloc)
		}
		return allocs
	}

	ws := memdb.NewWatchSet()
	iter1, err := state.AllocsByIDPrefix(ws, ns1.Name, sharedPrefix)
	assert.Nil(err)
	iter2, err := state.AllocsByIDPrefix(ws, ns2.Name, sharedPrefix)
	assert.Nil(err)

	allocsNs1 := gatherAllocs(iter1)
	allocsNs2 := gatherAllocs(iter2)
	assert.Len(allocsNs1, 1)
	assert.Len(allocsNs2, 1)

	iter1, err = state.AllocsByIDPrefix(ws, ns1.Name, alloc1.ID[:8])
	assert.Nil(err)

	allocsNs1 = gatherAllocs(iter1)
	assert.Len(allocsNs1, 1)
	assert.False(watchFired(ws))
}
