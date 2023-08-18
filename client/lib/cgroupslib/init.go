// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
)

// Init will initialize the cgroup tree that the Nomad client will use for
// isolating resources of tasks. cores is the cpuset granted for use by Nomad.
func Init(log hclog.Logger, cores string) {
	log.Info("INIT INIT", "cores", cores)

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
		// the cgroup controllers we need to activate at the root and on the nomad slice
		const activation = "+cpuset +cpu +io +memory +pids"

		// the name of the cgroup subtree interface file
		const subtreeFile = "cgroup.subtree_control"

		// the name of the cpuset partition interface file
		const partitionFile = "cpuset.cpus.partition"

		// the name of the cpuset interface file
		const cpusetFile = "cpuset.cpus"

		//
		// configuring root cgroup (/sys/fs/cgroup)
		//

		if err := writeCG2(activation, subtreeFile); err != nil {
			log.Error("failed to create nomad cgroup", "error", err)
			return
		}

		//
		// configuring nomad.slice
		//

		if err := mkCG2(NomadCgroupParent); err != nil {
			log.Error("failed to create nomad cgroup", "error", err)
			return
		}

		if err := writeCG2(activation, NomadCgroupParent, subtreeFile); err != nil {
			log.Error("failed to set subtree control on nomad cgroup", "error", err)
			return
		}

		if err := writeCG2(cores, NomadCgroupParent, cpusetFile); err != nil {
			log.Error("failed to write root partition cpuset", "error", err)
			return
		}

		if err := writeCG2("root", NomadCgroupParent, partitionFile); err != nil {
			log.Error("failed to set root partition mode", "error", err)
			return
		}

		// todo: write cpuset.cpus

		log.Debug("top level partition root nomad.slice cgroup initialized", "cpuset", "xxx")

		//
		// configuring nomad.slice/share
		//

		if err := mkCG2(NomadCgroupParent, "share"); err != nil {
			log.Error("failed to create share cgroup", "error", err)
			return
		}

		log.Debug("partition member nomad.slice/share cgroup initialized")

		//
		// configuring nomad.slice/reserve
		//

		if err := mkCG2(NomadCgroupParent, "reserve"); err != nil {
			log.Error("failed to create share cgroup", "error", err)
			return
		}

		if err := writeCG2("isolated", NomadCgroupParent, "reserve", "cpuset.cpus.partition"); err != nil {
			log.Error("failed to set cpuset partition root", "error", err)
			return
		}

		log.Debug("partition root nomad.slice/reserve cgroup initialized", "cpuset", "xxx")
	}
}

func readRootCG2(filename string) (string, error) {
	p := filepath.Join(root, filename)
	b, err := os.ReadFile(p)
	return string(bytes.TrimSpace(b)), err
}

func filepathCG2(paths ...string) string {
	base := []string{root}
	base = append(base, paths...)
	p := filepath.Join(base...)
	return p
}

func writeCG2(content string, paths ...string) error {
	p := filepathCG2(paths...)
	return os.WriteFile(p, []byte(content), 0644)
}

func mkCG2(paths ...string) error {
	p := filepathCG2(paths...)
	return os.MkdirAll(p, 0755)
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
