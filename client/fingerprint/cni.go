package fingerprint

import (
	"fmt"
	"os"
	"strings"

	"github.com/containernetworking/cni/libcni"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

type CNIFingerprint struct {
	StaticFingerprinter
	logger log.Logger
}

func NewCNIFingerprint(logger log.Logger) Fingerprint {
	return &CNIFingerprint{logger: logger}
}

func (f *CNIFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	confDir := req.Config.CNIConfigDir
	networks := map[string]struct{}{}
	if _, err := os.Stat(confDir); os.IsNotExist(err) {
		f.logger.Debug("CNI config dir is not set or does not exist, skipping", "cni_config_dir", confDir)
		resp.Detected = false
		return nil
	}

	files, err := libcni.ConfFiles(confDir, []string{".conf", ".conflist", ".json"})
	if err != nil {
		return fmt.Errorf("failed to read cni conf files: %v", err)
	}

	for _, confFile := range files {
		if strings.HasSuffix(confFile, ".conflist") {
			confList, err := libcni.ConfListFromFile(confFile)
			if err != nil {
				return fmt.Errorf("failed to load CNI config list file %s: %v", confFile, err)
			}
			if _, ok := networks[confList.Name]; ok {
				f.logger.Warn("multiple CNI config names found, ignoring file", "name", confList.Name, "file", confFile)
				continue
			}
			networks[confList.Name] = struct{}{}
		} else {
			conf, err := libcni.ConfFromFile(confFile)
			if err != nil {
				return fmt.Errorf("failed to load CNI config file %s: %v", confFile, err)
			}
			if _, ok := networks[conf.Network.Name]; ok {
				f.logger.Warn("multiple CNI config names found, ignoring file", "name", conf.Network.Name, "file", confFile)
				continue
			}
			networks[conf.Network.Name] = struct{}{}
		}
	}

	var nodeNetworks structs.Networks

	for name := range networks {
		nodeNetworks = append(nodeNetworks, &structs.NetworkResource{
			Mode: fmt.Sprintf("cni/%s", name),
		})
		f.logger.Debug("detected CNI network", "name", name)
	}

	resp.NodeResources = &structs.NodeResources{
		Networks: nodeNetworks,
	}

	resp.Detected = true
	return nil
}
