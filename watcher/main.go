package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/hashicorp/nomad/api"
)

type JobStatusMap map[string]string
type JobSchedulingRules struct {
	END_AT   string
	START_AT string
	SCHEDULE string
	ALLOC_ID string
}
type NodeSchedulingRules map[string]JobSchedulingRules

func main() {
	// GET CLI ARGS
	// argZero := os.Args[0]
	initialArg := os.Args[1]
	restOfArgs := os.Args[2:]

	// slog.Debug("argZero - ", argZero)
	// slog.Debug("initialArg - ", initialArg)
	// slog.Debug("restOfArgs - ", restOfArgs)

	if initialArg == "block" {
		fmt.Println("Running as: blocker")
		runAsBlocker(restOfArgs)
	} else if initialArg == "system" {
		fmt.Println("Running as: system watcher")
		runAsSystemWatcher()
	} else if initialArg == "block-local" {
		fmt.Println("Running as: local blocker")
		runAsLocalBlocker(restOfArgs)
	} else {
		fmt.Println("Running as: watcher")
		runAsWatcher()
	}
}

func runAsBlocker(argsList []string) {
	// Get my pid
	pid := os.Getpid()
	slog.Debug("blocker", "pid:", pid)

	allocDir := os.Getenv("ALLOC_DIR")
	pidFilePath := allocDir + "/pid"
	lockFilePath := allocDir + "/lock"
	err := os.WriteFile(pidFilePath, []byte(fmt.Sprint(pid)), 0644)
	if err != nil {
		slog.Error("Error writing pid file:", err)
		os.Exit(1)
	}

	for {
		fmt.Println("=====")
		fmt.Println("Blocking task execution until unblocked by watcher...")
		fmt.Println("Current Time:", time.Now().String())
		fmt.Println("=====")
		lockString, err := filePathToString(lockFilePath)
		if err != nil {
			slog.Error("Error reading lockfile:", err)
		}
		slog.Debug("blocker", "lockString", lockString)

		if lockString == "run" {
			fmt.Println("Unblocked by watcher")
			fmt.Println("Task Starting!")
			break
		} else {
			time.Sleep(1 * time.Second)
		}
	}

	initialCommand := argsList[0]
	restArgs := argsList[1:]
	cmd := exec.Command(initialCommand, restArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	_, err = cmd.Output()
	if err != nil {
		slog.Error("Error executing command:", err)
	}
}

func runAsLocalBlocker(argsList []string) {
	startAtString := argsList[0]
	endAtString := argsList[1]

	for {
		// TODO: This does extra work in the loop!
		outOfRange, timeUntilStart, _, err := parseTimes(startAtString, endAtString)
		if err != nil {
			fmt.Println("Error parsing times:", err)
			os.Exit(1)
		}
		if *outOfRange {
			fmt.Println("=====")
			fmt.Println("Blocking task execution watcher...")
			fmt.Println("Unblocking in:", timeUntilStart.String())
			fmt.Println("=====")
			time.Sleep(1 * time.Second)
			continue
		}

		fmt.Println("Unblocked")
		fmt.Println("Task Starting")
		break
	}

	os.Exit(0)
}

func runAsWatcher() {
	// GET THE 3 ENV VARS YOU NEED
	// If time is even, remove the lockfile and kill the jib, log "killing job"
	// If time is odd, say "all good"

	// read 3 env vars: START_AT, END_AT, SCHEDULE
	// START_AT and END_AT are in the format "HH:MM:SS"
	// SCHEDULE is in the format "0 0 0 * * *"
	// SCHEDULE is a cron expression
	startAt := os.Getenv("START_AT")
	endAt := os.Getenv("END_AT")
	allocDir := os.Getenv("ALLOC_DIR")
	lockFilePath := allocDir + "/lock"
	pidFilePath := allocDir + "/pid"
	// schedule := os.Getenv("SCHEDULE")
	// skipDates := os.Getenv("SKIP_DATES")

	for {
		currentTime := time.Now()
		startAtTimeNoDate, err := time.Parse("15:04:05", startAt)
		if err != nil {
			slog.Error("Error parsing start at:", err)
		}
		startAtTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), startAtTimeNoDate.Hour(), startAtTimeNoDate.Minute(), startAtTimeNoDate.Second(), 0, currentTime.Location())

		endAtTimeNoDate, err := time.Parse("15:04:05", endAt)
		if err != nil {
			slog.Error("Error parsing end at:", err)
		}
		endAtTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), endAtTimeNoDate.Hour(), endAtTimeNoDate.Minute(), endAtTimeNoDate.Second(), 0, currentTime.Location())

		var timeUntilStart time.Duration
		var timeUntilEnd time.Duration
		if currentTime.Before(startAtTime) {
			timeUntilStart = startAtTime.Sub(currentTime)
		} else {
			startAtTimeTomorrow := startAtTime.Add(24 * time.Hour)
			timeUntilStart = startAtTimeTomorrow.Sub(currentTime)
		}

		if currentTime.Before(endAtTime) {
			timeUntilEnd = endAtTime.Sub(currentTime)
		} else {
			endAtTimeTomorrow := endAtTime.Add(24 * time.Hour)
			timeUntilEnd = endAtTimeTomorrow.Sub(currentTime)
		}

		outOfRange := currentTime.Before(startAtTime) || currentTime.After(endAtTime)

		lockString, err := filePathToString(lockFilePath)
		if err != nil {
			slog.Error("Error reading lock file:", err)
		}

		pidString, err := filePathToString(pidFilePath)
		if err != nil {
			slog.Error("Error parsing pidfile", err)
		}

		pidAsInt, err := strconv.Atoi(pidString)
		if err != nil {
			slog.Error("error parsing pid", err)
		}

		isRunningAccordingToFile := lockString == "run"

		if outOfRange {
			if isRunningAccordingToFile {
				if currentTime.Before(startAtTime) {
					fmt.Println("Main task should be stopped as it is before start time")
				}
				if currentTime.After(endAtTime) {
					fmt.Println("Main task should be stopped as it after end time")
				}

				err := os.WriteFile(lockFilePath, []byte("stop"), 0644)
				if err != nil {
					slog.Error("Error writing stop to lock:", err)
				}
				fmt.Println("Killing main task...")
				err = syscall.Kill(pidAsInt, syscall.SIGKILL)
				if err != nil {
					slog.Error("Error signalining the main task to die:", err)
				}
				os.Exit(1)
				// os.Signal(os.Interrupt)
			} else {
				fmt.Println("Main task not running")
				fmt.Println("Will start in: ", timeUntilStart.String())
			}
		} else {
			if lockString != "run" {
				fmt.Println("Unblocking the main task")
				err := os.WriteFile(lockFilePath, []byte("run"), 0644)
				if err != nil {
					slog.Error("Error writing run to lockfile:", err)
				}
			} else {
				fmt.Println("Main task is running")
				fmt.Println("Will stop in: ", timeUntilEnd.String())
				if lockString != "run" {
					fmt.Println("Writing the file")
					err := os.WriteFile(lockFilePath, []byte("run"), 0644)
					if err != nil {
						fmt.Println("Writing run to the file error:")
						fmt.Println(err)
					}
				}
			}
		}

		time.Sleep(3 * time.Second)
	}
}

func filePathToString(path string) (string, error) {
	slog.Debug("file-read", "path", path)
	lockContents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(lockContents), nil
}

func runAsSystemWatcher() {
	client, err := createNomadClient()
	if err != nil {
		slog.Error("nomad-watcher: error creating client: %v", err)
	}

	self, err := client.Agent().Self()
	if err != nil {
		slog.Error("nomad-watcher: error retrieving nodes: %v", err)
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

	nodeSchedulingRules := make(NodeSchedulingRules)
	updatedScheduleRules(client, nodeId, nodeSchedulingRules)

	ticker := time.NewTicker(1 * time.Second)

	go func() {
		for range ticker.C {
			handleSchedule(client, nodeSchedulingRules)
			updatedScheduleRules(client, nodeId, nodeSchedulingRules)
		}
	}()

	for {
		fmt.Println("Watcher alive")
		time.Sleep(10 * time.Second)
	}
}

func handleSchedule(client *api.Client, schedulesMap NodeSchedulingRules) {
	// Get and parse the stop time
	// If the job is running and it is past the stop time - send a kill command

	// TODO: IF THE JOB NEEDS TO START AND ITS ALLOC WAS GCED, WE WONT GET IT!
	// So then we would need to have some sort of schedule-aware

	// What if... this is really a pre-start task + a sidecar?
	// Prestart boots the main when the time passes (still reads job meta?)
	// No need to even use the Nomad client? (can I get meta in env?)
	// TODO: this should not be a prestart?!?!
	// Sidecar locks the lock, kills the task, turns it off in Nomad
	// Retries is infinite since this'll keep going?

	for jobId, jobRules := range schedulesMap {
		outOfRange, _, _, err := parseTimes(jobRules.START_AT, jobRules.END_AT)
		if err != nil {
			fmt.Println("Error parsing times:", err)
			continue
		}

		if *outOfRange {
			// if *timeUntilStart < 0 {
			// 	fmt.Println("Main task should be stopped as it is before start time")
			// }
			// if *timeUntilEnd < 0 {
			// 	fmt.Println("Main task should be stopped as it after end time")
			// }
			_, _, err := client.Jobs().Deregister(jobId, false, &api.WriteOptions{})
			if err != nil {
				fmt.Println("Error deregistering job:", err)
			}

			// fmt.Println("PAUSING JOB...")
			// q := &api.QueryOptions{}
			// alloc, _, err := client.Allocations().Info(jobRules.ALLOC_ID, q)
			// if err != nil {
			// 	fmt.Println("Error getting alloc info to signal:", err)
			// }

			// err = client.Allocations().Signal(alloc, q, "", "SIGSTOP")
			// if err != nil {
			// 	fmt.Println("Error signaling alloc:", err)
			// }
		} else {
			fmt.Println("Job " + jobId + " is running and all is well")

			// q := &api.QueryOptions{}
			// alloc, _, err := client.Allocations().Info(jobRules.ALLOC_ID, q)
			// if err != nil {
			// 	fmt.Println("Error getting alloc info to signal:", err)
			// }

			// err = client.Allocations().Signal(alloc, q, "", "SIGCONT")
			// if err != nil {
			// 	fmt.Println("Error signaling alloc:", err)
			// }
		}
	}
}

func updatedScheduleRules(client *api.Client, nodeId string, schedulesMap NodeSchedulingRules) {
	jobNameMap := make(JobStatusMap)

	// TODO: This doesn't work... why?
	allocFilter := "DesiredStatus == \"no\""
	allocQueryOpts := &api.QueryOptions{
		Filter: allocFilter,
	}
	allocs, _, err := client.Nodes().Allocations(nodeId, allocQueryOpts)
	if err != nil {
		fmt.Printf("nomad-watcher: error retrieving allocations: %v", err)
	}
	for _, alloc := range allocs {
		fmt.Println("Alloc Name:", alloc.Name)
		fmt.Println("Job for Alloc:", alloc.JobID)
		if alloc.DesiredStatus == "run" {
			fmt.Println("Alloc is running")
			jobNameMap[alloc.JobID] = alloc.ID
		}
	}

	for key, allocId := range jobNameMap {
		job, _, err := client.Jobs().Info(key, nil)
		if err != nil {
			fmt.Println("nomad-watcher: error retrieving job ", key, err)
			continue
		}

		if job.Meta["END_AT"] != "" {
			jobSchedule := JobSchedulingRules{
				END_AT:   job.Meta["END_AT"],
				START_AT: job.Meta["START_AT"],
				SCHEDULE: job.Meta["SCHEDULE"],
				ALLOC_ID: allocId,
			}

			schedulesMap[*job.ID] = jobSchedule
		}
	}
}

func createNomadClient() (*api.Client, error) {
	clientConfig := api.DefaultConfig()
	client, err := api.NewClient(clientConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func parseTimes(startAtString, endAtString string) (*bool, *time.Duration, *time.Duration, error) {
	currentTime := time.Now()
	startAtTimeNoDate, err := time.Parse("15:04:05", startAtString)
	if err != nil {
		fmt.Println("Error parsing start at:", err)
		return nil, nil, nil, err
	}
	startAtTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), startAtTimeNoDate.Hour(), startAtTimeNoDate.Minute(), startAtTimeNoDate.Second(), 0, currentTime.Location())

	endAtTimeNoDate, err := time.Parse("15:04:05", endAtString)
	if err != nil {
		fmt.Println("Error parsing end at:", err)
		return nil, nil, nil, err
	}
	endAtTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), endAtTimeNoDate.Hour(), endAtTimeNoDate.Minute(), endAtTimeNoDate.Second(), 0, currentTime.Location())

	var timeUntilStart time.Duration
	var timeUntilEnd time.Duration
	if currentTime.Before(startAtTime) {
		timeUntilStart = startAtTime.Sub(currentTime)
	} else {
		startAtTimeTomorrow := startAtTime.Add(24 * time.Hour)
		timeUntilStart = startAtTimeTomorrow.Sub(currentTime)
	}

	if currentTime.Before(endAtTime) {
		timeUntilEnd = endAtTime.Sub(currentTime)
	} else {
		endAtTimeTomorrow := endAtTime.Add(24 * time.Hour)
		timeUntilEnd = endAtTimeTomorrow.Sub(currentTime)
	}

	outOfRange := currentTime.Before(startAtTime) || currentTime.After(endAtTime)

	return &outOfRange, &timeUntilStart, &timeUntilEnd, nil
}
