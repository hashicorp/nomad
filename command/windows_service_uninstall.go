// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/helper/winsvc"
	"github.com/posener/complete"
)

type WindowsServiceUninstallCommand struct {
	Meta
	serviceManagerFn  func() (winsvc.WindowsServiceManager, error)
	privilegedCheckFn func() bool
}

func (c *WindowsServiceUninstallCommand) AutoCompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetDefault)
}

func (c *WindowsServiceUninstallCommand) Synopsis() string {
	return "Uninstall the nomad Windows system service"
}

func (c *WindowsServiceUninstallCommand) Name() string { return "windows service uninstall" }

func (c *WindowsServiceUninstallCommand) Help() string {
	helpText := `
Usage: nomad windows service uninstall [options]

  This command uninstalls nomad as a Windows system service.

General Options:

` + generalOptionsUsage(usageOptsDefault)
	return strings.TrimSpace(helpText)
}

func (c *WindowsServiceUninstallCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetDefault)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	if args = flags.Args(); len(args) > 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Set helper functions to default if unset
	if c.serviceManagerFn == nil {
		c.serviceManagerFn = winsvc.NewWindowsServiceManager
	}
	if c.privilegedCheckFn == nil {
		c.privilegedCheckFn = winsvc.IsPrivilegedProcess
	}

	// Check that command is being run with elevated permissions
	if !c.privilegedCheckFn() {
		c.Ui.Error("Service uninstall must be run with Administator privileges")
		return 1
	}

	c.Ui.Output("Uninstalling nomad Windows service...")

	m, err := c.serviceManagerFn()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Could not connect to Windows service manager - %s", err))
		return 1
	}
	defer m.Close()

	if err := c.performUninstall(m); err != nil {
		c.Ui.Error(fmt.Sprintf("Service uninstall failed: %s", err))
		return 1
	}

	c.Ui.Info("Successfully uninstalled nomad Windows service")
	return 0
}

func (c *WindowsServiceUninstallCommand) performUninstall(m winsvc.WindowsServiceManager) error {
	// Check that the nomad service is currently registered
	exists, err := m.IsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME)
	if err != nil {
		return fmt.Errorf("unable to check for existing service - %w", err)
	}

	if !exists {
		return nil
	}

	// Grab the service and ensure the service is stopped
	srvc, err := m.GetService(winsvc.WINDOWS_SERVICE_NAME)
	if err != nil {
		return fmt.Errorf("could not get existing service - %w", err)
	}
	defer srvc.Close()

	if err := srvc.Stop(); err != nil {
		return fmt.Errorf("unable to stop service - %w", err)
	}

	// Remove the service from the event log
	if err := srvc.DisableEventlog(); err != nil {
		return fmt.Errorf("could not remove eventlog configuration - %w", err)
	}

	// Finally, delete the service
	if err := srvc.Delete(); err != nil {
		return fmt.Errorf("could not delete service - %w", err)
	}

	return nil
}
