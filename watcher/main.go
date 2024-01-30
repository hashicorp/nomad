package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// make tyoe for job scheduling rules
// type JobStatusMap map[string]string
// type JobMeta map[string]string
// type JobSchedulingRules map[string]JobMeta

func main() {
	// GET CLI ARGS
	argZero := os.Args[0]
	argA := os.Args[1]
	restOfArgs := os.Args[2:]

	fmt.Println("argZero - ", argZero)
	fmt.Println("argA - ", argA)
	fmt.Println("restOfArgs - ", restOfArgs)

	if argA == "wait" {
		runAsBlocker(restOfArgs)
	} else {
		runAsWatcher()
	}
}

func runAsBlocker(argsList []string) {
	fmt.Println("I AM IN BLOCKING MODE")

	// Get my pid
	pid := os.Getpid()
	fmt.Println("My pid is:", pid)

	allocDir := os.Getenv("ALLOC_DIR")
	pidFilePath := allocDir + "/pid"
	lockFilePath := allocDir + "/lock"
	err := os.WriteFile(pidFilePath, []byte(fmt.Sprint(pid)), 0644)
	if err != nil {
		fmt.Println("Error writing pid file:", err)
		os.Exit(1)
	}

	for {
		fmt.Println("BLOCK BLOCK BLOCK")
		lockString, err := filePathToString(lockFilePath)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("lockString", lockString)

		if lockString == "run" {
			fmt.Println("starting core task")
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
		fmt.Println("Error executing command:", err)
	}
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
			fmt.Println("Error parsing start at:", err)
		}
		startAtTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), startAtTimeNoDate.Hour(), startAtTimeNoDate.Minute(), startAtTimeNoDate.Second(), 0, currentTime.Location())

		endAtTimeNoDate, err := time.Parse("15:04:05", endAt)
		if err != nil {
			fmt.Println("Error parsing end at:", err)
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

		if currentTime.Before(startAtTime) {
			timeUntilEnd = endAtTime.Sub(currentTime)
		} else {
			endAtTimeTomorrow := endAtTime.Add(24 * time.Hour)
			timeUntilEnd = endAtTimeTomorrow.Sub(currentTime)
		}

		outOfRange := currentTime.Before(startAtTime) || currentTime.After(endAtTime)

		fmt.Println("lockFilePath", lockFilePath)

		lockString, err := filePathToString(lockFilePath)
		if err != nil {
			fmt.Println("Error reading lock file:", err)
		}

		pidString, err := filePathToString(pidFilePath)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("main process pid is:", pidString)

		pidAsInt, err := strconv.Atoi(pidString)
		if err != nil {
			fmt.Println("error parsing pid as int:", pidString)
		}

		isRunningAccordingToFile := lockString == "run"

		if outOfRange {
			if isRunningAccordingToFile {
				if currentTime.Before(startAtTime) {
					fmt.Println("Before start")
				}
				if currentTime.After(endAtTime) {
					fmt.Println("After end")
				}

				fmt.Println("Writing stop to lockfile")
				err := os.WriteFile(lockFilePath, []byte("stop"), 0644)
				if err != nil {
					fmt.Println("STOP ERR:")
					fmt.Println(err)
				}

				// fmt.Println("Stopping the task with a SIGKILL")
				fmt.Println("Writing stop to lockfile")
				err = syscall.Kill(pidAsInt, syscall.SIGKILL)
				if err != nil {
					fmt.Println("Error signalining the main task to die:", err)
				}
				os.Exit(1)
				// os.Signal(os.Interrupt)
			} else {
				fmt.Println("Main task not running...")
				fmt.Println("Will start in, ", timeUntilStart.String())
			}
		} else {
			fmt.Println("Job should be running...")
			fmt.Println("Will stop in, ", timeUntilEnd.String())

			if lockString != "run" {
				fmt.Println("Writing the file")
				err := os.WriteFile(lockFilePath, []byte("run"), 0644)
				if err != nil {
					fmt.Println("Writing run to the file error:")
					fmt.Println(err)
				}
			}
		}

		time.Sleep(5 * time.Second)
	}
}

func filePathToString(path string) (string, error) {
	fmt.Println("Reading file: + ", path)
	lockContents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(lockContents), nil
}
