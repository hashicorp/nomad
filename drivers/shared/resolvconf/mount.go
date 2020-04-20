package resolvconf

import (
	"io"
	"os"
	"path/filepath"

	dresolvconf "github.com/docker/libnetwork/resolvconf"
	"github.com/hashicorp/nomad/plugins/drivers"
)

func GenerateDNSMount(taskDir string, conf *drivers.DNSConfig) (*drivers.MountConfig, error) {
	var nSearches, nServers, nOptions int
	path := filepath.Join(taskDir, "resolv.conf")
	mount := &drivers.MountConfig{
		TaskPath:        "/etc/resolv.conf",
		HostPath:        path,
		Readonly:        true,
		PropagationMode: "private",
	}
	if conf != nil {
		nServers = len(conf.Servers)
		nSearches = len(conf.Searches)
		nOptions = len(conf.Options)
	}

	// Use system dns if no configuration is given
	if nServers == 0 && nSearches == 0 && nOptions == 0 {
		if err := copySystemDNS(path); err != nil {
			return nil, err
		}

		return mount, nil
	}

	_, err := dresolvconf.Build(path, conf.Servers, conf.Searches, conf.Options)
	if err != nil {
		return nil, err
	}

	return mount, nil
}

func copySystemDNS(dest string) error {
	in, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
