// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cluster3

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/api"
	nomadapi "github.com/hashicorp/nomad/api"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
	"oss.indeed.com/go/libtime"
)

type Cluster struct {
	t *testing.T

	consulClient *consulapi.Client
	nomadClient  *nomadapi.Client
	vaultClient  *vaultapi.Client

	timeout        time.Duration
	leaderReady    bool
	consulReady    bool
	vaultReady     bool
	linuxClients   int
	windowsClients int
	showState      bool
}

func (c *Cluster) wait() {
	errCh := make(chan error)

	statusAPI := c.nomadClient.Status()
	nodesAPI := c.nomadClient.Nodes()
	consulStatusAPI := c.consulClient.Status()
	vaultSysAPI := c.vaultClient.Sys()

	waitLeader := wait.InitialSuccess(
		wait.Timeout(c.timeout),
		wait.Gap(1*time.Second),
		wait.TestFunc(func() (bool, error) {
			if !c.leaderReady {
				return true, nil
			}
			result, err := statusAPI.Leader()
			return result != "", err
		}),
	)

	waitLinuxClients := wait.InitialSuccess(
		wait.Timeout(c.timeout),
		wait.Gap(1*time.Second),
		wait.ErrorFunc(func() error {
			if c.linuxClients <= 0 {
				return nil
			}
			queryOpts := &nomadapi.QueryOptions{
				Filter: `Attributes["kernel.name"] == "linux"`,
			}
			nodes, _, err := nodesAPI.List(queryOpts)
			if err != nil {
				return err
			}
			eligible := len(nodes)
			if eligible < c.linuxClients {
				return fmt.Errorf("not enough linux clients, want %d, got %d", c.linuxClients, eligible)
			}
			return nil
		}),
	)

	waitWindowsClients := wait.InitialSuccess(
		wait.Timeout(c.timeout),
		wait.Gap(1*time.Second),
		wait.ErrorFunc(func() error {
			if c.windowsClients <= 0 {
				return nil
			}
			return errors.New("todo: windows")
		}),
	)

	waitConsul := wait.InitialSuccess(
		wait.Timeout(c.timeout),
		wait.Gap(1*time.Second),
		wait.TestFunc(func() (bool, error) {
			if !c.consulReady {
				return true, nil
			}
			result, err := consulStatusAPI.Leader()
			return result != "", err
		}),
	)

	waitVault := wait.InitialSuccess(
		wait.Timeout(c.timeout),
		wait.Gap(1*time.Second),
		wait.TestFunc(func() (bool, error) {
			if !c.vaultReady {
				return true, nil
			}
			result, err := vaultSysAPI.Leader()
			if err != nil {
				return false, fmt.Errorf("failed to find vault leader: %w", err)
			}
			if result == nil {
				return false, errors.New("empty response for vault leader")
			}
			return result.ActiveTime.String() != "", nil
		}),
	)

	// todo: generalize

	go func() {
		err := waitLeader.Run()
		errCh <- err
	}()

	go func() {
		err := waitLinuxClients.Run()
		errCh <- err
	}()

	go func() {
		err := waitWindowsClients.Run()
		errCh <- err
	}()

	go func() {
		err := waitConsul.Run()
		errCh <- err
	}()

	go func() {
		err := waitVault.Run()
		errCh <- err
	}()

	for i := 0; i < 5; i++ {
		err := <-errCh
		must.NoError(c.t, err)
	}
}

type Option func(c *Cluster)

func Establish(t *testing.T, opts ...Option) {
	c := &Cluster{
		t:       t,
		timeout: 10 * time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	c.setClients()
	c.wait()
	c.dump()
}

func (c *Cluster) setClients() {
	nomadClient, nomadErr := nomadapi.NewClient(nomadapi.DefaultConfig())
	must.NoError(c.t, nomadErr, must.Sprint("failed to create nomad api client"))
	c.nomadClient = nomadClient

	consulClient, consulErr := consulapi.NewClient(consulapi.DefaultConfig())
	must.NoError(c.t, consulErr, must.Sprint("failed to create consul api client"))
	c.consulClient = consulClient

	vConfig := vaultapi.DefaultConfig()
	if os.Getenv("VAULT_ADDR") == "" {
		vConfig.Address = "http://localhost:8200"
	}
	vaultClient, vaultErr := vaultapi.NewClient(vConfig)
	must.NoError(c.t, vaultErr, must.Sprint("failed to create vault api client"))
	c.vaultClient = vaultClient
}

func Timeout(timeout time.Duration) Option {
	return func(c *Cluster) {
		c.timeout = timeout
	}
}

func LinuxClients(count int) Option {
	return func(c *Cluster) {
		c.linuxClients = count
	}
}

func WindowsClients(count int) Option {
	panic("not yet implemented")
	// return func(c *Cluster) {
	// c.windowsClients = count
	// }
}

func Leader() Option {
	return func(c *Cluster) {
		c.leaderReady = true
	}
}

func Consul() Option {
	return func(c *Cluster) {
		c.consulReady = true
	}
}

func Vault() Option {
	return func(c *Cluster) {
		c.vaultReady = true
	}
}

func ShowState() Option {
	return func(c *Cluster) {
		c.showState = true
	}
}

func (c *Cluster) dump() {
	if !c.showState {
		return
	}

	servers := func() {
		debug("\n--- LEADER / SERVER STATUS ---")
		statusAPI := c.nomadClient.Status()
		leader, leaderErr := statusAPI.Leader()
		must.NoError(c.t, leaderErr, must.Sprint("unable to get leader"))
		debug("leader:     %s", leader)
		peers, peersErr := statusAPI.Peers()
		must.NoError(c.t, peersErr, must.Sprint("unable to get peers"))
		for i, peer := range peers {
			debug("peer (%d/%d): %s", i+1, len(peers), peer)
		}
	}

	nodes := func() {
		debug("\n--- NODE STATUS ---")
		nodesAPI := c.nomadClient.Nodes()
		stubs, _, stubsErr := nodesAPI.List(nil)
		must.NoError(c.t, stubsErr, must.Sprint("unable to list nodes"))
		for i, stub := range stubs {
			node, _, nodeErr := nodesAPI.Info(stub.ID, nil)
			must.NoError(c.t, nodeErr, must.Sprint("unable to get node info"))
			debug("NODE %s @ %s (%d/%d)", node.Name, node.Datacenter, i+1, len(stubs))
			debug("\tID: %s", node.ID)
			shares, cores := node.NodeResources.Cpu.CpuShares, node.NodeResources.Cpu.TotalCpuCores
			debug("\tNodeResources: shares: %d, cores: %d", shares, cores)
			debug("\tPool: %s, Class: %q", node.NodePool, node.NodeClass)
			debug("\tStatus: %s %s", node.Status, node.StatusDescription)
			debug("\tDrain: %t", node.Drain)
			for driver, info := range node.Drivers {
				debug("\t[%s]", driver)
				debug("\t\tDetected: %t", info.Detected)
				debug("\t\tHealthy: %t %q", info.Healthy, info.HealthDescription)
			}
			debug("\tEvents")
			for i, event := range node.Events {
				debug("\t\t(%d/%d) %s @ %s", i+1, len(node.Events), event.Message, event.Timestamp)
			}
		}
	}

	allocs := func() {
		allocsAPI := c.nomadClient.Allocations()
		opts := &api.QueryOptions{Namespace: "*"}
		stubs, _, stubsErr := allocsAPI.List(opts)
		must.NoError(c.t, stubsErr, must.Sprint("unable to get allocs list"))
		debug("\n--- ALLOCATIONS (found %d) ---", len(stubs))
		for _, stub := range stubs {
			info, _, infoErr := allocsAPI.Info(stub.ID, nil)
			must.NoError(c.t, infoErr, must.Sprint("unable to get alloc"))
			debug("ALLOC (%s/%s, %s)", info.Namespace, *info.Job.ID, info.TaskGroup)
			debug("\tNode: %s, NodeID: %s", info.NodeName, info.NodeID)
			debug("\tClientStatus: %s %q", info.ClientStatus, info.ClientDescription)
			debug("\tClientTerminalStatus: %t", info.ClientTerminalStatus())
			debug("\tDesiredStatus: %s %q", info.DesiredStatus, info.DesiredDescription)
			debug("\tServerTerminalStatus: %t", info.ServerTerminalStatus())
			debug("\tDeployment: %s, Healthy: %t", info.DeploymentID, *info.DeploymentStatus.Healthy)
			for task, resources := range info.TaskResources {
				shares, cores, memory, memoryMax := *resources.CPU, *resources.Cores, *resources.MemoryMB, *resources.MemoryMaxMB
				debug("\tTask [%s] shares: %d, cores: %d, memory: %d, memory_max: %d", task, shares, cores, memory, memoryMax)
			}
		}
	}

	evals := func() {
		debug("\n--- EVALUATIONS ---")
		evalsAPI := c.nomadClient.Evaluations()
		opts := &api.QueryOptions{Namespace: "*"}
		stubs, _, stubsErr := evalsAPI.List(opts)
		must.NoError(c.t, stubsErr, must.Sprint("unable to list evaluations"))
		for i, stub := range stubs {
			eval, _, evalErr := evalsAPI.Info(stub.ID, opts)
			must.NoError(c.t, evalErr, must.Sprint("unable to get eval"))
			debug("EVAL (%d/%d) %s/%s on %q", i+1, len(stubs), eval.Namespace, eval.JobID, eval.NodeID)
			createTime := libtime.FromMilliseconds(eval.CreateTime / 1_000_000)
			debug("\tStatus: %s", eval.Status)
			debug("\tCreateIndex: %d, CreateTime: %s", eval.CreateIndex, createTime)
			debug("\tDeploymentID: %s", eval.DeploymentID)
			debug("\tQuotaLimitReached: %q", eval.QuotaLimitReached)
			debug("\tEscapedComputedClass: %t", eval.EscapedComputedClass)
			debug("\tBlockedEval: %q", eval.BlockedEval)
			debug("\tClassEligibility: %v", eval.ClassEligibility)
			debug("\tQueuedAllocations: %v", eval.QueuedAllocations)
		}
	}

	servers()
	nodes()
	allocs()
	evals()

	debug("\n--- END ---\n")

	// TODO
	// - deployments
	// - services
	// - anything else interesting
}

// debug uses Printf for outputting immediately to standard out instead of
// using Logf which witholds output until after the test runs
func debug(msg string, args ...any) {
	fmt.Printf(msg+"\n", args...)
}
