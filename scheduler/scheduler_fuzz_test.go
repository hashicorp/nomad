// TestScheduler_Fuzz is the only go test entry point in this file
// simulator.check() runs all the invariant state assertions
// simulator.fuzz* generate objects using the simulator.rand

// SCHEDULER_FUZZ_SIZE from the environment will set the cluster size (see newSimulator)
// SCHEDULER_FUZZ_SEED will replay a cluster simulation

// Test caching will prevent the test from running with new values, use either
// `go clean // -testcache` or
// `GOFLAGS="-count=1""

package scheduler

import (
	cr "crypto/rand"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"os"
	"strconv"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestScheduler_Fuzz(t *testing.T) {
	s := newSimulator(t)
	fmt.Printf("test seed: %d\n", s.seed)

	// Create some nodes
	for i := 0; i < s.size; i++ {
		node := s.fuzzNode()
		s.nodes = append(s.nodes, node)
	}

	// Create some jobs
	for i := 0; i < s.size; i++ {
		job := s.fuzzJob()
		s.jobs = append(s.jobs, job)
	}

	runErr := s.run()
	if runErr == nil {
		return
	}

	// Shrink
	var err error
	var prevNodes []*structs.Node
	var prevJobs []*structs.Job
	rnd := rand.New(rand.NewSource(randInt64()))

	for i := s.shrinkLimit; i > 0; i-- {
		prevNodes = s.nodes[:]
		prevJobs = s.jobs[:]

		rnd.Shuffle(len(s.nodes), func(i, j int) {
			t := s.nodes[i]
			s.nodes[i] = s.nodes[j]
			s.nodes[j] = t
		})

		rnd.Shuffle(len(s.jobs), func(i, j int) {
			t := s.jobs[i]
			s.jobs[i] = s.jobs[j]
			s.jobs[j] = t
		})

		s.nodes = s.nodes[:len(s.nodes)/2]
		s.jobs = s.jobs[:len(s.jobs)/2]

		s.reset()
		err = s.run()
		if err == nil {
			// we chose the wrong half, so shrink at this size again. re-seed
			// our shuffler so that we choose half of a different order next
			// time. Keep decrementing shrinkLimit.
			s.nodes = prevNodes
			s.jobs = prevJobs
			rnd = rand.New(rand.NewSource(randInt64()))
		}
	}

	t.Fatalf("property test failure %v\nreproduction data: nodes %#v\njobs %#v",
		err,
		s.nodes,
		s.jobs)
}

// simulator defines the fuzz testing setup
type simulator struct {
	// size of the simulation, scaled to nodes and jobs
	size int
	// shrinkLimit is the number of times to attempt to bisect the simulation looking
	// for a smaller dataset that produces the error
	shrinkLimit int

	seed int64
	rand *rand.Rand
	t    *testing.T
	h    *Harness

	// nodes and jobs under test, for shrinking
	nodes []*structs.Node
	jobs  []*structs.Job
}

// check is the place to add new invariant checks, it's called in run
func (s *simulator) check() error {
	if err := s.checkJobs(); err != nil {
		return err
	}
	return nil
}

func (s *simulator) checkJobs() error {
	ws := memdb.NewWatchSet()
	jobs, err := s.h.State.JobsByNamespace(ws, structs.DefaultNamespace)
	if err != nil {
		return fmt.Errorf("check: %v", err)
	}

	type ctx struct {
		job *structs.Job
		// map of taskgroup name to allocation
		allocs map[string][]*structs.Allocation
		// map of taskgroup name to blocked evals
		blocked []*structs.Evaluation
	}

	ctxs := []ctx{}

	for {
		raw := jobs.Next()
		if raw == nil {
			break
		}
		job := raw.(*structs.Job)

		c := ctx{
			job:     job,
			allocs:  map[string][]*structs.Allocation{},
			blocked: []*structs.Evaluation{},
		}

		allocs, _ := s.h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
		for _, a := range allocs {
			c.allocs[a.TaskGroup] = append(c.allocs[a.TaskGroup], a)
		}

		c.blocked, _ = s.h.State.EvalsByJob(ws, job.Namespace, job.ID)
		ctxs = append(ctxs, c)
	}

	// Jobs have the right number of allocs + blocked
	for _, c := range ctxs {
		switch c.job.Type {
		case structs.JobTypeSystem:
			if len(c.allocs)+len(c.blocked) != len(s.nodes) {
				return fmt.Errorf("check: job %s only has %d allocs, %d blocked",
					c.job.ID,
					len(c.allocs),
					len(c.blocked))
			}
		default:
			missing := false
			for _, tg := range c.job.TaskGroups {
				if tg.Count != len(c.allocs[tg.Name]) {
					missing = true
					break
				}
			}

			if missing && len(c.blocked) != 1 {
				return fmt.Errorf("check: job %s missing allocs, %d blocked",
					c.job.ID,
					len(c.blocked))
			}
		}

		// Jobs have summaries with the correct count
		for _, c := range ctxs {
			js, err := s.h.State.JobSummaryByID(ws, c.job.Namespace, c.job.ID)
			if err != nil {
				return fmt.Errorf("check: %v", err)
			}

			for name, ts := range js.Summary {
				if err != nil {
					return err
				}

				if ts.Running != len(c.allocs[name]) {
					return fmt.Errorf("check: running %d, expected %d", ts.Running, len(c.allocs[name]))
				}
			}
		}
	}

	return nil
}

func newSimulator(t *testing.T) *simulator {
	s := &simulator{
		size:        50,
		shrinkLimit: 25,
		t:           t,
		h:           NewHarness(t),
	}

	// Reading from the environment allows us to run small as a unit test and run a much
	// larger test async
	sizeEnv, err := strconv.Atoi(os.Getenv("SCHEDULER_FUZZ_SIZE"))
	if err == nil {
		s.size = sizeEnv
		s.shrinkLimit = sizeEnv / 2
	}

	// Running with the same seed will create the same set of input structs
	seedEnv, err := strconv.ParseInt(os.Getenv("SCHEDULER_FUZZ_SEED"), 10, 64)
	if err == nil {
		s.seed = seedEnv
	} else {
		s.seed = randInt64()
	}

	s.rand = rand.New(rand.NewSource(s.seed))
	return s
}

func randInt64() int64 {
	bn := new(big.Int).SetInt64(math.MaxInt64)
	bi, _ := cr.Int(cr.Reader, bn)
	return bi.Int64()
}

func (s *simulator) reset() {
	s.h = NewHarness(s.t)
}

func (s *simulator) run() error {
	// Create some nodes
	for _, node := range s.nodes {
		require.NoError(s.t, s.h.State.UpsertNode(s.h.NextIndex(), node))
	}

	// Register jobs
	for _, job := range s.jobs {
		require.NoError(s.t, s.h.State.UpsertJob(s.h.NextIndex(), job))

		// Create a mock evaluation to register the job
		eval := &structs.Evaluation{
			Namespace:   structs.DefaultNamespace,
			ID:          uuid.Generate(),
			Priority:    job.Priority,
			TriggeredBy: structs.EvalTriggerJobRegister,
			JobID:       job.ID,
			Status:      structs.EvalStatusPending,
		}

		err := s.h.State.UpsertEvals(s.h.NextIndex(), []*structs.Evaluation{eval})
		require.NoError(s.t, err)

		// Process the evaluation
		err = s.h.Process(NewServiceScheduler, eval)
		require.NoError(s.t, err)

		// Test invariants
		err = s.check()
	}

	return nil
}

func (s *simulator) fuzzNode() *structs.Node {
	cpu := int64(s.fuzzInt(4000))
	mem := int64(s.fuzzInt(8192))
	df := int64(s.fuzzInt(100) * 1024)

	rcpu := cpu / 40
	rmem := mem / 40
	rdf := df / 25 * 1024

	node := &structs.Node{
		ID:         uuid.Generate(),
		SecretID:   uuid.Generate(),
		Datacenter: "dc1",
		Name:       "node-" + uuid.Generate()[:8],
		Drivers: map[string]*structs.DriverInfo{
			"exec": {
				Detected: true,
				Healthy:  true,
			},
			"mock_driver": {
				Detected: true,
				Healthy:  true,
			},
		},
		Attributes: map[string]string{
			"kernel.name":        "linux",
			"arch":               "x86",
			"nomad.version":      "0.12.0",
			"driver.exec":        "1",
			"driver.mock_driver": "1",
		},

		// TODO Remove once clientv2 gets merged
		Resources: &structs.Resources{
			CPU:      int(cpu),
			MemoryMB: int(mem),
			DiskMB:   int(df),
		},
		Reserved: &structs.Resources{
			CPU:      int(rcpu),
			MemoryMB: int(rmem),
			DiskMB:   int(rdf),
			Networks: []*structs.NetworkResource{
				{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []structs.Port{{Label: "ssh", Value: 22}},
					MBits:         1,
				},
			},
		},

		NodeResources: &structs.NodeResources{
			Cpu: structs.NodeCpuResources{
				CpuShares: cpu,
			},
			Memory: structs.NodeMemoryResources{
				MemoryMB: mem,
			},
			Disk: structs.NodeDiskResources{
				DiskMB: df,
			},
			Networks: []*structs.NetworkResource{
				{
					Mode:   "host",
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
			NodeNetworks: []*structs.NodeNetworkResource{
				{
					Mode:   "host",
					Device: "eth0",
					Speed:  1000,
				},
			},
		},
		ReservedResources: &structs.NodeReservedResources{
			Cpu: structs.NodeReservedCpuResources{
				CpuShares: rcpu,
			},
			Memory: structs.NodeReservedMemoryResources{
				MemoryMB: rmem,
			},
			Disk: structs.NodeReservedDiskResources{
				DiskMB: rdf,
			},
			Networks: structs.NodeReservedNetworkResources{
				ReservedHostPorts: "22",
			},
		},
		Links: map[string]string{
			"consul": "foobar.dc1",
		},
		Meta: map[string]string{
			"pci-dss":  "true",
			"database": "mysql",
			"version":  "5.6",
		},
		NodeClass:             "linux-medium-pci",
		Status:                structs.NodeStatusReady,
		SchedulingEligibility: structs.NodeSchedulingEligible,
	}

	node.ComputeClass()
	return node
}

var jobTypes = []string{"service", "batch", "system"}
var taskGroupNames = []string{"one", "two", "three", "four"}

func (s *simulator) fuzzJob() *structs.Job {
	job := &structs.Job{
		ID:         uuid.Generate(),
		Namespace:  structs.DefaultNamespace,
		Type:       jobTypes[rand.Intn(3)],
		Priority:   s.fuzzInt(100),
		TaskGroups: s.fuzzTaskGroups(),
	}
	return job
}

func (s *simulator) fuzzTaskGroups() []*structs.TaskGroup {
	var out []*structs.TaskGroup
	// count in the range 1-10
	count := s.fuzzInt(10) + 1
	if count == 0 {
		count = 1
	}

	for _, name := range taskGroupNames[:rand.Intn(4)] {
		tg := &structs.TaskGroup{
			Name:  name,
			Count: count,
			Tasks: s.fuzzTasks(),
		}
		out = append(out, tg)
	}

	return out
}

func (s *simulator) fuzzTasks() []*structs.Task {
	var out []*structs.Task
	size := s.fuzzInt(10)

	for i := 0; i < size; i++ {
		t := &structs.Task{
			Resources: &structs.Resources{
				CPU:      s.fuzzInt(100),
				MemoryMB: s.fuzzInt(100),
				DiskMB:   s.fuzzInt(100),
			},
		}
		out = append(out, t)
	}

	return out
}

func (s *simulator) fuzzInt(exclusiveLimit int) int {
	return s.rand.Intn(exclusiveLimit)
}
