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
	nomadapi "github.com/hashicorp/nomad/api"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
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
