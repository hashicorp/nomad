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
		// the name of the cpuset interface file
		const cpusetFile = "cpuset.cpus"

		// the name of the cpuset mems interface file
		const memsFile = "cpuset.mems"

		const memsSet = "0" // TODO(shoenig) get from topology

		// the name of the clone_children interface file
		const cloneChilds = "cgroup.clone_children"

		// create the /nomad cgroup (or whatever the name is configured to be)
		// for each cgroup controller we are going to use
		controllers := []string{"freezer", "memory", "cpu", "cpuset"}
		for _, ctrl := range controllers {
			p := filepath.Join(root, ctrl, NomadCgroupParent)
			if err := os.MkdirAll(p, 0755); err != nil {
				log.Error("failed to create nomad cgroup", "controller", ctrl, "error", err)
			}
		}

		//
		// configure cpuset partitioning
		//

		if err := writeCG(memsSet, "cpuset", NomadCgroupParent, memsFile); err != nil {
			log.Error("failed to set cpuset.mems on nomad cpuset cgroup", "error", err)
		}

		if err := writeCG("1", "cpuset", NomadCgroupParent, cloneChilds); err != nil {
			log.Error("failed to set clone_children on nomad cpuset cgroup", "error", err)
			return
		}

		if err := writeCG(cores, "cpuset", NomadCgroupParent, cpusetFile); err != nil {
			log.Error("failed to write cores to nomad cpuset cgroup", "error", err)
			return
		}

		if err := mkCG("cpuset", NomadCgroupParent, SharePartition()); err != nil {
			log.Error("failed to create share cpuset partition", "error", err)
			return
		}

		if err := writeCG("0", "cpuset", NomadCgroupParent, SharePartition(), memsFile); err != nil {
			log.Error("failed to set cpuset.mems on share cpuset partition", "error", err)
			return
		}

		if err := writeCG("0", "cpuset", NomadCgroupParent, SharePartition(), cloneChilds); err != nil {
			log.Error("failed to set clone_children on nomad cpuset cgroup", "error", err)
			return
		}

		if err := mkCG("cpuset", NomadCgroupParent, ReservePartition()); err != nil {
			log.Error("failed to create reserve cpuset partition", "error", err)
			return
		}

		if err := writeCG("0", "cpuset", NomadCgroupParent, ReservePartition(), memsFile); err != nil {
			log.Error("failed to set cpuset.mems on reserve cpuset partition", "error", err)
			return
		}

		if err := writeCG("0", "cpuset", NomadCgroupParent, ReservePartition(), cloneChilds); err != nil {
			log.Error("failed to set clone_children on nomad cpuset cgroup", "error", err)
			return
		}

		log.Debug("nomad cpuset partitions initialized", "cores", cores)

	case CG2:
		// the cgroup controllers we need to activate at the root and on the nomad slice
		const activation = "+cpuset +cpu +io +memory +pids"

		// the name of the cgroup subtree interface file
		const subtreeFile = "cgroup.subtree_control"

		// the name of the cpuset interface file
		const cpusetFile = "cpuset.cpus"

		//
		// configuring root cgroup (/sys/fs/cgroup)
		//

		if err := writeCG(activation, subtreeFile); err != nil {
			log.Error("failed to create nomad cgroup", "error", err)
			return
		}

		//
		// configuring nomad.slice
		//

		if err := mkCG(NomadCgroupParent); err != nil {
			log.Error("failed to create nomad cgroup", "error", err)
			return
		}

		if err := writeCG(activation, NomadCgroupParent, subtreeFile); err != nil {
			log.Error("failed to set subtree control on nomad cgroup", "error", err)
			return
		}

		if err := writeCG(cores, NomadCgroupParent, cpusetFile); err != nil {
			log.Error("failed to write root partition cpuset", "error", err)
			return
		}

		log.Debug("top level partition root nomad.slice cgroup initialized")

		//
		// configuring nomad.slice/share (member)
		//

		if err := mkCG(NomadCgroupParent, SharePartition()); err != nil {
			log.Error("failed to create share cgroup", "error", err)
			return
		}

		if err := writeCG(activation, NomadCgroupParent, SharePartition(), subtreeFile); err != nil {
			log.Error("failed to set subtree control on cpuset share partition", "error", err)
			return
		}

		log.Debug("partition member nomad.slice/share cgroup initialized")

		//
		// configuring nomad.slice/reserve (member)
		//

		if err := mkCG(NomadCgroupParent, ReservePartition()); err != nil {
			log.Error("failed to create share cgroup", "error", err)
			return
		}

		if err := writeCG(activation, NomadCgroupParent, ReservePartition(), subtreeFile); err != nil {
			log.Error("failed to set subtree control on cpuset reserve partition", "error", err)
			return
		}

		log.Debug("partition member nomad.slice/reserve cgroup initialized")
	}
}

func readRootCG2(filename string) (string, error) {
	p := filepath.Join(root, filename)
	b, err := os.ReadFile(p)
	return string(bytes.TrimSpace(b)), err
}

// filepathCG will return the given paths based on the cgroup root
func filepathCG(paths ...string) string {
	base := []string{root}
	base = append(base, paths...)
	p := filepath.Join(base...)
	return p
}

// writeCG will write content to the cgroup interface file given by paths
func writeCG(content string, paths ...string) error {
	p := filepathCG(paths...)
	return os.WriteFile(p, []byte(content), 0644)
}

// mkCG will create a cgroup at the given path
func mkCG(paths ...string) error {
	p := filepathCG(paths...)
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
func LinuxResourcesPath(allocID, task string, reserveCores bool) string {
	partition := GetPartitionFromBool(reserveCores)
	switch GetMode() {
	case CG1:
		return filepath.Join(root, "cpuset", NomadCgroupParent, partition, scopeCG1(allocID, task))
	default:
		return filepath.Join(root, NomadCgroupParent, partition, scopeCG2(allocID, task))
	}
}
