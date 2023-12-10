// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

	// country is used to set a country code for the CA
	country string

	// postalCode is used to set a postal code for the CA
	postalCode string

	// province is used to set a province for the CA
	province string

	// locality is used to set a locality for the CA
	locality string

	// streetAddress is used to set a street address for the CA
	streetAddress string

	// organization is used to set an organization for the CA
	organization string

	// organizationalUnit is used to set an organizational unit for the CA
	organizationalUnit string
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

  -country
    Country of the CA. Defaults to "US".

  -days
    Provide number of days the CA is valid for from now on.
    Defaults to 5 years or 1825 days.

  -domain
    Domain of Nomad cluster. Only used in combination with -name-constraint.
    Defaults to "nomad".

  -locality
    Locality of the CA. Defaults to "San Francisco".

  -name-constraint
    Enables the DNS name restriction functionality to the CA. Results in the CA
    rejecting certificates for any other DNS zone. If enabled, localhost and the
    value of -domain will be added to the allowed DNS zones field. If the UI is
    going to be served over HTTPS its hostname must be added with
    -additional-domain. Defaults to false.

  -organization
    Organization of the CA. Defaults to "HashiCorp Inc.".

  -organizational-unit
    Organizational Unit of the CA. Defaults to "Nomad".

  -postal-code
    Postal Code of the CA. Defaults to "94105".

  -province
    Province of the CA. Defaults to "CA".

  -street-address
    Street Address of the CA. Defaults to "101 Second Street".

`
	return strings.TrimSpace(helpText)
}

func (c *TLSCACreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-additional-domain":   complete.PredictAnything,
			"-common-name":         complete.PredictAnything,
			"-days":                complete.PredictAnything,
			"-country":             complete.PredictAnything,
			"-domain":              complete.PredictAnything,
			"-locality":            complete.PredictAnything,
			"-name-constraint":     complete.PredictAnything,
			"-organization":        complete.PredictAnything,
			"-organizational-unit": complete.PredictAnything,
			"-postal-code":         complete.PredictAnything,
			"-province":            complete.PredictAnything,
			"-street-address":      complete.PredictAnything,
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
	flagSet.IntVar(&c.days, "days", 0, "")
	flagSet.BoolVar(&c.constraint, "name-constraint", false, "")
	flagSet.StringVar(&c.domain, "domain", "", "")
	flagSet.StringVar(&c.commonName, "common-name", "", "")
	flagSet.StringVar(&c.country, "country", "", "")
	flagSet.StringVar(&c.postalCode, "postal-code", "", "")
	flagSet.StringVar(&c.province, "province", "", "")
	flagSet.StringVar(&c.locality, "locality", "", "")
	flagSet.StringVar(&c.streetAddress, "street-address", "", "")
	flagSet.StringVar(&c.organization, "organization", "", "")
	flagSet.StringVar(&c.organizationalUnit, "organizational-unit", "", "")
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
	if c.IsCustom() && c.days != 0 || c.IsCustom() {
		c.domain = "nomad"
	} else {
		if c.commonName == "" {
			c.Ui.Error("Please provide the -common-name flag when customizing the CA")
			c.Ui.Error(commandErrorText(c))
			return 1
		}
		if c.country == "" {
			c.Ui.Error("Please provide the -country flag when customizing the CA")
			c.Ui.Error(commandErrorText(c))
			return 1
		}

		if c.organization == "" {
			c.Ui.Error("Please provide the -organization flag when customizing the CA")
			c.Ui.Error(commandErrorText(c))
			return 1
		}

		if c.organizationalUnit == "" {
			c.Ui.Error("Please provide the -organizational-unit flag when customizing the CA")
			c.Ui.Error(commandErrorText(c))
			return 1
		}
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
		constraints = []string{c.domain, "localhost", "nomad"}
		constraints = append(constraints, c.additionalDomain...)
	}

	ca, pk, err := tlsutil.GenerateCA(tlsutil.CAOpts{
		Name:                c.commonName,
		Days:                c.days,
		PermittedDNSDomains: constraints,
		Country:             c.country,
		PostalCode:          c.postalCode,
		Province:            c.province,
		Locality:            c.locality,
		StreetAddress:       c.streetAddress,
		Organization:        c.organization,
		OrganizationalUnit:  c.organizationalUnit,
	})
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

// IsCustom checks whether any of TLSCACreateCommand parameters have been populated with
// non-default values.
func (c *TLSCACreateCommand) IsCustom() bool {
	return c.commonName == "" &&
		c.country == "" &&
		c.postalCode == "" &&
		c.province == "" &&
		c.locality == "" &&
		c.streetAddress == "" &&
		c.organization == "" &&
		c.organizationalUnit == ""

}
