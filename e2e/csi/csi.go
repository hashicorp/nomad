// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package csi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
)

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "CSI",
		CanRunLocal: true,
		Consul:      false,
		Cases: []framework.TestCase{
			new(CSIControllerPluginEBSTest), // see ebs.go
			new(CSINodeOnlyPluginEFSTest),   // see efs.go
		},
	})
}

const ns = ""

var pluginAllocWait = &e2e.WaitConfig{Interval: 5 * time.Second, Retries: 12} // 1min
var pluginWait = &e2e.WaitConfig{Interval: 5 * time.Second, Retries: 36}      // 3min
var reapWait = &e2e.WaitConfig{Interval: 5 * time.Second, Retries: 36}        // 3min

// assertNoErrorElseDump calls a non-halting assert on the error and dumps the
// plugin logs if it fails.
func assertNoErrorElseDump(f *framework.F, err error, msg string, pluginJobIDs []string) {
	if err != nil {
		dumpLogs(pluginJobIDs)
		f.Assert().NoError(err, fmt.Sprintf("%v: %v", msg, err))
	}
}

// requireNoErrorElseDump calls a halting assert on the error and dumps the
// plugin logs if it fails.
func requireNoErrorElseDump(f *framework.F, err error, msg string, pluginJobIDs []string) {
	if err != nil {
		dumpLogs(pluginJobIDs)
		f.NoError(err, fmt.Sprintf("%v: %v", msg, err))
	}
}

func dumpLogs(pluginIDs []string) error {

	for _, id := range pluginIDs {
		allocs, err := e2e.AllocsForJob(id, ns)
		if err != nil {
			return fmt.Errorf("could not find allocs for plugin: %v", err)
		}
		for _, alloc := range allocs {
			allocID := alloc["ID"]
			out, err := e2e.AllocLogs(allocID, "", e2e.LogsStdErr)
			if err != nil {
				return fmt.Errorf("could not get logs for alloc: %v\n%s", err, out)
			}
			_, isCI := os.LookupEnv("CI")
			if isCI {
				fmt.Println("--------------------------------------")
				fmt.Println("allocation logs:", allocID)
				fmt.Println(out)
				continue
			}
			f, err := os.Create(allocID + ".log")
			if err != nil {
				return fmt.Errorf("could not create log file: %v", err)
			}
			defer f.Close()
			_, err = f.WriteString(out)
			if err != nil {
				return fmt.Errorf("could not write to log file: %v", err)
			}
			fmt.Printf("nomad alloc logs written to %s.log\n", allocID)
		}
	}
	return nil
}

// waitForVolumeClaimRelease makes sure we don't try to re-claim a volume
// that's in the process of being unpublished. we can't just wait for allocs
// to stop, but need to wait for their claims to be released
func waitForVolumeClaimRelease(volID string, wc *e2e.WaitConfig) error {
	var out string
	var err error
	testutil.WaitForResultRetries(wc.Retries, func() (bool, error) {
		time.Sleep(wc.Interval)
		out, err = e2e.Command("nomad", "volume", "status", volID)
		if err != nil {
			return false, err
		}
		section, err := e2e.GetSection(out, "Allocations")
		if err != nil {
			return false, err
		}
		return strings.Contains(section, "No allocations placed"), nil
	}, func(e error) {
		if e == nil {
			err = nil
		}
		err = fmt.Errorf("alloc claim was not released: %v\n%s", e, out)
	})
	return err
}

// TODO(tgross): replace this w/ AllocFS().Stat() after
// https://github.com/hashicorp/nomad/issues/7365 is fixed
func readFile(client *api.Client, allocID string, path string) (bytes.Buffer, error) {
	var stdout, stderr bytes.Buffer
	alloc, _, err := client.Allocations().Info(allocID, nil)
	if err != nil {
		return stdout, err
	}
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()

	_, err = client.Allocations().Exec(ctx,
		alloc, "task", false,
		[]string{"cat", path},
		os.Stdin, &stdout, &stderr,
		make(chan api.TerminalSize), nil)
	return stdout, err
}

func waitForPluginStatusMinNodeCount(pluginID string, minCount int, wc *e2e.WaitConfig) error {

	return waitForPluginStatusCompare(pluginID, func(out string) (bool, error) {
		expected, err := e2e.GetField(out, "Nodes Expected")
		if err != nil {
			return false, err
		}
		expectedCount, err := strconv.Atoi(strings.TrimSpace(expected))
		if err != nil {
			return false, err
		}
		if expectedCount < minCount {
			return false, fmt.Errorf(
				"expected Nodes Expected >= %d, got %q", minCount, expected)
		}
		healthy, err := e2e.GetField(out, "Nodes Healthy")
		if err != nil {
			return false, err
		}
		if healthy != expected {
			return false, fmt.Errorf(
				"expected Nodes Healthy >= %d, got %q", minCount, healthy)
		}
		return true, nil
	}, wc)
}

func waitForPluginStatusControllerCount(pluginID string, count int, wc *e2e.WaitConfig) error {

	return waitForPluginStatusCompare(pluginID, func(out string) (bool, error) {

		expected, err := e2e.GetField(out, "Controllers Expected")
		if err != nil {
			return false, err
		}
		expectedCount, err := strconv.Atoi(strings.TrimSpace(expected))
		if err != nil {
			return false, err
		}
		if expectedCount != count {
			return false, fmt.Errorf(
				"expected Controllers Expected = %d, got %d", count, expectedCount)
		}
		healthy, err := e2e.GetField(out, "Controllers Healthy")
		if err != nil {
			return false, err
		}
		healthyCount, err := strconv.Atoi(strings.TrimSpace(healthy))
		if err != nil {
			return false, err
		}
		if healthyCount != count {
			return false, fmt.Errorf(
				"expected Controllers Healthy = %d, got %d", count, healthyCount)
		}
		return true, nil

	}, wc)
}

func waitForPluginStatusCompare(pluginID string, compare func(got string) (bool, error), wc *e2e.WaitConfig) error {
	var err error
	testutil.WaitForResultRetries(wc.Retries, func() (bool, error) {
		time.Sleep(wc.Interval)
		out, err := e2e.Command("nomad", "plugin", "status", pluginID)
		if err != nil {
			return false, err
		}
		return compare(out)
	}, func(e error) {
		err = fmt.Errorf("plugin status check failed: %v", e)
	})
	return err
}

// volumeRegister creates or registers a volume spec from a file but with a
// unique ID. The caller is responsible for recording that ID for later
// cleanup.
func volumeRegister(volID, volFilePath, createOrRegister string) error {

	// a CSI RPC to create a volume can take a long time because we
	// have to wait on the AWS API to provision a disk, but a register
	// should not because it only has to check the API for compatibility
	timeout := time.Second * 30
	if createOrRegister == "create" {
		timeout = time.Minute * 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nomad", "volume", createOrRegister, "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("could not open stdin?: %w", err)
	}

	content, err := os.ReadFile(volFilePath)
	if err != nil {
		return fmt.Errorf("could not open vol file: %w", err)
	}

	// hack off the first line to replace with our unique ID
	var idRegex = regexp.MustCompile(`(?m)^id[\s]+= ".*"`)
	volspec := idRegex.ReplaceAllString(string(content),
		fmt.Sprintf("id = %q", volID))

	// the EBS plugin uses the name as an idempotency token across the
	// whole AWS account, so it has to be globally unique
	var nameRegex = regexp.MustCompile(`(?m)^name[\s]+= ".*"`)
	volspec = nameRegex.ReplaceAllString(volspec,
		fmt.Sprintf("name = %q", uuid.Generate()))

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, volspec)
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not register vol: %w\n%v", err, string(out))
	}
	return nil
}
