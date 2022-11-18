package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"

	"github.com/hashicorp/nomad/helper/flags"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/lib/file"
)

type TLSCACreateCommand struct {
	Meta

	// days is the number of days the CA will be valid for
	days int

	// constraint boolean enables the name constraint option in the CA which
	// will then reject any domains other than the ones stiputalted in -domain
	// and -addtitional-domain.
	constraint bool

	// domain is used to provide a custom domain for the CA
	domain string

	// commonName is used to set a common name for the CA
	commonName string

	// additionalDomain provides a list of restricted domains to the CA which
	// will then reject any domains other than these.
	additionalDomain flags.StringFlag
}

func (c *TLSCACreateCommand) Help() string {
	helpText := `
Usage: nomad tls ca create [options]

  Create a new certificate authority.

CA Create Options:

  -additional-domain
    Add additional DNS zones to the allowed list for the CA. The server will
    reject certificates for DNS names other than those specified in -domain and
    -additional-domain. This flag can be used multiple times. Only used in
    combination with -domain and -name-constraint.

  -common-name
    Common Name of CA. Defaults to "Nomad Agent CA".

  -days
    Provide number of days the CA is valid for from now on.
    Defaults to 5 years or 1825 days.

  -domain
    Domain of Nomad cluster. Only used in combination with -name-constraint.
    Defaults to "nomad".

  -name-constraint
    Enables the DNS name restriction functionality to the CA. Results in the CA
    rejecting certificates for any other DNS zone. If enabled, localhost and the
    value of -domain will be added to the allowed DNS zones field. If the UI is
    going to be served over HTTPS its hostname must be added with
    -additional-domain. Defaults to false.
`
	return strings.TrimSpace(helpText)
}

func (c *TLSCACreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-additional-domain": complete.PredictAnything,
			"-common-name":       complete.PredictAnything,
			"-days":              complete.PredictAnything,
			"-domain":            complete.PredictAnything,
			"-name-constraint":   complete.PredictAnything,
		})
}

func (c *TLSCACreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *TLSCACreateCommand) Synopsis() string {
	return "Create a certificate authority for Nomad"
}

func (c *TLSCACreateCommand) Name() string { return "tls ca create" }

func (c *TLSCACreateCommand) Run(args []string) int {

	flagSet := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flagSet.Usage = func() { c.Ui.Output(c.Help()) }
	flagSet.Var(&c.additionalDomain, "additional-domain", "")
	flagSet.IntVar(&c.days, "days", 1825, "")
	flagSet.BoolVar(&c.constraint, "name-constraint", false, "")
	flagSet.StringVar(&c.domain, "domain", "nomad", "")
	flagSet.StringVar(&c.commonName, "common-name", "", "")
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
	if c.domain != "" && c.domain != "nomad" && !c.constraint {
		c.Ui.Error("Please provide the -name-constraint flag to use a custom domain constraint")
		return 1
	}
	if c.domain == "nomad" && c.constraint {
		c.Ui.Error("Please provide the -domain flag if you want to enable custom domain constraints")
		return 1
	}
	if c.additionalDomain != nil && c.domain == "" && !c.constraint {
		c.Ui.Error("Please provide the -name-constraint flag to use a custom domain constraints")
		return 1
	}

	certFileName := fmt.Sprintf("%s-agent-ca.pem", c.domain)
	pkFileName := fmt.Sprintf("%s-agent-ca-key.pem", c.domain)

	if !(fileDoesNotExist(certFileName)) {
		c.Ui.Error(fmt.Sprintf("CA certificate file '%s' already exists", certFileName))
		return 1
	}
	if !(fileDoesNotExist(pkFileName)) {
		c.Ui.Error(fmt.Sprintf("CA key file '%s' already exists", pkFileName))
		return 1
	}

	constraints := []string{}
	if c.constraint {
		constraints = []string{c.domain, "localhost"}
		constraints = append(constraints, c.additionalDomain...)
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
	c.Ui.Output("==> CA certificate saved to: " + certFileName)

	if err := file.WriteAtomicWithPerms(pkFileName, []byte(pk), 0755, 0600); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}
	c.Ui.Output("==> CA certificate key saved to: " + pkFileName)

	return 0
}
