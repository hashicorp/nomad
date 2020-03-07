package fingerprint

import (
	"fmt"
	"os"
	"strings"

	"github.com/containernetworking/cni/libcni"
	log "github.com/hashicorp/go-hclog"
)

type CNIFingerprint struct {
	StaticFingerprinter
	logger   log.Logger
	networks map[string]struct{}
}

func NewCNIFingerprint(logger log.Logger) Fingerprint {
	return &CNIFingerprint{logger: logger, networks: make(map[string]struct{})}
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

	for name := range networks {
		resp.AddAttribute(fmt.Sprintf("cni.network.%s", name), "1")
	}
	for name := range f.networks {
		if _, ok := networks[name]; !ok {
			resp.RemoveAttribute(fmt.Sprintf("cni.network.%s", name))
		}
	}
	f.networks = networks
	resp.Detected = true
	return nil
}

func (f *CNIFingerprint) Reload() {}
