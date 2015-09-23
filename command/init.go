package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

const (
	// DefaultInitName is the default name we use when
	// initializing the example file
	DefaultInitName = "example.nomad"
)

// InitCommand generates a new job template that you can customize to your
// liking, like vagrant init
type InitCommand struct {
	Meta
}

func (c *InitCommand) Help() string {
	helpText := `
Usage: nomad init

  Creates an example job file that can be used as a starting
  point to customize further.

`
	return strings.TrimSpace(helpText)
}

func (c *InitCommand) Synopsis() string {
	return "Create an example job file"
}

func (c *InitCommand) Run(args []string) int {
	// Check if the file already exists
	_, err := os.Stat(DefaultInitName)
	if err == nil || !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Job '%s' already exists", DefaultInitName))
		return 1
	} else if !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Failed to stat '%s': %v", DefaultInitName, err))
		return 1
	}

	// Write out the example
	err = ioutil.WriteFile(DefaultInitName, []byte(defaultJob), 0660)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to write '%s': %v", DefaultInitName, err))
		return 1
	}

	// Success
	c.Ui.Output(fmt.Sprintf("Example job file written to %s", DefaultInitName))
	return 0
}

const defaultJob = `
# There can only be a single job definition per file.
# Create a job with ID and Name 'example'
job "example" {
	# Run the job in the global region, which is the default.
	# region = "global"

	# Specify the datacenters within the region this job can run in.
	datacenters = ["dc1"]

	# Service type jobs optimize for long-lived services. This is
	# the default but we can change to batch for short-lived tasks.
	# type = "service"

	# Priority controls our access to resources and scheduling priority.
	# This can be 1 to 100, inclusively, and defaults to 50.
	# priority = 50

	# Restrict our job to only linux. We can specify multiple
	# constraints as needed.
	constraint {
		attribute = "$attr.kernel.name"
		value = "linux"
	}

	# Configure the job to do rolling updates
	update {
		# Stagger updates every 10 seconds
		stagger = "10s"

		# Update a single task at a time
		max_parallel = 1
	}

	# Create a 'cache' group. Each task in the group will be
	# scheduled onto the same machine.
	group "cache" {
		# Control the number of instances of this groups.
		# Defaults to 1
		# count = 1

		# Define a task to run
		task "redis" {
			# Use Docker to run the task.
			driver = "docker"

			# Configure Docker driver with the image
			config {
				image = "redis:latest"
			}

			# We must specify the resources required for
			# this task to ensure it runs on a machine with
			# enough capacity.
			resources {
				cpu = 500 # 500 Mhz
				memory = 256 # 256MB
				network {
					mbits = 10
					dynamic_ports = ["redis"]
				}
			}
		}
	}
}
`
