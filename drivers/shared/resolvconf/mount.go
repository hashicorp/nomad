// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resolvconf

import (
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/libnetwork/resolvconf"
	"github.com/docker/docker/libnetwork/types"
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

	currRC, err := resolvconf.Get()
	if err != nil {
		return nil, err
	}

	var (
		dnsList        = resolvconf.GetNameservers(currRC.Content, types.IP)
		dnsSearchList  = resolvconf.GetSearchDomains(currRC.Content)
		dnsOptionsList = resolvconf.GetOptions(currRC.Content)
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

	_, err = resolvconf.Build(path, dnsList, dnsSearchList, dnsOptionsList)
	if err != nil {
		return nil, err
	}

	return mount, nil
}

func copySystemDNS(filePath string) error {
	in, err := os.Open(resolvconf.Path())
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()

	content, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, content, 0644)
}
