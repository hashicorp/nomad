// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package runcshim

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	ExitError = 1
)

func exitWithMsg(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(ExitError)
}

// This init() must be initialized last in package required by the child plugin
// process. It's recommended to avoid any other `init()` or inline any necessary
// calls here. See eeaa95d commit message for more details.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "runcshim" {
		if len(os.Args) <= 3 {
			exitWithMsg("expected path to runc compatible binary")
		}

		if slices.Contains(os.Args, "create") {
			var bundleRoot string
			if bundleIndex := slices.Index(os.Args, "--bundle"); bundleIndex != -1 {
				if !(bundleIndex+1 < len(os.Args)) {
					exitWithMsg("bundle directory not passed")
				}
				bundleRoot = os.Args[bundleIndex+1]
			} else { // Use cwd
				wd, err := os.Getwd()
				if err != nil {
					exitWithMsg(fmt.Sprint(err))
				}
				bundleRoot = wd
			}
			configFile := fmt.Sprintf("%s/config.json", bundleRoot)
			jsonFile, err := os.Open(configFile)
			if err != nil {
				exitWithMsg(fmt.Sprintf("Could not open %q: %v", configFile, err))
			}
			byteValue, err := io.ReadAll(jsonFile)
			if err != nil {
				jsonFile.Close()
				exitWithMsg(fmt.Sprintf("Could not read %q: %v", configFile, err))
			}
			jsonFile.Close()

			var spec specs.Spec
			err = json.Unmarshal(byteValue, &spec)
			if err != nil {
				exitWithMsg(fmt.Sprintf("Could not unmarshal config: %v", err))
			}

			annotation, ok := spec.Annotations["network_ns"]
			if !ok {
				exitWithMsg("Missing `network_ns` annotation. Are we called from Nomad?")
			}
			if spec.Linux == nil {
				exitWithMsg("Missing `linux` configuration, you are using linux are you?")
			}

			// If there is a network namespace, modify it
			foundNetworkNamespace := false
			for idx := range spec.Linux.Namespaces {
				if spec.Linux.Namespaces[idx].Type == "network" {
					spec.Linux.Namespaces[idx].Path = annotation
					foundNetworkNamespace = true
					break
				}
			}
			// if not add one
			if !foundNetworkNamespace {
				var namespace = specs.LinuxNamespace{Type: "network", Path: annotation}
				spec.Linux.Namespaces = append(spec.Linux.Namespaces, namespace)
			}

			jsonBytes, err := json.Marshal(spec)
			if err != nil {
				exitWithMsg(fmt.Sprintf("Could not marshal config: %v", err))
			}
			err = os.WriteFile(configFile, jsonBytes, 0600)
			if err != nil {
				exitWithMsg(fmt.Sprintf("Failed writing config.json: %v", err))
			}
		}

		runc_binary := os.Args[2]
		// Resolve full path via $PATH
		runc_binary, err := exec.LookPath(runc_binary)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not resolve full path for %q: %v", runc_binary, err)
			os.Exit(ExitError)
		}
		args := append([]string{filepath.Base(runc_binary)}, os.Args[3:]...)
		syscall.Exec(runc_binary, args, os.Environ())
	}
}
