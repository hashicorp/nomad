package main

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
)

func main() {
	fmt.Println("Hello, world.")
	client, err := createNomadClient()
	if err != nil {
		fmt.Printf("nomad-watcher: error creating client: %v", err)
	}

	self, err := client.Agent().Self()
	if err != nil {
		fmt.Printf("nomad-watcher: error retrieving nodes: %v", err)
	}

	fmt.Println("Agent ID is: ", self.Member.Name)

	// TODO: Change so that this works
	// Pretty sure I can just interpolate this into an env var!!
	// ${node.unique.name} Or id?
	nodeName := "mike-VMW1R39HWM"
	nodeFilter := "Name == \"" + nodeName + "\""
	nodeQueryOpts := &api.QueryOptions{
		Filter: nodeFilter,
	}
	nodes, _, err := client.Nodes().List(nodeQueryOpts)
	if err != nil {
		fmt.Printf("nomad-watcher: error retrieving nodes: %v", err)
	}

	nodeId := ""

	for _, node := range nodes {
		fmt.Println("Node Id:", node.ID)
		nodeId = node.ID
	}

	// TODO: This doesn't work... why?
	allocFilter := "DesiredStatus == \"no\""
	allocQueryOpts := &api.QueryOptions{
		Filter: allocFilter,
	}
	allocs, _, err := client.Nodes().Allocations(nodeId, allocQueryOpts)
	if err != nil {
		fmt.Printf("nomad-watcher: error retrieving allocations: %v", err)
	}

	jobNameMap := make(map[string]string)
	jobSchedulingRules := make(map[string]map[string]string)

	for _, alloc := range allocs {
		// fmt.Println("Alloc Name:", alloc.Name)
		// fmt.Println("Job for Alloc:", alloc.JobID)
		jobNameMap[alloc.JobID] = alloc.DesiredStatus
	}

	for key, value := range jobNameMap {
		fmt.Println("Job ID:", key, "Alloc Desired Status:", value)
	}

	for key, _ := range jobNameMap {
		fmt.Println("nomad-watcher: Getting meta for job: ", key)

		job, _, err := client.Jobs().Info(key, nil)
		if err != nil {
			fmt.Println("nomad-watcher: error retrieving job ", key, err)
			continue
		}

		fmt.Println("Meta:", job.Meta)

		if job.Meta["schedule"] != "" {
			fmt.Println("Job ID:", key, "Schedule:", job.Meta["schedule"])
			jobSchedulingRules[key] = job.Meta
		}
	}
}

func getUpdatedScheduleRules() {
}

func createNomadClient() (*api.Client, error) {
	clientConfig := api.DefaultConfig()
	client, err := api.NewClient(clientConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}
