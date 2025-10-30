// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"os"
	"strconv"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

const bytesPerMegabyte = 1024 * 1024

// StorageFingerprint is used to measure the amount of storage free for
// applications that the Nomad agent will run on this machine.
type StorageFingerprint struct {
	StaticFingerprinter
	logger log.Logger
}

func NewStorageFingerprint(logger log.Logger) Fingerprint {
	fp := &StorageFingerprint{logger: logger.Named("storage")}
	return fp
}

func (f *StorageFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	cfg := req.Config

	// Guard against unset AllocDir
	storageDir := cfg.AllocDir
	if storageDir == "" {
		var err error
		storageDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("unable to get CWD from filesystem: %s", err)
		}
	}

	volume, total, err := f.diskInfo(storageDir)
	if err != nil {
		return fmt.Errorf("failed to determine disk space for %s: %v", storageDir, err)
	}

	if cfg.DiskTotalMB > 0 {
		total = uint64(cfg.DiskTotalMB) * bytesPerMegabyte
	}

	free := total - f.reservedDisk(req)

	// DEPRECATED: remove in 1.13.0
	if cfg.DiskFreeMB > 0 {
		free = uint64(cfg.DiskFreeMB) * bytesPerMegabyte
	}

	resp.AddAttribute("unique.storage.volume", volume)
	resp.AddAttribute("unique.storage.bytestotal", strconv.FormatUint(total, 10))
	resp.AddAttribute("unique.storage.bytesfree", strconv.FormatUint(free, 10))

	// set the disk size for the response
	resp.NodeResources = &structs.NodeResources{
		Disk: structs.NodeDiskResources{
			DiskMB: int64(total / bytesPerMegabyte),
		},
	}
	resp.Detected = true

	return nil
}

func (f *StorageFingerprint) reservedDisk(req *FingerprintRequest) uint64 {
	switch {
	case req.Config.Node == nil:
		return 0
	case req.Config.Node.ReservedResources == nil:
		return 0
	default:
		return uint64(req.Config.Node.ReservedResources.Disk.DiskMB)
	}
}
