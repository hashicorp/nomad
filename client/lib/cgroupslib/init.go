// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set"
)

func Init(log hclog.Logger) {
	switch GetMode() {
	case CG1:
		// create the /nomad cgroup (or whatever the name is configured to be)
		// for each cgroup controller we are going to use
		controllers := []string{"freezer", "memory", "cpu", "cpuset"}
		for _, ctrl := range controllers {
			p := filepath.Join(root, ctrl, NomadCgroupParent)
			if err := os.MkdirAll(p, 0755); err != nil {
				log.Error("failed to create nomad cgroup", "controller", ctrl, "error", err)
			}
		}
	case CG2:
		// minimum controllers must be set first
		s, err := readRootCG2("cgroup.subtree_control")
		if err != nil {
			log.Error("failed to create nomad cgroup", "error", err)
			return
		}

		required := set.From([]string{"cpuset", "cpu", "io", "memory", "pids"})
		enabled := set.From(strings.Fields(s))
		needed := required.Difference(enabled)

		if needed.Size() == 0 {
			log.Debug("top level nomad.slice cgroup already exists")
			return // already setup
		}

		sb := new(strings.Builder)
		for _, controller := range needed.List() {
			sb.WriteString("+" + controller + " ")
		}

		activation := strings.TrimSpace(sb.String())
		if err = writeRootCG2("cgroup.subtree_control", activation); err != nil {
			log.Error("failed to create nomad cgroup", "error", err)
			return
		}

		nomadSlice := filepath.Join("/sys/fs/cgroup", NomadCgroupParent)
		if err := os.MkdirAll(nomadSlice, 0755); err != nil {
			log.Error("failed to create nomad cgroup", "error", err)
			return
		}

		log.Debug("top level nomad.slice cgroup initialized", "controllers", needed)
	}
}

func readRootCG2(filename string) (string, error) {
	p := filepath.Join(root, filename)
	b, err := os.ReadFile(p)
	return string(bytes.TrimSpace(b)), err
}

func writeRootCG2(filename, content string) error {
	p := filepath.Join(root, filename)
	return os.WriteFile(p, []byte(content), 0644)
}

// ReadNomadCG2 reads an interface file under the nomad.slice parent cgroup
// (or whatever its name is configured to be)
func ReadNomadCG2(filename string) (string, error) {
	p := filepath.Join(root, NomadCgroupParent, filename)
	b, err := os.ReadFile(p)
	return string(bytes.TrimSpace(b)), err
}

// ReadNomadCG1 reads an interface file under the /nomad cgroup of the given
// cgroup interface.
func ReadNomadCG1(iface, filename string) (string, error) {
	p := filepath.Join(root, iface, NomadCgroupParent, filename)
	b, err := os.ReadFile(p)
	return string(bytes.TrimSpace(b)), err
}

func WriteNomadCG1(iface, filename, content string) error {
	p := filepath.Join(root, iface, NomadCgroupParent, filename)
	return os.WriteFile(p, []byte(content), 0644)
}

// LinuxResourcesPath returns the filepath to the directory that the field
// x.Resources.LinuxResources.CpusetCgroupPath is expected to hold on to
func LinuxResourcesPath(allocID, task string) string {
	switch GetMode() {
	case CG1:
		return filepath.Join(root, "cpuset", NomadCgroupParent, scopeCG1(allocID, task))
	default:
		return filepath.Join(root, NomadCgroupParent, scopeCG2(allocID, task))
	}
}
