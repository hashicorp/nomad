package e2eutil

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/uuid"
)

// AgentRestart is a test helper function that restarts a client node
// running under systemd using a raw_exec job. Returns the job ID of
// the restart job so that callers can clean it up.
func AgentRestart(client *api.Client, nodeID string) (string, error) {
	ok, err := isUbuntu(client, nodeID)
	if !ok {
		// TODO(tgross): we're checking this because we want to use
		// systemctl to restart the node, but we should also figure
		// out a way to detect dev mode targets.
		return "", fmt.Errorf("AgentRestart only works against ubuntu targets")
	}
	if err != nil {
		return "", err
	}

	job := newRestartJob(nodeID)
	jobID := *job.ID
	_, _, err = client.Jobs().Register(job, nil)
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

func isUbuntu(client *api.Client, nodeID string) (bool, error) {
	node, _, err := client.Nodes().Info(nodeID, nil)
	if err != nil || node == nil {
		return false, err
	}
	if name, ok := node.Attributes["os.name"]; ok {
		return name == "ubuntu", nil
	}
	return false, nil
}

func newRestartJob(nodeID string) *api.Job {
	jobType := "batch"
	name := "restart"
	jobID := "restart-" + uuid.Generate()[0:8]
	attempts := 0
	job := &api.Job{
		Name:        &name,
		ID:          &jobID,
		Datacenters: []string{"dc1"},
		Type:        &jobType,
		TaskGroups: []*api.TaskGroup{
			{
				Name: &name,
				Constraints: []*api.Constraint{
					{
						LTarget: "${node.unique.id}",
						RTarget: nodeID,
						Operand: "=",
					},
				},
				RestartPolicy: &api.RestartPolicy{
					Attempts: &attempts,
				},
				Tasks: []*api.Task{
					{
						Name:   name,
						Driver: "raw_exec",
						Config: map[string]interface{}{
							"command": "systemctl",
							"args":    []string{"restart", "nomad"},
						},
					},
				},
			},
		},
	}
	job.Canonicalize()
	return job
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
