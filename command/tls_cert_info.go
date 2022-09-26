package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/posener/complete"
	"github.com/ryanuber/columnize"
)

type TLSCertInfoCommand struct {
	Meta
}

func (c *TLSCertInfoCommand) Help() string {
	helpText := `
Usage: nomad tls cert info [Certificate File]

Show certificate information

$ nomad tls cert info global-server-nomad.pem
Issuer CN     Nomad Agent CA 58896012363767591697986789371079092261
Common Name   CN=server.global.nomad
Expiry Date   2023-09-25 22:32:55 +0000 UTC
DNS Names     [server.global.nomad localhost]
IP Addresses  [127.0.0.1] 
`
	return strings.TrimSpace(helpText)
}

func (c *TLSCertInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *TLSCertInfoCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictOr(
		complete.PredictFiles("*.pem"),
	)
}

func (c *TLSCertInfoCommand) Synopsis() string {
	return "Show Certificate Information"
}

func (c *TLSCertInfoCommand) Name() string { return "tls cert info" }

func (c *TLSCertInfoCommand) Run(args []string) int {

	flagSet := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flagSet.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flagSet.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flagSet.Args()
	if l := len(args); l < 0 || l > 1 {
		c.Ui.Error("This command takes up to one argument")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	var certFile []byte
	var err error
	var file string

	if len(args) == 1 {
		file = args[0]
		certFile, err = os.ReadFile(file)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error reading Certifiate file: %v", err))
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
		fmt.Sprintf("Issuer CN|%s", certInfo.Issuer.CommonName),
		fmt.Sprintf("Common Name|%s", certInfo.Subject),
		fmt.Sprintf("Expiry Date|%s", certInfo.NotAfter),
		fmt.Sprintf("DNS Names|%s", certInfo.DNSNames),
		fmt.Sprintf("IP Addresses|%s", certInfo.IPAddresses),
	}

	// Print out the information
	c.Ui.Output(columnize.SimpleFormat(basic))
	return 0
}
