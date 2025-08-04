// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/nomad/helper/winsvc"
	"github.com/posener/complete"
)

type windowsInstallOpts struct {
	configDir, dataDir, installDir, binaryPath string
}

type WindowsServiceInstallCommand struct {
	Meta
	serviceManagerFn  func() (winsvc.WindowsServiceManager, error)
	expandPathFn      func(string) (string, error)
	privilegedCheckFn func() bool
}

func (c *WindowsServiceInstallCommand) Synopsis() string {
	return "Install the nomad Windows system service"
}

func (c *WindowsServiceInstallCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetDefault),
		complete.Flags{
			"-config-dir":  complete.PredictDirs("*"),
			"-data-dir":    complete.PredictDirs("*"),
			"-install-dir": complete.PredictDirs("*"),
		})
}

func (c *WindowsServiceInstallCommand) Name() string { return "windows service install" }

func (c *WindowsServiceInstallCommand) Help() string {
	helpText := `
Usage: nomad windows service install [options]

  This command installs nomad as a Windows system service.

General Options:

` + generalOptionsUsage(usageOptsDefault) + `

Service Install Options:

  -config-dir <dir>
    Directory to hold the nomad agent configuration.

  -data-dir <dir>
    Directory to hold the nomad agent state.

  -install-dir <dir>
    Directory to install the nomad binary.
`
	return strings.TrimSpace(helpText)
}

func (c *WindowsServiceInstallCommand) Run(args []string) int {
	opts := &windowsInstallOpts{}

	flags := c.Meta.FlagSet(c.Name(), FlagSetDefault)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&opts.configDir, "config-dir", "", "")
	flags.StringVar(&opts.dataDir, "data-dir", "", "")
	flags.StringVar(&opts.installDir, "install-dir", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	if args = flags.Args(); len(args) > 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Set helper functions to defaults if unset
	if c.expandPathFn == nil {
		c.expandPathFn = winsvc.ExpandPath
	}
	if c.serviceManagerFn == nil {
		c.serviceManagerFn = winsvc.NewWindowsServiceManager
	}
	if c.privilegedCheckFn == nil {
		c.privilegedCheckFn = winsvc.IsPrivilegedProcess
	}

	// Check that command is being run with elevated permissions
	if !c.privilegedCheckFn() {
		c.Ui.Error("Service install must be run with Administator privileges")
		return 1
	}

	c.Ui.Output("Installing nomad as a Windows service...")

	m, err := c.serviceManagerFn()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Could not connect to Windows service manager - %s", err))
		return 1
	}
	defer m.Close()

	if err := c.performInstall(m, opts); err != nil {
		c.Ui.Error(fmt.Sprintf("Service install failed: %s", err))
		return 1
	}

	c.Ui.Info("Successfully installed nomad Windows service")
	return 0
}

func (c *WindowsServiceInstallCommand) performInstall(m winsvc.WindowsServiceManager, opts *windowsInstallOpts) error {
	// Check if the nomad service has already been
	// registered. If so the service needs to be
	// stopped before proceeding with the install.
	exists, err := m.IsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME)
	if err != nil {
		return fmt.Errorf("unable to check for existing service - %w", err)
	}

	if exists {
		c.Ui.Output("  Stopping existing nomad service")
		nmdSvc, err := m.GetService(winsvc.WINDOWS_SERVICE_NAME)
		if err != nil {
			return fmt.Errorf("could not get existing service to stop - %w", err)
		}
		if err := nmdSvc.Stop(); err != nil {
			return fmt.Errorf("unable to stop existing service - %w", err)
		}
	}

	// Install the nomad binary into the system
	if err = c.binaryInstall(opts); err != nil {
		return fmt.Errorf("binary install failed - %w", err)
	}

	c.Ui.Output(fmt.Sprintf("  Nomad binary installed to: %s", opts.binaryPath))

	// Create a configuration directory and add
	// a basic configuration file if no configuration
	// currently exists
	if err = c.configInstall(opts); err != nil {
		return fmt.Errorf("configuration install failed - %w", err)
	}

	c.Ui.Output(fmt.Sprintf("  Nomad configuration directory: %s", opts.configDir))
	c.Ui.Output(fmt.Sprintf("  Nomad agent data directory: %s", opts.configDir))

	// Now let's install that service
	if err := c.serviceInstall(m, opts); err != nil {
		return fmt.Errorf("service install failed - %w", err)
	}

	return nil
}

func (c *WindowsServiceInstallCommand) serviceInstall(m winsvc.WindowsServiceManager, opts *windowsInstallOpts) error {
	var err error
	var srvc winsvc.WindowsService
	cmd := fmt.Sprintf("%s agent -config %s", opts.binaryPath, opts.configDir)

	exists, err := m.IsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME)
	if err != nil {
		return fmt.Errorf("service registration check failed - %w", err)
	}

	// If the service already exists, open it and update. Otherwise
	// create a new service.
	if exists {
		srvc, err = m.GetService(winsvc.WINDOWS_SERVICE_NAME)
		if err != nil {
			return fmt.Errorf("unable to get existing service - %w", err)
		}
		defer srvc.Close()
		if err := srvc.Configure(winsvc.WindowsServiceConfiguration{
			StartType:      winsvc.StartAutomatic,
			DisplayName:    winsvc.WINDOWS_SERVICE_DISPLAY_NAME,
			Description:    winsvc.WINDOWS_SERVICE_DESCRIPTION,
			BinaryPathName: cmd,
		}); err != nil {
			return fmt.Errorf("unable to configure service - %w", err)
		}
	} else {
		srvc, err = m.CreateService(winsvc.WINDOWS_SERVICE_NAME, opts.binaryPath,
			winsvc.WindowsServiceConfiguration{
				StartType:      winsvc.StartAutomatic,
				DisplayName:    winsvc.WINDOWS_SERVICE_DISPLAY_NAME,
				Description:    winsvc.WINDOWS_SERVICE_DESCRIPTION,
				BinaryPathName: cmd,
			},
		)
		if err != nil {
			return fmt.Errorf("unable to create service - %w", err)
		}
		defer srvc.Close()
	}

	// Enable the service in the Windows eventlog
	if err := srvc.EnableEventlog(); err != nil {
		return fmt.Errorf("could not configure eventlog - %w", err)
	}

	// Ensure the service is stopped
	if err := srvc.Stop(); err != nil {
		return fmt.Errorf("could not stop service - %w", err)
	}

	// Start the service so the new binary is in use
	if err := srvc.Start(); err != nil {
		return fmt.Errorf("could not start service - %w", err)
	}

	return nil
}

func (c *WindowsServiceInstallCommand) configInstall(opts *windowsInstallOpts) error {
	// If the config or data directory are unset, default them
	if opts.configDir == "" {
		opts.configDir = filepath.Join(winsvc.WINDOWS_INSTALL_APPDATA_DIRECTORY, "config")
	}
	if opts.dataDir == "" {
		opts.dataDir = filepath.Join(winsvc.WINDOWS_INSTALL_APPDATA_DIRECTORY, "data")
	}

	var err error
	if opts.configDir, err = c.expandPathFn(opts.configDir); err != nil {
		return fmt.Errorf("cannot generate configuration path - %s", err)
	}
	if opts.dataDir, err = c.expandPathFn(opts.dataDir); err != nil {
		return fmt.Errorf("cannot generate data path - %s", err)
	}

	// Ensure directories exist
	if err = os.MkdirAll(opts.configDir, 0o755); err != nil {
		return fmt.Errorf("cannot create configuration directory - %s", err)
	}

	if err = os.MkdirAll(opts.dataDir, 0o755); err != nil {
		return fmt.Errorf("cannot create data directory - %s", err)
	}

	// Check if any configuration files exist
	matches, _ := filepath.Glob(filepath.Join(opts.configDir, "*"))
	if len(matches) < 1 {
		f, err := os.Create(filepath.Join(opts.configDir, "config.hcl"))
		if err != nil {
			return fmt.Errorf("could not create default configuration file - %s", err)
		}
		fmt.Fprintf(f, strings.TrimSpace(`
# Full configuration options can be found at https://developer.hashicorp.com/nomad/docs/configuration

data_dir  = "%s"
bind_addr = "0.0.0.0"

server {
  # license_path is required for Nomad Enterprise as of Nomad v1.1.1+
  #license_path = "%s\license.hclic"
  enabled          = true
  bootstrap_expect = 1
}

client {
  enabled = true
  servers = ["127.0.0.1"]
}

log_level = "WARN"
eventlog {
  enabled = true
  level   = "ERROR"
}
`), strings.ReplaceAll(opts.dataDir, `\`, `\\`), strings.ReplaceAll(opts.configDir, `\`, `\\`))
		f.Close()
		c.Ui.Output(fmt.Sprintf("  Added initial configuration file: %s", f.Name()))
	}

	return nil
}

func (c *WindowsServiceInstallCommand) binaryInstall(opts *windowsInstallOpts) error {
	// Get the path to the currently executing nomad. This
	// will be installed for the service to call.
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot detect current nomad path - %s", err)
	}

	// Build the needed paths
	if opts.installDir == "" {
		opts.installDir = winsvc.WINDOWS_INSTALL_BIN_DIRECTORY
	}
	opts.installDir, err = c.expandPathFn(opts.installDir)
	if err != nil {
		return fmt.Errorf("cannot generate binary install path - %s", err)
	}
	opts.binaryPath = filepath.Join(opts.installDir, "nomad.exe")

	// Ensure the install directory exists
	if err = os.MkdirAll(opts.installDir, 0o755); err != nil {
		return fmt.Errorf("could not create binary install directory - %s", err)
	}

	// Create a new copy of the current binary to install
	exeFile, err := os.Open(exePath)
	if err != nil {
		return fmt.Errorf("cannot open current nomad path for install - %s", err)
	}
	defer exeFile.Close()

	// Copy into a temporary file which can then be moved
	// into the correct location.
	dstFile, err := os.CreateTemp(os.TempDir(), "nomad*")
	if err != nil {
		return fmt.Errorf("cannot create copy - %s", err)
	}
	defer dstFile.Close()

	if err = dstFile.Chmod(0o755); err != nil {
		return fmt.Errorf("cannot update install path permissions - %s", err)
	}

	if _, err = io.Copy(dstFile, exeFile); err != nil {
		return fmt.Errorf("cannot write nomad binary for install - %s", err)
	}
	dstFile.Close()

	// With a copy ready to be moved into place, ensure that
	// the path is clear then move the file.
	if err = os.RemoveAll(opts.binaryPath); err != nil {
		return fmt.Errorf("cannot remove existing nomad binary install - %s", err)
	}

	if err = os.Rename(dstFile.Name(), opts.binaryPath); err != nil {
		return fmt.Errorf("cannot install new nomad binary - %s", err)
	}

	return nil
}
