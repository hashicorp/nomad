// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"fmt"
	"strings"
	"slices"
	"bytes"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
)

const (
	// the name of the cpuset interface file
	cpusetFile = "cpuset.cpus"

	// the name of the cpuset mems interface file
	memsFile = "cpuset.mems"
)

// Init will initialize the cgroup tree that the Nomad client will use for
// isolating resources of tasks. cores is the cpuset granted for use by Nomad.
func Init(log hclog.Logger, cores string) {
	log.Info("initializing nomad cgroups", "cores", cores)

	switch GetMode() {
	case CG1:

		// the value to disable inheriting values from parent cgroup
		const noClone = "0"

		// the name of the clone_children interface file
		const cloneFile = "cgroup.clone_children"

		// create the /nomad cgroup (or whatever the name is configured to be)
		// for each cgroup controller we are going to use
		controllers := []string{"freezer", "memory", "cpu", "cpuset"}
		for _, ctrl := range controllers {
			p := filepath.Join(root, ctrl, NomadCgroupParent)
			if err := os.MkdirAll(p, 0755); err != nil {
				log.Error("failed to create nomad cgroup", "controller", ctrl, "error", err)
				return
			}
		}

		// determine the memset that will be set on the cgroup for each task
		//
		// nominally this will be all available but we have to read the root
		// cgroup to actually know what those are
		//
		// additionally if the nomad cgroup parent already exists, we must
		// use that memset instead, because it could have been setup out of
		// band from nomad itself
		var memsSet string
		if mems, err := detectMemsCG1(); err != nil {
			log.Error("failed to detect memset", "error", err)
			return
		} else {
			memsSet = mems
		}

		//
		// configure cpuset partitioning
		//
		// the tree is lopsided - tasks making use of reserved cpu cores get
		// their own cgroup with a static cpuset.cpus value. other tasks are
		// placed in the single share cgroup and share its dynamic cpuset.cpus
		// value
		//
		// e.g.,
		//  root/cpuset/nomad/
		//    share/{cgroup.procs, cpuset.cpus, cpuset.mems}
		//    reserve/
		//      abc123.task/{cgroup.procs, cpuset.cpus, cpuset.mems}
		//      def456.task/{cgroup.procs, cpuset.cpus, cpuset.mems}

		if err := writeCG(noClone, "cpuset", NomadCgroupParent, cloneFile); err != nil {
			log.Error("failed to set clone_children on nomad cpuset cgroup", "error", err)
			return
		}

		if err := writeCG(memsSet, "cpuset", NomadCgroupParent, memsFile); err != nil {
			log.Error("failed to set cpuset.mems on nomad cpuset cgroup", "error", err)
			return
		}

		if err := writeCG(cores, "cpuset", NomadCgroupParent, cpusetFile); err != nil {
			log.Error("failed to write cores to nomad cpuset cgroup", "error", err)
			return
		}

		//
		// share partition
		//

		if err := mkCG("cpuset", NomadCgroupParent, SharePartition()); err != nil {
			log.Error("failed to create share cpuset partition", "error", err)
			return
		}

		if err := writeCG(noClone, "cpuset", NomadCgroupParent, SharePartition(), cloneFile); err != nil {
			log.Error("failed to set clone_children on nomad cpuset cgroup", "error", err)
			return
		}

		if err := writeCG(memsSet, "cpuset", NomadCgroupParent, SharePartition(), memsFile); err != nil {
			log.Error("failed to set cpuset.mems on share cpuset partition", "error", err)
			return
		}

		//
		// reserve partition
		//

		if err := mkCG("cpuset", NomadCgroupParent, ReservePartition()); err != nil {
			log.Error("failed to create reserve cpuset partition", "error", err)
			return
		}

		if err := writeCG(noClone, "cpuset", NomadCgroupParent, ReservePartition(), cloneFile); err != nil {
			log.Error("failed to set clone_children on nomad cpuset cgroup", "error", err)
			return
		}

		if err := writeCG(memsSet, "cpuset", NomadCgroupParent, ReservePartition(), memsFile); err != nil {
			log.Error("failed to set cpuset.mems on reserve cpuset partition", "error", err)
			return
		}

		log.Debug("nomad cpuset partitions initialized", "cores", cores)

	case CG2:
		// the cgroup controllers we need to activate at the root and on the nomad slice
		controllers := []string{"cpuset", "cpu", "io", "memory", "pids"}

		// the name of the cgroup subtree interface file
		const subtreeFile = "cgroup.subtree_control"

		//
		// configuring root cgroup (/sys/fs/cgroup)
		//

		subtreeRootPath := filepath.Join(root, subtreeFile)
		content, _ := os.ReadFile(subtreeRootPath)
		rootSubtreeControllers := strings.Split(strings.TrimSpace(string(content)), " ")

		for _, controller := range controllers {
			if !slices.Contains(rootSubtreeControllers, controller) {
				log.Error("controller not enabled in your system, check kernel build configuration and commandline (/proc/cmdline)", "controller", controller)
			}
		}

		for _, controller := range controllers {
			if err := writeCG(fmt.Sprintf("+%s", controller), subtreeFile); err != nil {
				log.Error("failed to enable cgroup controller", "error", err, "controller", controller)
				return
			}
		}

		//
		// configuring nomad.slice
		//

		if err := mkCG(NomadCgroupParent); err != nil {
			log.Error("failed to create nomad cgroup", "error", err)
			return
		}

		for _, controller := range controllers {
			if err := writeCG(fmt.Sprintf("+%s", controller), NomadCgroupParent, subtreeFile); err != nil {
				log.Error("failed to enable controller on nomad cgroup", "error", err, "controller", controller)
				return
			}
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

		for _, controller := range controllers {
				if err := writeCG(fmt.Sprintf("+%s", controller), NomadCgroupParent, SharePartition(), subtreeFile); err != nil {
						log.Error("failed to set subtree control on share partition", "error", err)
						return
				}
		}

		log.Debug("partition member nomad.slice/share cgroup initialized")

		//
		// configuring nomad.slice/reserve (member)
		//

		if err := mkCG(NomadCgroupParent, ReservePartition()); err != nil {
			log.Error("failed to create share cgroup", "error", err)
			return
		}

		for _, controller := range controllers {
				if err := writeCG(fmt.Sprintf("+%s", controller), NomadCgroupParent, ReservePartition(), subtreeFile); err != nil {
						log.Error("failed to set subtree control on reserve partition", "error", err)
						return
				}
		}

		log.Debug("partition member nomad.slice/reserve cgroup initialized")
	}
}

// detectMemsCG1 will determine the cpuset.mems value to use for
// Nomad managed cgroups.
//
// Copy the value from the root cgroup cpuset.mems file, unless the nomad
// parent cgroup exists with a value set, in which case use the cpuset.mems
// value from there.
func detectMemsCG1() (string, error) {
	// read root cgroup mems file
	memsRootPath := filepath.Join(root, "cpuset", memsFile)
	b, err := os.ReadFile(memsRootPath)
	if err != nil {
		return "", err
	}
	memsFromRoot := string(bytes.TrimSpace(b))

	// read parent cgroup mems file (may not exist)
	memsParentPath := filepath.Join(root, "cpuset", NomadCgroupParent, memsFile)
	b2, err2 := os.ReadFile(memsParentPath)
	if err2 != nil {
		return memsFromRoot, nil
	}
	memsFromParent := string(bytes.TrimSpace(b2))

	// we found a value in the parent cgroup file, use that
	if memsFromParent != "" {
		return memsFromParent, nil
	}

	// otherwise use the value from the root cgroup
	return memsFromRoot, nil
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

// PathCG1 returns the filepath to the cgroup directory of the given interface
// and allocID / taskName.
func PathCG1(allocID, taskName, iface string) string {
	return filepath.Join(root, iface, NomadCgroupParent, ScopeCG1(allocID, taskName))
}

// LinuxResourcesPath returns the filepath to the directory that the field
// x.Resources.LinuxResources.CpusetCgroupPath is expected to hold on to
func LinuxResourcesPath(allocID, task string, reserveCores bool) string {
	partition := GetPartitionFromBool(reserveCores)
	mode := GetMode()
	switch {
	case mode == CG1 && reserveCores:
		return filepath.Join(root, "cpuset", NomadCgroupParent, partition, ScopeCG1(allocID, task))
	case mode == CG1 && !reserveCores:
		return filepath.Join(root, "cpuset", NomadCgroupParent, partition)
	default:
		return filepath.Join(root, NomadCgroupParent, partition, scopeCG2(allocID, task))
	}
}
