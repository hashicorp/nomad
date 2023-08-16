package main

// lazy global variables

import (
	"os"
	"path"
)

var (
	apiSock string

	lockPath  string
	jobID     string
	groupName string
	taskName  string

	allocID string

	allocDir string
	lockFile string
)

func init() {
	var ok bool

	if apiSock, ok = os.LookupEnv("API_SOCK"); !ok {
		apiSock = "/secrets/api.sock"
	}

	// determine what Nomad Variable path to use for the lock
	lockPath = os.Getenv("NOMAD_LOCK_PATH")
	// default if not ^: nomad/jobs/{jobID}/{group}/{task}
	if lockPath == "" {
		if jobID, ok = os.LookupEnv("NOMAD_JOB_ID"); !ok {
			jobID = "no-job-id"
		}
		if groupName, ok = os.LookupEnv("NOMAD_GROUP_NAME"); !ok {
			groupName = "no-group-name"
		}
		if taskName, ok = os.LookupEnv("NOMAD_TASK_NAME"); !ok {
			taskName = "no-task-name"
		}
		lockPath = path.Join("nomad", "jobs", jobID, groupName, taskName)
	}

	// we'll put the winning alloc id in the variable
	if allocID, ok = os.LookupEnv("NOMAD_ALLOC_ID"); !ok {
		allocID = "no-alloc-id"
	}

	lockFile = os.Getenv("NOMAD_LOCK_FILE")
	if lockFile == "" {
		// write file to alloc dir?  probably prefer the template{} signaling approach...
		if allocDir, ok = os.LookupEnv("NOMAD_ALLOC_DIR"); !ok {
			allocDir = "."
		}
		lockFile = path.Join(allocDir, "lock-file")
	}
}
