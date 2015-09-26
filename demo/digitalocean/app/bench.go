package main

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

func main() {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	total := 10000
	isRunning := false
	start := time.Now()
	allocClient := client.Allocations()

	cmd := exec.Command("nomad", "run", "bench.nomad")
	if err := cmd.Run(); err != nil {
		fmt.Println("nomad run failed: " + err.Error())
		return
	}

	fmt.Printf("benchmarking %d allocations\n", total)
	for i := 0; ; i++ {
		time.Sleep(100 * time.Millisecond)

		allocs, _, err := allocClient.List(nil)
		if err != nil {
			fmt.Println(err.Error())

			// keep going to paper over minor errors
			continue
		}
		now := time.Now()

		running := 0
		for _, alloc := range allocs {
			if alloc.ClientStatus == structs.AllocClientStatusRunning {
				if !isRunning {
					fmt.Printf("time to first running: %s\n", now.Sub(start))
					isRunning = true
				}
				running++
			}
		}

		if i%10 == 0 || running == total {
			fmt.Printf("%d running after %s\n", running, now.Sub(start))
		}

		if running == total {
			return
		}
	}
}
