package main

import (
	"fmt"
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

	total := 4000
	running := 0
	start := time.Now()
	allocClient := client.Allocations()

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

		for _, alloc := range allocs {
			if alloc.ClientStatus == structs.AllocClientStatusRunning {
				if running == 0 {
					fmt.Println("time to first running: %s", now.Sub(start))
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
