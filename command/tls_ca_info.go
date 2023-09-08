// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/posener/complete"
	"github.com/ryanuber/columnize"
)

type TLSCAInfoCommand struct {
	Meta
}

func (c *TLSCAInfoCommand) Help() string {
	helpText := `
Usage: nomad tls ca info <CA file>

  Show information about a certificate authority.
`
	return strings.TrimSpace(helpText)
}

func (c *TLSCAInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *TLSCAInfoCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictOr(
		complete.PredictFiles("*.pem"),
	)
}

func (c *TLSCAInfoCommand) Synopsis() string {
	return "Show certificate authority information"
}

func (c *TLSCAInfoCommand) Name() string { return "tls cert info" }

func (c *TLSCAInfoCommand) Run(args []string) int {

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l < 0 || l > 1 {
		c.Ui.Error("This command takes up to one argument")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	var certFile []byte
	var err error
	var file string
	if len(args) == 0 {
		c.Ui.Error(fmt.Sprintf("Error reading CA file: %v", err))
		return 1
	}
	if len(args) == 1 {
		file = args[0]
		certFile, err = os.ReadFile(file)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error reading CA file: %v", err))
			return 1
		}
	}

	certInfo, err := tlsutil.ParseCert(string(certFile))
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}
	// Format the certificate info
	basic := []string{
		fmt.Sprintf("Serial Number|%s", certInfo.SerialNumber),
		fmt.Sprintf("Issuer CN|%s", certInfo.Issuer.CommonName),
		fmt.Sprintf("Common Name|%s", certInfo.Subject),
		fmt.Sprintf("Expiry Date|%s", certInfo.NotAfter),
		fmt.Sprintf("Permitted DNS Domains|%s", certInfo.PermittedDNSDomains),
	}

	// Print out the information
	c.Ui.Output(columnize.SimpleFormat(basic))
	return 0
}
