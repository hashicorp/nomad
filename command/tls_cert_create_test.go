package command

import (
	"crypto/x509"
	"net"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestValidateTLSCertCreateCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(NewCertCreate().Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

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

		"client+node": {[]string{"-client", "-node", "foo"},
			"-node requires -server"},
		"cli+node": {[]string{"-cli", "-node", "foo"},
			"-node requires -server"},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ui := cli.NewMockUi()
			cmd := &TLSCertCreateCommand{
				Meta: Meta{
					Ui: ui,
				},
			}
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
	testDir := testutil.TempDir(t, "tls")

	defer testutil.SwitchToTempDir(t, testDir)()

	ui := cli.NewMockUi()
	caCmd := &TLSCACreateCommand{
		Meta: Meta{
			Ui: ui,
		},
	}

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
	}

	// The following subtests must run serially.
	cases := []testcase{
		{"server0",
			"server",
			[]string{"-server"},
			"dc1-server-nomad-0.pem",
			"dc1-server-nomad-0-key.pem",
			"server.dc1.nomad",
			[]string{
				"server.dc1.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"server1-with-node",
			"server",
			[]string{"-server", "-node", "mysrv"},
			"dc1-server-nomad-1.pem",
			"dc1-server-nomad-1-key.pem",
			"server.dc1.nomad",
			[]string{
				"mysrv.server.dc1.nomad",
				"server.dc1.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"server0-dc2-altdomain",
			"server",
			[]string{"-server", "-dc", "dc2", "-domain", "nomad"},
			"dc2-server-nomad-0.pem",
			"dc2-server-nomad-0-key.pem",
			"server.dc2.nomad",
			[]string{
				"server.dc2.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"client0",
			"client",
			[]string{"-client"},
			"dc1-client-nomad-0.pem",
			"dc1-client-nomad-0-key.pem",
			"client.dc1.nomad",
			[]string{
				"client.dc1.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"client1",
			"client",
			[]string{"-client"},
			"dc1-client-nomad-1.pem",
			"dc1-client-nomad-1-key.pem",
			"client.dc1.nomad",
			[]string{
				"client.dc1.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"client0-dc2-altdomain",
			"client",
			[]string{"-client", "-dc", "dc2", "-domain", "nomad"},
			"dc2-client-nomad-0.pem",
			"dc2-client-nomad-0-key.pem",
			"client.dc2.nomad",
			[]string{
				"client.dc2.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"cli0",
			"cli",
			[]string{"-cli"},
			"dc1-cli-nomad-0.pem",
			"dc1-cli-nomad-0-key.pem",
			"cli.dc1.nomad",
			[]string{
				"cli.dc1.nomad",
				"localhost",
			},
			nil,
		},
		{"cli1",
			"cli",
			[]string{"-cli"},
			"dc1-cli-nomad-1.pem",
			"dc1-cli-nomad-1-key.pem",
			"cli.dc1.nomad",
			[]string{
				"cli.dc1.nomad",
				"localhost",
			},
			nil,
		},
		{"cli0-dc2-altdomain",
			"cli",
			[]string{"-cli", "-dc", "dc2", "-domain", "nomad"},
			"dc2-cli-nomad-0.pem",
			"dc2-cli-nomad-0-key.pem",
			"cli.dc2.nomad",
			[]string{
				"cli.dc2.nomad",
				"localhost",
			},
			nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		require.True(t, t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &TLSCertCreateCommand{
				Meta: Meta{
					Ui: ui,
				},
			}
			require.Equal(t, 0, cmd.Run(tc.args))
			require.Equal(t, "", ui.ErrorWriter.String())

			cert, _ := testutil.ExpectFiles(t, tc.certPath, tc.keyPath)
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
