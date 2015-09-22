package command

import (
	"fmt"
	"os"
	"path/filepath"
)

// InitCommand generates a new job template that you can customize to your
// liking, like vagrant init
type InitCommand struct {
	Meta
}

func (c *InitCommand) Help() string {
	return initUsage
}

func (c *InitCommand) Run(args []string) int {
	dir, err := os.Getwd()
	if err != nil {
		c.Ui.Error("Unable to determine pwd; aborting")
		return 1
	}

	// Derive the job name from the pwd folder name, which is our best guess at
	// the project's name
	jobname := filepath.Base(dir)
	jobfile := fmt.Sprintf("%s.nomad", jobname)
	jobpath := filepath.Join(dir, jobfile)
	if _, err := os.Stat(jobpath); err == nil {
		c.Ui.Error(fmt.Sprintf("%s file already exists", jobfile))
		return 1
	}

	file, err := os.Create(jobfile)
	defer file.Close()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Unable to create file %s: %s", jobfile, err))
		return 1
	}

	_, err = file.WriteString(defaultJob)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to write job template to %s", jobfile))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Initialized nomad job template in %s", jobfile))

	return 0
}

func (c *InitCommand) Synopsis() string {
	return "Create a new job template"
}

const initUsage = ``

const defaultJob = `
job "my-app" {
    region = "global"
    type = "service"
    priority = 50

    // Each task in the group will be scheduled on the same machine(s).
    group "app-group" {
        // How many copies of this group should we run?
        count = 5

        task "python-webapp" {
            driver = "docker"
            config {
                image = "org/container"
            }
            resources {
                // For CPU 1024 = 1ghz
                cpu = 500
                // Memory in megabytes
                memory = 128

                network {
                    dynamic_ports = [
                        "http",
                        "https",
                    ]
                }
            }
        }

        task "logshipper" {
            driver = "exec"
        }

        constraint {
            attribute = "kernel.os"
            value = "linux"
        }
    }
}
`
