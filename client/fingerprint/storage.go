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

	volume, total, free, err := f.diskFree(storageDir)
	if err != nil {
		return fmt.Errorf("failed to determine disk space for %s: %v", storageDir, err)
	}

	if cfg.DiskTotalMB > 0 {
		total = uint64(cfg.DiskTotalMB) * bytesPerMegabyte
	}
	if cfg.DiskFreeMB > 0 {
		free = uint64(cfg.DiskFreeMB) * bytesPerMegabyte
	}

	if total < free {
		return fmt.Errorf("detected more free disk space (%d) than total disk space (%d), use disk_total_mb and disk_free_mb to correct", free, total)
	}

	resp.AddAttribute("unique.storage.volume", volume)
	resp.AddAttribute("unique.storage.bytestotal", strconv.FormatUint(total, 10))
	resp.AddAttribute("unique.storage.bytesfree", strconv.FormatUint(free, 10))

	// set the disk size for the response
	// COMPAT(0.10): Remove in 0.10
	resp.Resources = &structs.Resources{
		DiskMB: int(free / bytesPerMegabyte),
	}
	resp.NodeResources = &structs.NodeResources{
		Disk: structs.NodeDiskResources{
			DiskMB: int64(free / bytesPerMegabyte),
		},
	}
	resp.Detected = true

	return nil
}
