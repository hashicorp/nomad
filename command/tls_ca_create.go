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

	// days is the number of days the CA will be valid for
	days int

	// constraint provides a spoecific name constraint to the CA which will then
	// reject any domains other than this.
	constraint bool

	// domain is used to provide a custom domain for the CA
	domain string

	// commonName is used to set a common name for the CA
	commonName string

	// additioanlNameConstraint provides a list of name constraints to the CA which will then
	// reject any domains other than these.
	additionalNameConstraint flags.StringFlag
}

func (c *TLSCACreateCommand) Help() string {
	helpText := `
Usage: nomad tls ca create [options]

Create a new Certificate Authority.

Here are some simple examples, more details can be found below or in 
the documentation.

Create a new Nomad CA

  $ nomad tls ca create
  ==> CA Certificate saved to: nomad-agent-ca.pem
  ==> CA Certificate key saved to: nomad-agent-ca-key.pem

  Create a new Nomad CA with a Custiom Domain

  $ nomad tls ca create -domain foo
  ==> CA Certificate saved to: foo-agent-ca.pem
  ==> CA Certificate key saved to: foo-agent-ca-key.pem


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

	flagSet := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flagSet.Usage = func() { c.Ui.Output(c.Help()) }
	flagSet.IntVar(&c.days, "days", 1825, "")
	flagSet.BoolVar(&c.constraint, "name-constraint", false, "")
	flagSet.StringVar(&c.domain, "domain", "nomad", "")
	flagSet.StringVar(&c.commonName, "common-name", "", "")
	flagSet.Var(&c.additionalNameConstraint, "additional-name-constraint", "")
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
	certFileName := fmt.Sprintf("%s-agent-ca.pem", c.domain)
	pkFileName := fmt.Sprintf("%s-agent-ca-key.pem", c.domain)

	if !(fileDoesNotExist(certFileName)) {
		c.Ui.Error(fmt.Sprintf("CA Certificate File '%s' already exists", certFileName))
		return 1
	}
	if !(fileDoesNotExist(pkFileName)) {
		c.Ui.Error(fmt.Sprintf("CA Key File '%s' already exists", pkFileName))
		return 1
	}

	constraints := []string{}
	if c.constraint {
		constraints = []string{c.domain, "localhost"}
		constraints = append(constraints, c.additionalNameConstraint...)
	}

	ca, pk, err := tlsutil.GenerateCA(tlsutil.CAOpts{Name: c.commonName, Days: c.days, Domain: c.domain, PermittedDNSDomains: constraints})
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
