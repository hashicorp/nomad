// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package resolvconf

import (
	"bytes"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/hashicorp/nomad/lib/resolvconf"
	"github.com/hashicorp/nomad/plugins/drivers"
)

func GenerateDNSMount(taskDir string, conf *drivers.DNSConfig) (*drivers.MountConfig, error) {
	var nSearches, nServers, nOptions int
	mount := &drivers.MountConfig{
		TaskPath:        "/etc/resolv.conf",
		HostPath:        filepath.Join(taskDir, "resolv.conf"),
		Readonly:        true,
		PropagationMode: "private",
	}
	if conf != nil {
		nServers = len(conf.Servers)
		nSearches = len(conf.Searches)
		nOptions = len(conf.Options)
	}

	root, err := os.OpenRoot(taskDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open task dir: %w", err)
	}
	defer root.Close()

	// Use system dns if no configuration is given
	if nServers == 0 && nSearches == 0 && nOptions == 0 {
		if err := copySystemDNS(root); err != nil {
			return nil, err
		}
		return mount, nil
	}

	currRC, err := resolvconf.Get()
	if err != nil {
		return nil, err
	}

	var (
		dnsList        = resolvconf.GetNameservers(currRC.Content, resolvconf.IP)
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

	content, err := buildResolvConf(dnsList, dnsSearchList, dnsOptionsList)
	if err != nil {
		return nil, err
	}

	if err := writeToRoot(root, "resolv.conf", content); err != nil {
		return nil, err
	}

	return mount, nil
}

// buildResolvConf generates resolv.conf content from the given DNS parameters
// without writing to disk, mirroring the logic in resolvconf.Build.
func buildResolvConf(nameservers, dnsSearch, dnsOptions []string) ([]byte, error) {
	var ns []netip.Addr
	for _, addr := range nameservers {
		ipAddr, err := netip.ParseAddr(addr)
		if err != nil {
			return nil, fmt.Errorf("bad nameserver address: %w", err)
		}
		ns = append(ns, ipAddr)
	}
	rc := resolvconf.ResolvConf{}
	rc.OverrideNameServers(ns)
	rc.OverrideSearch(dnsSearch)
	rc.OverrideOptions(dnsOptions)
	return rc.Generate(false)
}

func copySystemDNS(root *os.Root) error {
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

	return writeToRoot(root, "resolv.conf", content)
}

func writeToRoot(root *os.Root, name string, content []byte) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, bytes.NewReader(content))
	return err
}
