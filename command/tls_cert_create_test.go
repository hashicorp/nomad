package command

import (
	"crypto/x509"
	"net"
	"os"
	"testing"

	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestTlsCertCreateCommand_InvalidArgs(t *testing.T) {
	t.Parallel()

	type testcase struct {
		args      []string
		expectErr string
	}

	cases := map[string]testcase{
		"no args (ca/key inferred)": {[]string{},
			"Please provide either -server, -client, or -cli"},
		"no ca": {[]string{"-ca", "", "-key", ""},
			"Please provide the ca"},
		"no key": {[]string{"-ca", "foo.pem", "-key", ""},
			"Please provide the key"},

		"server+client+cli": {[]string{"-server", "-client", "-cli"},
			"Please provide either -server, -client, or -cli"},
		"server+client": {[]string{"-server", "-client"},
			"Please provide either -server, -client, or -cli"},
		"server+cli": {[]string{"-server", "-cli"},
			"Please provide either -server, -client, or -cli"},
		"client+cli": {[]string{"-client", "-cli"},
			"Please provide either -server, -client, or -cli"},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ui := cli.NewMockUi()
			cmd := &TLSCertCreateCommand{Meta: Meta{Ui: ui}}
			require.NotEqual(t, 0, cmd.Run(tc.args))
			got := ui.ErrorWriter.String()
			if tc.expectErr == "" {
				require.NotEmpty(t, got) // don't care
			} else {
				require.Contains(t, got, tc.expectErr)
			}
		})
	}
}

func TestTlsCertCreateCommand_fileCreate(t *testing.T) {
	testDir := t.TempDir()
	previousDirectory, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(testDir))
	defer os.Chdir(previousDirectory)

	ui := cli.NewMockUi()
	caCmd := &TLSCACreateCommand{Meta: Meta{Ui: ui}}

	// Setup CA keys
	caCmd.Run([]string{"nomad"})

	type testcase struct {
		name      string
		typ       string
		args      []string
		certPath  string
		keyPath   string
		expectCN  string
		expectDNS []string
		expectIP  []net.IP
		errOut    string
	}

	// The following subtests must run serially.
	cases := []testcase{
		{"server0",
			"server",
			[]string{"-server"},
			"global-server-nomad.pem",
			"global-server-nomad-key.pem",
			"server.global.nomad",
			[]string{
				"server.global.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
			"==> WARNING: Server Certificates grants authority to become a\n    server and access all state in the cluster including root keys\n    and all ACL tokens. Do not distribute them to production hosts\n    that are not server nodes. Store them as securely as CA keys.\n",
		},
		{"server0-region2-altdomain",
			"server",
			[]string{"-server", "-cluster-region", "region2", "-domain", "nomad"},
			"region2-server-nomad.pem",
			"region2-server-nomad-key.pem",
			"server.region2.nomad",
			[]string{
				"server.region2.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
			"==> WARNING: Server Certificates grants authority to become a\n    server and access all state in the cluster including root keys\n    and all ACL tokens. Do not distribute them to production hosts\n    that are not server nodes. Store them as securely as CA keys.\n",
		},
		{"client0",
			"client",
			[]string{"-client"},
			"global-client-nomad.pem",
			"global-client-nomad-key.pem",
			"client.global.nomad",
			[]string{
				"client.global.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
			"",
		},
		{"client0-region2-altdomain",
			"client",
			[]string{"-client", "-cluster-region", "region2", "-domain", "nomad"},
			"region2-client-nomad.pem",
			"region2-client-nomad-key.pem",
			"client.region2.nomad",
			[]string{
				"client.region2.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
			"",
		},
		{"cli0",
			"cli",
			[]string{"-cli"},
			"global-cli-nomad.pem",
			"global-cli-nomad-key.pem",
			"cli.global.nomad",
			[]string{
				"cli.global.nomad",
				"localhost",
			},
			nil,
			"",
		},
		{"cli0-region2-altdomain",
			"cli",
			[]string{"-cli", "-cluster-region", "region2", "-domain", "nomad"},
			"region2-cli-nomad.pem",
			"region2-cli-nomad-key.pem",
			"cli.region2.nomad",
			[]string{
				"cli.region2.nomad",
				"localhost",
			},
			nil,
			"",
		},
	}

	for _, tc := range cases {
		tc := tc
		require.True(t, t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &TLSCertCreateCommand{Meta: Meta{Ui: ui}}
			require.Equal(t, 0, cmd.Run(tc.args))
			require.Equal(t, tc.errOut, ui.ErrorWriter.String())

			// is a valid cert expects the cert
			cert := testutil.IsValidCertificate(t, tc.certPath)
			require.Equal(t, tc.expectCN, cert.Subject.CommonName)
			require.True(t, cert.BasicConstraintsValid)
			require.Equal(t, x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment, cert.KeyUsage)
			switch tc.typ {
			case "server":
				require.Equal(t,
					[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
					cert.ExtKeyUsage)
			case "client":
				require.Equal(t,
					[]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
					cert.ExtKeyUsage)
			case "cli":
				require.Len(t, cert.ExtKeyUsage, 0)
			}
			require.False(t, cert.IsCA)
			require.Equal(t, tc.expectDNS, cert.DNSNames)
			require.Equal(t, tc.expectIP, cert.IPAddresses)
		}))
	}
}
