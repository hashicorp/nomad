// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package e2eutil

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/testutil"
)

// AgentDisconnect is a test helper function that runs a raw_exec job
// that will disconnect a client at the network level and reconnect it
// after the specified period of time.
//
// Returns once the job is registered with the job ID of the restart
// job and any registration errors, not after the duration, so that
// callers can take actions while the client is down.
func AgentDisconnect(nodeID string, after time.Duration) (string, error) {
	jobID := "disconnect-" + nodeID
	vars := []string{"-var", "nodeID=" + nodeID}
	if after > 0 {
		vars = append(vars, "-var", fmt.Sprintf("time=%d", int(after.Seconds())))
	}

	jobFilePath := "../e2eutil/input/disconnect-node.nomad"

	// TODO: temporary hack around having older tests running on the
	// framework vs new tests not, as the framework has a different
	// working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if filepath.Base(dir) == "e2e" {
		jobFilePath = "e2eutil/input/disconnect-node.nomad"
	}

	err = RegisterWithArgs(jobID, jobFilePath, vars...)
	return jobID, err
}

// AgentRestartAfter is a test helper function that runs a raw_exec
// job that will stop a client and restart it after the specified
// period of time. The node must be running under systemd.
//
// Returns once the job is registered with the job ID of the restart
// job and any registration errors, not after the duration, so that
// callers can take actions while the client is down.
func AgentRestartAfter(nodeID string, after time.Duration) (string, error) {
	jobID := "restart-" + nodeID
	vars := []string{"-var", "nodeID=" + nodeID}
	if after > 0 {
		vars = append(vars, "-var", fmt.Sprintf("time=%d", int(after.Seconds())))
	}

	jobFilePath := "../e2eutil/input/restart-node.nomad"

	// TODO: temporary hack around having older tests running on the
	// framework vs new tests not, as the framework has a different
	// working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if filepath.Base(dir) == "e2e" {
		jobFilePath = "e2eutil/input/restart-node.nomad"
	}

	err = RegisterWithArgs(jobID, jobFilePath, vars...)
	return jobID, err
}

// AgentRestart is a test helper function that restarts a client node
// running under systemd using a raw_exec job. Returns the job ID of
// the restart job so that callers can clean it up.
func AgentRestart(client *api.Client, nodeID string) (string, error) {

	jobID, err := AgentRestartAfter(nodeID, 0)
	if err != nil {
		return jobID, err
	}

	reasonErr := fmt.Errorf("timed out")
	retries := 30
	for retries > 0 {
		time.Sleep(1 * time.Second)
		retries--

		allocStubs, _, err := client.Jobs().Allocations(jobID, true, nil)
		if err != nil {
			reasonErr = err
			continue
		}

		if len(allocStubs) > 0 {
		INNER:
			for _, state := range allocStubs[0].TaskStates {
				if state.State == "dead" {
					node, _, err := client.Nodes().Info(nodeID, nil)
					if err != nil {
						reasonErr = err
						break INNER
					}
					if node != nil && node.Status == "ready" {
						return jobID, nil
					}
					reasonErr = fmt.Errorf("node status not ready")
				}
			}
		}
	}
	return jobID, fmt.Errorf("node did not become ready: %v", reasonErr)
}

// ListWindowsClientNodes returns a list of Windows client IDs, so that tests
// can skip operating-specific tests if there are no Windows clients available.
// Returns an error only on client errors.
func ListWindowsClientNodes(client *api.Client) ([]string, error) {
	return listClientNodesByOS(client, "windows")
}

// ListLinuxClientNodes returns a list of Linux client IDs, so that tests
// can skip operating-specific tests if there are no Linux clients available
// Returns an error only on client errors.
func ListLinuxClientNodes(client *api.Client) ([]string, error) {
	return listClientNodesByOS(client, "linux")
}

func listClientNodesByOS(client *api.Client, osName string) ([]string, error) {
	nodeIDs := []string{}
	nodes, _, err := client.Nodes().List(&api.QueryOptions{})
	if err != nil {
		return nodeIDs, fmt.Errorf("could not query nodes: %v", err)
	}
	for _, stubNode := range nodes {
		node, _, err := client.Nodes().Info(stubNode.ID, nil)
		if err != nil {
			return nodeIDs, fmt.Errorf("could not query nodes: %v", err)
		}
		if name, ok := node.Attributes["kernel.name"]; ok && name == osName {
			nodeIDs = append(nodeIDs, stubNode.ID)
		}
	}
	return nodeIDs, nil
}

func NodeStatusList() ([]map[string]string, error) {

	out, err := Command("nomad", "node", "status", "-verbose")
	if err != nil {
		return nil, fmt.Errorf("'nomad node status' failed: %w", err)
	}

	nodes, err := ParseColumns(out)
	if err != nil {
		return nil, fmt.Errorf("could not parse node status output: %w", err)
	}
	return nodes, nil
}

func NodeStatusListFiltered(filterFn func(string) bool) ([]map[string]string, error) {

	out, err := Command("nomad", "node", "status", "-verbose")
	if err != nil {
		return nil, fmt.Errorf("'nomad node status' failed: %w", err)
	}

	allNodes, err := ParseColumns(out)
	if err != nil {
		return nil, fmt.Errorf("could not parse node status output: %w", err)
	}
	nodes := []map[string]string{}

	for _, node := range allNodes {
		out, err := Command("nomad", "node", "status", "-verbose", node["ID"])
		if err != nil {
			return nil, fmt.Errorf("could not node status output: %w", err)
		}
		if filterFn(out) {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

func WaitForNodeStatus(nodeID, status string, wc *WaitConfig) error {
	var got string
	var err error
	interval, retries := wc.OrDefault()
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)

		nodeStatuses, err := NodeStatusList()
		if err != nil {
			return false, err
		}
		for _, nodeStatus := range nodeStatuses {
			if nodeStatus["ID"] == nodeID {
				got = nodeStatus["Status"]
				if got == status {
					return true, nil
				}
			}
		}
		return false, nil
	}, func(e error) {
		err = fmt.Errorf("node status check failed: got %#v", got)
	})
	return err
}
