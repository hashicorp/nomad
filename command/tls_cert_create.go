package command

import (
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/posener/complete"

	"github.com/hashicorp/nomad/helper/flags"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/lib/file"
)

type TLSCertCreateCommand struct {
	Meta

	// dnsNames is a list of additional dns records to add to the SAN addresses
	dnsNames flags.StringFlag

	// ipAddresses is a list of additional IP address records to add to the SAN
	// addresses
	ipAddresses flags.StringFlag

	// ca is used to set a custom CA certificate to create certificates from.
	ca string

	cli    bool
	client bool

	// key is used to set the custom CA certificate key when creating
	// certificates.
	key string

	// days is the number of days the certificate will be valid for.
	days int

	// cluster_region is used to add the region name to the certifacte SAN
	// records
	cluster_region string

	// domain is used to provide a custom domain for the certificate.
	domain string

	server bool
}

func (c *TLSCertCreateCommand) Help() string {
	helpText := `
Usage: nomad tls cert create [options]

  Create a new TLS certificate to use within the Nomad cluster TLS
  configuration. You should use the -client, -server or -cli options to create
  certificates for these roles.

Certificate Create Options:

  -additional-dnsname
    Provide an additional dnsname for Subject Alternative Names.
    "localhost" is always included. This flag may be provided multiple times.

  -additional-ipaddress
    Provide an additional ipaddress for Subject Alternative Names.
    "127.0.0.1" is always included. This flag may be provided multiple times.

  -ca
    Provide path to the certificate authority certificate. Defaults to
    #DOMAIN#-agent-ca.pem.

  -cli
    Generate a certificate for use with the Nomad CLI.

  -client
    Generate a client certificate.

  -cluster-region
    Provide the datacenter. Only used for -server certificates.
    Defaults to "global".

  -days
    Provide number of days the certificate is valid for from now on.
    Defaults to 1 year.

  -domain
    Provide the domain. Only used for -server certificates.

  -key
    Provide path to the certificate authority key. Defaults to
    #DOMAIN#-agent-ca-key.pem.

  -server
    Generate a server certificate.
`
	return strings.TrimSpace(helpText)
}

func (c *TLSCertCreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-additional-dnsname":   complete.PredictAnything,
			"-additional-ipaddress": complete.PredictAnything,
			"-ca":                   complete.PredictAnything,
			"-cli":                  complete.PredictNothing,
			"-client":               complete.PredictNothing,
			"-days":                 complete.PredictAnything,
			"-cluster-region":       complete.PredictAnything,
			"-domain":               complete.PredictAnything,
			"-key":                  complete.PredictAnything,
			"-server":               complete.PredictNothing,
		})
}

func (c *TLSCertCreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *TLSCertCreateCommand) Synopsis() string {
	return "Create a new TLS certificate"
}

func (c *TLSCertCreateCommand) Name() string { return "tls cert create" }

func (c *TLSCertCreateCommand) Run(args []string) int {

	flagSet := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flagSet.Usage = func() { c.Ui.Output(c.Help()) }
	flagSet.Var(&c.dnsNames, "additional-dnsname", "")
	flagSet.Var(&c.ipAddresses, "additional-ipaddress", "")
	flagSet.StringVar(&c.ca, "ca", "#DOMAIN#-agent-ca.pem", "")
	flagSet.BoolVar(&c.cli, "cli", false, "")
	flagSet.BoolVar(&c.client, "client", false, "")
	flagSet.StringVar(&c.key, "key", "#DOMAIN#-agent-ca-key.pem", "")
	flagSet.IntVar(&c.days, "days", 365, "")
	flagSet.StringVar(&c.cluster_region, "cluster-region", "global", "")
	flagSet.StringVar(&c.domain, "domain", "nomad", "")
	flagSet.BoolVar(&c.server, "server", false, "")
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
	if c.ca == "" {
		c.Ui.Error("Please provide the ca")
		return 1
	}
	if c.key == "" {
		c.Ui.Error("Please provide the key")
		return 1
	}
	if !((c.server && !c.client && !c.cli) ||
		(!c.server && c.client && !c.cli) ||
		(!c.server && !c.client && c.cli)) {
		c.Ui.Error("Please provide either -server, -client, or -cli")
		return 1
	}

	var DNSNames []string
	var IPAddresses []net.IP
	var extKeyUsage []x509.ExtKeyUsage
	var name, prefix string

	for _, d := range c.dnsNames {
		if len(d) > 0 {
			DNSNames = append(DNSNames, strings.TrimSpace(d))
		}
	}

	for _, i := range c.ipAddresses {
		if len(i) > 0 {
			IPAddresses = append(IPAddresses, net.ParseIP(strings.TrimSpace(i)))
		}
	}

	if c.server {
		name = fmt.Sprintf("server.%s.%s", c.cluster_region, c.domain)
		DNSNames = append(DNSNames, name)
		DNSNames = append(DNSNames, "localhost")

		IPAddresses = append(IPAddresses, net.ParseIP("127.0.0.1"))
		extKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
		prefix = fmt.Sprintf("%s-server-%s", c.cluster_region, c.domain)

	} else if c.client {
		name = fmt.Sprintf("client.%s.%s", c.cluster_region, c.domain)
		DNSNames = append(DNSNames, []string{name, "localhost"}...)
		IPAddresses = append(IPAddresses, net.ParseIP("127.0.0.1"))
		extKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
		prefix = fmt.Sprintf("%s-client-%s", c.cluster_region, c.domain)
	} else if c.cli {
		name = fmt.Sprintf("cli.%s.%s", c.cluster_region, c.domain)
		DNSNames = []string{name, "localhost"}
		prefix = fmt.Sprintf("%s-cli-%s", c.cluster_region, c.domain)
	} else {
		c.Ui.Error("Neither client, cli nor server - should not happen")
		return 1
	}

	var pkFileName, certFileName string

	tmpCert := fmt.Sprintf("%s.pem", prefix)
	tmpPk := fmt.Sprintf("%s-key.pem", prefix)

	// Check if the CA file already exists
	if !(fileDoesNotExist(tmpCert)) {
		c.Ui.Error(fmt.Sprintf("Certificate file '%s' already exists", tmpCert))
		return 1
	}
	// Check if the Key file file already exists
	if !(fileDoesNotExist(tmpPk)) {
		c.Ui.Error(fmt.Sprintf("Key file '%s' already exists", tmpPk))
		return 1
	}

	certFileName = tmpCert
	pkFileName = tmpPk

	caFile := strings.Replace(c.ca, "#DOMAIN#", c.domain, 1)
	keyFile := strings.Replace(c.key, "#DOMAIN#", c.domain, 1)
	cert, err := os.ReadFile(caFile)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error reading CA: %s", err))
		return 1
	}
	caKey, err := os.ReadFile(keyFile)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error reading CA key: %s", err))
		return 1
	}

	if c.server {
		c.Ui.Warn(
			`==> WARNING: Server Certificates grants authority to become a
    server and access all state in the cluster including root keys
    and all ACL tokens. Do not distribute them to production hosts
    that are not server nodes. Store them as securely as CA keys.`)
	}
	c.Ui.Info("==> Using CA file " + caFile + " and CA key " + keyFile)

	signer, err := tlsutil.ParseSigner(string(caKey))
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	pub, priv, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer: signer, CA: string(cert), Name: name, Days: c.days,
		DNSNames: DNSNames, IPAddresses: IPAddresses, ExtKeyUsage: extKeyUsage,
	})
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	if err = tlsutil.Verify(string(cert), pub, name); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	if err := file.WriteAtomicWithPerms(certFileName, []byte(pub), 0755, 0666); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	if c.server {
		c.Ui.Output("==> Server Certificate saved to " + certFileName)
	} else if c.client {
		c.Ui.Output("==> Client Certificate saved to " + certFileName)
	} else if c.cli {
		c.Ui.Output("==> Cli Certificate saved to " + certFileName)
	}

	if err := file.WriteAtomicWithPerms(pkFileName, []byte(priv), 0755, 0600); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}
	if c.server {
		c.Ui.Output("==> Server Certificate key saved to " + pkFileName)
	} else if c.client {
		c.Ui.Output("==> Client Certificate key saved to " + pkFileName)
	} else if c.cli {
		c.Ui.Output("==> CLI Certificate key saved to " + pkFileName)
	}

	return 0
}
