package fingerprint

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

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

func (f *StorageFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {

	// Initialize these to empty defaults
	node.Attributes["storage.volume"] = ""
	node.Attributes["storage.bytestotal"] = ""
	node.Attributes["storage.bytesfree"] = ""
	if node.Resources == nil {
		node.Resources = &structs.Resources{}
	}

	// Guard against unset AllocDir
	storageDir := cfg.AllocDir
	if storageDir == "" {
		var err error
		storageDir, err = os.Getwd()
		if err != nil {
			return false, fmt.Errorf("Unable to get CWD from filesystem: %s", err)
		}
	}

	volume, total, free, err := f.diskFree(storageDir)
	if err != nil {
		return false, err
	}

	node.Attributes["storage.volume"] = volume
	node.Attributes["storage.bytestotal"] = strconv.FormatUint(total, 10)
	node.Attributes["storage.bytesfree"] = strconv.FormatUint(free, 10)

	const mb = 1024 * 1024
	node.Resources.DiskMB = int(free / mb)

	return true, nil
}
