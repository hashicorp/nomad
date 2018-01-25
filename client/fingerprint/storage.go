package fingerprint

import (
	"fmt"
	"log"
	"os"
	"strconv"

	cstructs "github.com/hashicorp/nomad/client/structs"
)

const bytesPerMegabyte = 1024 * 1024

// StorageFingerprint is used to measure the amount of storage free for
// applications that the Nomad agent will run on this machine.
type StorageFingerprint struct {
	StaticFingerprinter
	logger *log.Logger
}

func NewStorageFingerprint(logger *log.Logger) Fingerprint {
	fp := &StorageFingerprint{logger: logger}
	return fp
}

func (f *StorageFingerprint) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	cfg := req.Config

	// Initialize these to empty defaults
	resp.Attributes["unique.storage.volume"] = ""
	resp.Attributes["unique.storage.bytestotal"] = ""
	resp.Attributes["unique.storage.bytesfree"] = ""

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

	resp.Attributes["unique.storage.volume"] = volume
	resp.Attributes["unique.storage.bytestotal"] = strconv.FormatUint(total, 10)
	resp.Attributes["unique.storage.bytesfree"] = strconv.FormatUint(free, 10)

	resp.Resources.DiskMB = int(free / bytesPerMegabyte)

	return nil
}
