package nomad

import (
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

func testStateStore(t *testing.T) *StateStore {
	state, err := NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if state == nil {
		t.Fatalf("missing state")
	}
	return state
}

func mockNode() *structs.Node {
	node := &structs.Node{
		ID:         generateUUID(),
		Datacenter: "dc1",
		Name:       "foobar",
		Attributes: map[string]string{
			"os":            "linux",
			"arch":          "x86",
			"version":       "0.1.0",
			"driver.docker": "1.0.0",
		},
		Resources: &structs.Resources{
			CPU:      4.0,
			MemoryMB: 8192,
			DiskMB:   100 * 1024,
			IOPS:     150,
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					Public:        true,
					CIDR:          "192.168.0.100/32",
					ReservedPorts: []int{22},
					MBits:         1000,
				},
			},
		},
		Reserved: &structs.Resources{
			CPU:      0.1,
			MemoryMB: 256,
			DiskMB:   4 * 1024,
		},
		Links: map[string]string{
			"consul": "foobar.dc1",
		},
		Meta: map[string]string{
			"pci-dss": "true",
		},
		NodeClass: "linux-medium-pci",
		Status:    structs.NodeStatusInit,
	}
	return node
}

func mockJob() *structs.Job {
	job := &structs.Job{
		ID:        generateUUID(),
		Name:      "my-job",
		Type:      structs.JobTypeService,
		Priority:  50,
		AllAtOnce: false,
		Constraints: []*structs.Constraint{
			&structs.Constraint{
				Hard:    true,
				LTarget: "attr.os",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*structs.TaskGroup{
			&structs.TaskGroup{
				Name:  "web",
				Count: 10,
				Tasks: []*structs.Task{
					&structs.Task{
						Name:   "web",
						Driver: "docker",
						Config: map[string]string{
							"image":   "hashicorp/web",
							"version": "v1.2.3",
						},
						Resources: &structs.Resources{
							CPU:      0.5,
							MemoryMB: 256,
						},
					},
				},
				Meta: map[string]string{
					"elb_check_type":     "http",
					"elb_check_interval": "30s",
					"elb_check_min":      "3",
				},
			},
		},
		Meta: map[string]string{
			"owner": "armon",
		},
		Status: structs.JobStatusPending,
	}
	return job
}

func TestStateStore_RegisterNode_GetNode(t *testing.T) {
	state := testStateStore(t)
	node := mockNode()

	err := state.RegisterNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(node, out) {
		t.Fatalf("bad: %#v %#v", node, out)
	}

	index, err := state.GetIndex("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_DeregisterNode_GetNode(t *testing.T) {
	state := testStateStore(t)
	node := mockNode()

	err := state.RegisterNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.DeregisterNode(1001, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", node, out)
	}

	index, err := state.GetIndex("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_UpdateNode_GetNode(t *testing.T) {
	state := testStateStore(t)
	node := mockNode()

	err := state.RegisterNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.UpdateNodeStatus(1001, node.ID, structs.NodeStatusReady)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.Status != structs.NodeStatusReady {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.GetIndex("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_Nodes(t *testing.T) {
	state := testStateStore(t)
	var nodes []*structs.Node

	for i := 0; i < 10; i++ {
		node := mockNode()
		nodes = append(nodes, node)

		err := state.RegisterNode(1000+uint64(i), node)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	iter, err := state.Nodes()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Node
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Node))
	}

	sort.Sort(NodeIDSort(nodes))
	sort.Sort(NodeIDSort(out))

	if !reflect.DeepEqual(nodes, out) {
		t.Fatalf("bad: %#v %#v", nodes, out)
	}
}

func TestStateStore_RestoreNode(t *testing.T) {
	state := testStateStore(t)

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	node := mockNode()
	err = restore.NodeRestore(node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	restore.Commit()

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, node) {
		t.Fatalf("Bad: %#v %#v", out, node)
	}
}

func TestStateStore_RegisterJob_GetJob(t *testing.T) {
	state := testStateStore(t)
	job := mockJob()

	err := state.RegisterJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetJobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(job, out) {
		t.Fatalf("bad: %#v %#v", job, out)
	}

	index, err := state.GetIndex("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_UpdateRegisterJob_GetJob(t *testing.T) {
	state := testStateStore(t)
	job := mockJob()

	err := state.RegisterJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	job2 := mockJob()
	job2.ID = job.ID
	err = state.RegisterJob(1001, job2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetJobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(job2, out) {
		t.Fatalf("bad: %#v %#v", job2, out)
	}

	if out.CreateIndex != 1000 {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.GetIndex("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_DeregisterJob_GetJob(t *testing.T) {
	state := testStateStore(t)
	job := mockJob()

	err := state.RegisterJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.DeregisterJob(1001, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetJobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", job, out)
	}

	index, err := state.GetIndex("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_Jobs(t *testing.T) {
	state := testStateStore(t)
	var jobs []*structs.Job

	for i := 0; i < 10; i++ {
		job := mockJob()
		jobs = append(jobs, job)

		err := state.RegisterJob(1000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	iter, err := state.Jobs()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Job))
	}

	sort.Sort(JobIDSort(jobs))
	sort.Sort(JobIDSort(out))

	if !reflect.DeepEqual(jobs, out) {
		t.Fatalf("bad: %#v %#v", jobs, out)
	}
}

func TestStateStore_RestoreJob(t *testing.T) {
	state := testStateStore(t)

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	job := mockJob()
	err = restore.JobRestore(job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	restore.Commit()

	out, err := state.GetJobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, job) {
		t.Fatalf("Bad: %#v %#v", out, job)
	}
}

func TestStateStore_Indexes(t *testing.T) {
	state := testStateStore(t)
	node := mockNode()

	err := state.RegisterNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	iter, err := state.Indexes()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*IndexEntry
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*IndexEntry))
	}

	expect := []*IndexEntry{
		&IndexEntry{"nodes", 1000},
	}

	if !reflect.DeepEqual(expect, out) {
		t.Fatalf("bad: %#v %#v", expect, out)
	}
}

func TestStateStore_RestoreIndex(t *testing.T) {
	state := testStateStore(t)

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	index := &IndexEntry{"jobs", 1000}
	err = restore.IndexRestore(index)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	restore.Commit()

	out, err := state.GetIndex("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != 1000 {
		t.Fatalf("Bad: %#v %#v", out, 1000)
	}
}

// NodeIDSort is used to sort nodes by ID
type NodeIDSort []*structs.Node

func (n NodeIDSort) Len() int {
	return len(n)
}

func (n NodeIDSort) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}

func (n NodeIDSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// JobIDis used to sort jobs by id
type JobIDSort []*structs.Job

func (n JobIDSort) Len() int {
	return len(n)
}

func (n JobIDSort) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}

func (n JobIDSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
