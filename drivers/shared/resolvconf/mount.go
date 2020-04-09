package resolvconf

import (
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/nomad/plugins/drivers"
)

func GenerateDNSMount(taskDir string, conf *drivers.DNSConfig) (*drivers.MountConfig, error) {
	var nSearches, nServers, nOptions int
	path := filepath.Join(taskDir, "resolv.conf")
	if conf != nil {
		nServers := len(conf.Servers)
		nSearches := len(conf.Searches)
		nOptions := len(conf.Options)
	}

	// Use system dns if no configuration is given
	if nServers == 0 && nSearches == 0 && nOptions == 0 {
		if err := copySystemDNS(path); err != nil {
			return nil, err
		}

		return &drivers.MountConfig{
			TaskPath:        "/etc/resolv.conf",
			HostPath:        dest,
			Readonly:        true,
			PropagationMode: "private",
		}, nil
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rc, err := New(conf.Servers, conf.Searches, conf.Options)
	if err != nil {
		return nil, err
	}

	if _, err := f.Write(rc.Content()); err != nil {
		return nil, err
	}

	return &drivers.MountConfig{
		TaskPath:        "/etc/resolv.conf",
		HostPath:        path,
		Readonly:        true,
		PropagationMode: "private",
	}, nil
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
