// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/hashicorp/nomad/api"
)

func main() {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	var total int
	if len(os.Args) != 2 {
		fmt.Println("need 1 arg")
		return
	}

	if total, err = strconv.Atoi(os.Args[1]); err != nil {
		fmt.Println("arg 1 must be number")
		return
	}

	fh, err := os.CreateTemp("", "bench")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer os.Remove(fh.Name())

	jobContent := fmt.Sprintf(job, total)
	if _, err := fh.WriteString(jobContent); err != nil {
		fmt.Println(err.Error())
		return
	}
	fh.Close()

	isRunning := false
	allocClient := client.Allocations()

	cmd := exec.Command("nomad", "run", fh.Name())
	if err := cmd.Run(); err != nil {
		fmt.Println("nomad run failed: " + err.Error())
		return
	}
	start := time.Now()

	last := 0
	fmt.Printf("benchmarking %d allocations\n", total)
	opts := &api.QueryOptions{AllowStale: true}
	for {
		time.Sleep(100 * time.Millisecond)

		allocs, _, err := allocClient.List(opts)
		if err != nil {
			fmt.Println(err.Error())

			// keep going to paper over minor errors
			continue
		}
		now := time.Now()

		running := 0
		for _, alloc := range allocs {
			if alloc.ClientStatus == api.AllocClientStatusRunning {
				if !isRunning {
					fmt.Printf("time to first running: %s\n", now.Sub(start))
					isRunning = true
				}
				running++
			}
		}

		if last != running {
			fmt.Printf("%d running after %s\n", running, now.Sub(start))
		}
		last = running

		if running == total {
			return
		}
	}
}

const job = `
job "bench" {
	datacenters = ["ams2", "ams3", "nyc3", "sfo1"]

	group "cache" {
		count = %d

		task "redis" {
			driver = "docker"

			config {
				image = "redis"
			}

			resources {
				cpu = 100
				memory = 100
			}
		}
	}
}
`
