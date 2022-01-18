package resolvconf

import (
	"io"
	"os"
	"path/filepath"

	dresolvconf "github.com/docker/libnetwork/resolvconf"
	"github.com/docker/libnetwork/types"
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

	currRC, err := dresolvconf.Get()
	if err != nil {
		return nil, err
	}

	var (
		dnsList        = dresolvconf.GetNameservers(currRC.Content, types.IP)
		dnsSearchList  = dresolvconf.GetSearchDomains(currRC.Content)
		dnsOptionsList = dresolvconf.GetOptions(currRC.Content)
	)
	if nServers > 0 {
		dnsList = conf.Servers
	}
	if nSearches > 0 {
		dnsSearchList = conf.Searches
	}
	if nOptions > 0 {
		dnsOptionsList = conf.Options
	}

	_, err = dresolvconf.Build(path, dnsList, dnsSearchList, dnsOptionsList)
	if err != nil {
		return nil, err
	}

	return mount, nil
}

func copySystemDNS(dest string) error {
	in, err := os.Open(dresolvconf.Path())
	if err != nil {
		return err
	}
	defer in.Close()

	content, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	return os.WriteFile(dest, content, 0644)
}
