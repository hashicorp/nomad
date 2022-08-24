package command

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"github.com/hashicorp/nomad/helper/flags"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/lib/file"
	"github.com/posener/complete"
)

type TLSCertCreateCommand struct {
	Meta
}

func NewCertCreate() *TLSCertCreateCommand {
	return &TLSCertCreateCommand{}
}

func (c *TLSCertCreateCommand) Help() string {
	helpText := `
Usage: nomad tls cert create [options]

Create a new certificate

$ nomad tls cert create -server
==> WARNING: Server Certificates grants authority to become a
    server and access all state in the cluster including root keys
    and all ACL tokens. Do not distribute them to production hosts
    that are not server nodes. Store them as securely as CA keys.
==> Using nomad-agent-ca.pem and nomad-agent-ca-key.pem
==> Saved dc1-server-nomad-0.pem
==> Saved dc1-server-nomad-0-key.pem
$ nomad tls cert create -client
==> Using nomad-agent-ca.pem and nomad-agent-ca-key.pem
==> Saved dc1-client-nomad-0.pem
==> Saved dc1-client-nomad-0-key.pem

Certificate Create Options:
  -additional-dnsname
    Provide an additional dnsname for Subject Alternative Names.
    localhost is always included. This flag may be provided multiple times.

  -additional-ipaddress
    Provide an additional ipaddress for Subject Alternative Names.
    "127.0.0.1 is always included. This flag may be provided multiple times.")

  -ca
    Provide path to the ca. Defaults to #DOMAIN#-agent-ca.pem.

  -cli
    Generate cli certificate.

  -client
    Generate client certificate.

  -days
    Provide number of days the certificate is valid for from now on. 
    Defaults to 1 year.

  -dc
    Provide the datacenter. Matters only for -server certificates. 
    Defaults to dc1.

  -domain
    Provide the domain. Matters only for -server certificates.
  
  -key
    Provide path to the key. Defaults to #DOMAIN#-agent-ca-key.pem.

  -node
    When generating a server cert and this is set an additional dns name is 
    included of the form <node>.server.<datacenter>.<domain>.

  - server
    Generate server certificate.
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
			"-dc":                   complete.PredictAnything,
			"-domain":               complete.PredictAnything,
			"-key":                  complete.PredictAnything,
			"-node":                 complete.PredictAnything,
			"-server":               complete.PredictNothing,
		})
}

func (c *TLSCertCreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *TLSCertCreateCommand) Synopsis() string {
	return "Create a new certificate"
}

func (c *TLSCertCreateCommand) Name() string { return "tls cert create" }

func (c *TLSCertCreateCommand) Run(args []string) int {

	var (
		dnsnames    flags.StringFlag
		ipaddresses flags.StringFlag
		ca          string
		cli         bool
		client      bool
		key         string
		days        int
		dc          string
		domain      string
		node        string
		server      bool
	)

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.Var(&dnsnames, "additional-dnsname", "")
	flags.Var(&ipaddresses, "additional-ipaddress", "")
	flags.StringVar(&ca, "ca", "#DOMAIN#-agent-ca.pem", "")
	flags.BoolVar(&cli, "cli", false, "")
	flags.BoolVar(&client, "client", false, "")
	flags.StringVar(&key, "key", "#DOMAIN#-agent-ca-key.pem", "")
	flags.IntVar(&days, "days", 365, "")
	flags.StringVar(&dc, "dc", "dc1", "")
	flags.StringVar(&domain, "domain", "nomad", "")
	flags.StringVar(&node, "node", "", "")
	flags.BoolVar(&server, "server", false, "")
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
	if ca == "" {
		c.Ui.Error("Please provide the ca")
		return 1
	}
	if key == "" {
		c.Ui.Error("Please provide the key")
		return 1
	}
	if !((server && !client && !cli) ||
		(!server && client && !cli) ||
		(!server && !client && cli)) {
		c.Ui.Error("Please provide either -server, -client, or -cli")
		return 1
	}

	if node != "" && !server {
		c.Ui.Error("-node requires -server")
		return 1
	}

	var DNSNames []string
	var IPAddresses []net.IP
	var extKeyUsage []x509.ExtKeyUsage
	var name, prefix string

	for _, d := range dnsnames {
		if len(d) > 0 {
			DNSNames = append(DNSNames, strings.TrimSpace(d))
		}
	}

	for _, i := range ipaddresses {
		if len(i) > 0 {
			IPAddresses = append(IPAddresses, net.ParseIP(strings.TrimSpace(i)))
		}
	}

	if server {
		name = fmt.Sprintf("server.%s.%s", dc, domain)

		if node != "" {
			nodeName := fmt.Sprintf("%s.server.%s.%s", node, dc, domain)
			DNSNames = append(DNSNames, nodeName)
		}
		DNSNames = append(DNSNames, name)
		DNSNames = append(DNSNames, "localhost")

		IPAddresses = append(IPAddresses, net.ParseIP("127.0.0.1"))
		extKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
		prefix = fmt.Sprintf("%s-server-%s", dc, domain)

	} else if client {
		name = fmt.Sprintf("client.%s.%s", dc, domain)
		DNSNames = append(DNSNames, []string{name, "localhost"}...)
		IPAddresses = append(IPAddresses, net.ParseIP("127.0.0.1"))
		extKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
		prefix = fmt.Sprintf("%s-client-%s", dc, domain)
	} else if cli {
		name = fmt.Sprintf("cli.%s.%s", dc, domain)
		DNSNames = []string{name, "localhost"}
		prefix = fmt.Sprintf("%s-cli-%s", dc, domain)
	} else {
		c.Ui.Error("Neither client, cli nor server - should not happen")
		return 1
	}

	var pkFileName, certFileName string
	max := 10000
	for i := 0; i <= max; i++ {
		tmpCert := fmt.Sprintf("%s-%d.pem", prefix, i)
		tmpPk := fmt.Sprintf("%s-%d-key.pem", prefix, i)
		if FileDoesNotExist(tmpCert) && FileDoesNotExist(tmpPk) {
			certFileName = tmpCert
			pkFileName = tmpPk
			break
		}
		if i == max {
			c.Ui.Error("Could not find a filename that doesn't already exist")
			return 1
		}
	}

	caFile := strings.Replace(ca, "#DOMAIN#", domain, 1)
	keyFile := strings.Replace(key, "#DOMAIN#", domain, 1)
	cert, err := ioutil.ReadFile(caFile)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error reading CA: %s", err))
		return 1
	}
	caKey, err := ioutil.ReadFile(keyFile)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error reading CA key: %s", err))
		return 1
	}

	if server {
		c.Ui.Info(
			`==> WARNING: Server Certificates grants authority to become a
    server and access all state in the cluster including root keys
    and all ACL tokens. Do not distribute them to production hosts
    that are not server nodes. Store them as securely as CA keys.`)
	}
	c.Ui.Info("==> Using " + caFile + " and " + keyFile)

	signer, err := tlsutil.ParseSigner(string(caKey))
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	pub, priv, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer: signer, CA: string(cert), Name: name, Days: days,
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
	c.Ui.Output("==> Saved " + certFileName)

	if err := file.WriteAtomicWithPerms(pkFileName, []byte(priv), 0755, 0600); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}
	c.Ui.Output("==> Saved " + pkFileName)

	return 0
}
