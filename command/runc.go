// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"syscall"
)

type RuncCommand struct {
}

func (c *RuncCommand) Help() string {
	return ""
}

func (c *RuncCommand) Name() string { return "runc" }

func (c *RuncCommand) Run(args []string) int {
	if slices.Contains(args, "create") && slices.Contains(args, "--bundle") {
		bundle := args[slices.Index(args, "--bundle")+1]

		jsonFile, _ := os.Open(fmt.Sprintf("%s/config.json", bundle))
		byteValue, _ := io.ReadAll(jsonFile)
		jsonFile.Close()

		var config map[string]interface{}
		err := json.Unmarshal([]byte(byteValue), &config)
		if err != nil {
			return 1
		}

		linuxConfig := config["linux"].(map[string]interface{})
		if config["annotations"] == nil {
			goto exec // Nothing to do
		}
		annotations := config["annotations"].(map[string]interface{})
		if annotations["network_ns"] == nil {
			goto exec // Nothing to do
		}
		namespaces := linuxConfig["namespaces"].([]interface{})
		// If there is a network namespace, modify it
		foundNetworkNamespace := false
		for i, v := range namespaces {
			extractedValue := v.(map[string]interface{})
			if extractedValue["type"] == "network" {
				extractedValue["path"] = annotations["network_ns"]
				namespaces[i] = extractedValue
				foundNetworkNamespace = true
				break
			}
		}
		// if not add one
		if !foundNetworkNamespace {
			namespace := map[string]interface{}{"type": "network", "path": annotations["network_ns"]}
			namespaces = append(namespaces, namespace)
		}
		linuxConfig["namespaces"] = namespaces
		config["linux"] = linuxConfig

		jsonBytes, _ := json.Marshal(config)
		os.WriteFile("/tmp/config.json", jsonBytes, 0600)
		os.WriteFile(fmt.Sprintf("%s/config.json", bundle), jsonBytes, 0600)
	}
exec:
	args = append([]string{"runc"}, args...)
	syscall.Exec("/usr/bin/runc", args, os.Environ())
	return 0
}

func (c *RuncCommand) Synopsis() string {
	return ""
}
