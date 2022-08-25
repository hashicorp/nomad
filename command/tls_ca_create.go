package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/helper/flags"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/lib/file"
	"github.com/posener/complete"
)

type TLSCACreateCommand struct {
	Meta
}

func (c *TLSCACreateCommand) Help() string {
	helpText := `
Usage: nomad tls ca create [options]

This command has subcommands for interacting with Certificate Authorities.

Here are some simple examples, and more detailed examples are available
in the subcommands or the documentation.

Create a new Nomad CA

  $ nomad tls ca create
  ==> CA Certificate saved to: nomad-agent-ca.pem
  ==> CA Certificate key saved to: nomad-agent-ca-key.pem


CA Create Options:
  -additional-name-constraint
    Add name constraints for the CA. Results in rejecting certificates
    for other DNS than specified. Can be used multiple times. Only used 
    in combination with -name-constraint.

  -common-name
    Common Name of CA. Defaults to Nomad Agent CA..

  -days
    Provide number of days the CA is valid for from now on.
    Defaults to 5 years or 1825 days.

  -domain
    Domain of nomad cluster. Only used in combination with -name-constraint. 
    Defaults to nomad.

  -name-constraint
    Add name constraints for the CA. Results in rejecting
    certificates for other DNS than specified. If turned on localhost and 
    -domain will be added to the allowed DNS. If the UI is going to be served 
    over HTTPS its DNS has to be added with -additional-constraint. It is not
    possible to add that after the fact! Defaults to false.

`
	return strings.TrimSpace(helpText)
}

func (c *TLSCACreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-additional-name-constraint": complete.PredictAnything,
			"-common-name":                complete.PredictAnything,
			"-days":                       complete.PredictAnything,
			"-domain":                     complete.PredictAnything,
			"-name-constraint":            complete.PredictAnything,
		})
}

func (c *TLSCACreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *TLSCACreateCommand) Synopsis() string {
	return "Create a Certificate Authority for Nomad"
}

func (c *TLSCACreateCommand) Name() string { return "tls ca create" }

func (c *TLSCACreateCommand) Run(args []string) int {

	var (
		days                     int
		constraint               bool
		domain                   string
		commonName               string
		additionalNameConstraint flags.StringFlag
	)

	flagSet := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flagSet.Usage = func() { c.Ui.Output(c.Help()) }
	flagSet.IntVar(&days, "days", 1825, "")
	flagSet.BoolVar(&constraint, "name-constraint", false, "")
	flagSet.StringVar(&domain, "domain", "nomad", "")
	flagSet.StringVar(&commonName, "common-name", "", "")
	flagSet.Var(&additionalNameConstraint, "additional-name-constraint", "")
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
	certFileName := fmt.Sprintf("%s-agent-ca.pem", domain)
	pkFileName := fmt.Sprintf("%s-agent-ca-key.pem", domain)

	if !(fileDoesNotExist(certFileName)) {
		c.Ui.Error(fmt.Sprintf("CA Certificate File '%s' already exists", certFileName))
		return 1
	}
	if !(fileDoesNotExist(pkFileName)) {
		c.Ui.Error(fmt.Sprintf("CA Key File '%s' already exists", pkFileName))
		return 1
	}

	constraints := []string{}
	if constraint {
		constraints = []string{domain, "localhost"}
		constraints = append(constraints, additionalNameConstraint...)
	}

	ca, pk, err := tlsutil.GenerateCA(tlsutil.CAOpts{Name: commonName, Days: days, Domain: domain, PermittedDNSDomains: constraints})
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	if err := file.WriteAtomicWithPerms(certFileName, []byte(ca), 0755, 0666); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}
	c.Ui.Output("==> CA Certificate saved to: " + certFileName)

	if err := file.WriteAtomicWithPerms(pkFileName, []byte(pk), 0755, 0600); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}
	c.Ui.Output("==> CA Certificate key saved to: " + pkFileName)

	return 0
}
