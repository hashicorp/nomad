// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"crypto/x509"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestTlsCertCreateCommand_InvalidArgs(t *testing.T) {
	ci.Parallel(t)

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
			ci.Parallel(t)
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

func TestTlsCertCreateCommandDefaults_fileCreate(t *testing.T) {
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
		{"server0-region1",
			"server",
			[]string{"-server", "-region", "region1"},
			"region1-server-nomad.pem",
			"region1-server-nomad-key.pem",
			"server.region1.nomad",
			[]string{
				"server.region1.nomad",
				"server.global.nomad",
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
			[]net.IP(nil),
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
					[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
					cert.ExtKeyUsage)
			case "cli":
				require.Equal(t,
					[]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
					cert.ExtKeyUsage)
			}
			require.False(t, cert.IsCA)
			require.Equal(t, tc.expectDNS, cert.DNSNames)
			require.Equal(t, tc.expectIP, cert.IPAddresses)
		}))
	}
}

func TestTlsRecordPreparation(t *testing.T) {
	type testcase struct {
		name                string
		certType            string
		regionName          string
		domain              string
		dnsNames            []string
		ipAddresses         []string
		expectedipAddresses []net.IP
		expectedDNSNames    []string
		expectedName        string
		expectedextKeyUsage []x509.ExtKeyUsage
		expectedPrefix      string
	}
	// The default values are region = global and domain = nomad.
	cases := []testcase{
		{
			name:                "server0",
			certType:            "server",
			regionName:          "global",
			domain:              "nomad",
			dnsNames:            []string{},
			ipAddresses:         []string{},
			expectedipAddresses: []net.IP{net.ParseIP("127.0.0.1")},
			expectedDNSNames: []string{
				"server.global.nomad",
				"localhost",
			},
			expectedName:        "server.global.nomad",
			expectedextKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			expectedPrefix:      "global-server-nomad",
		},
		{
			name:                "server0-region1",
			certType:            "server",
			regionName:          "region1",
			domain:              "nomad",
			dnsNames:            []string{},
			ipAddresses:         []string{},
			expectedipAddresses: []net.IP{net.ParseIP("127.0.0.1")},
			expectedDNSNames: []string{
				"server.region1.nomad",
				"server.global.nomad",
				"localhost",
			},
			expectedName:        "server.region1.nomad",
			expectedextKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			expectedPrefix:      "region1-server-nomad",
		},
		{
			name:                "server0-domain1",
			certType:            "server",
			regionName:          "global",
			domain:              "domain1",
			dnsNames:            []string{},
			ipAddresses:         []string{},
			expectedipAddresses: []net.IP{net.ParseIP("127.0.0.1")},
			expectedDNSNames: []string{
				"server.global.nomad",
				"server.global.domain1",
				"localhost",
			},
			expectedName:        "server.global.domain1",
			expectedextKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			expectedPrefix:      "global-server-domain1",
		},
		{
			name:                "server0-dns",
			certType:            "server",
			regionName:          "global",
			domain:              "nomad",
			dnsNames:            []string{"server.global.foo"},
			ipAddresses:         []string{},
			expectedipAddresses: []net.IP{net.ParseIP("127.0.0.1")},
			expectedDNSNames: []string{
				"server.global.foo",
				"server.global.nomad",
				"localhost",
			},
			expectedName:        "server.global.nomad",
			expectedextKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			expectedPrefix:      "global-server-nomad",
		},
		{
			name:                "server0-ips",
			certType:            "server",
			regionName:          "global",
			domain:              "nomad",
			dnsNames:            []string{},
			ipAddresses:         []string{"10.0.0.1"},
			expectedipAddresses: []net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("127.0.0.1")},
			expectedDNSNames: []string{
				"server.global.nomad",
				"localhost",
			},
			expectedName:        "server.global.nomad",
			expectedextKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			expectedPrefix:      "global-server-nomad",
		},
		{
			name:                "client0",
			certType:            "client",
			regionName:          "global",
			domain:              "nomad",
			dnsNames:            []string{},
			ipAddresses:         []string{},
			expectedipAddresses: []net.IP{net.ParseIP("127.0.0.1")},
			expectedDNSNames: []string{
				"client.global.nomad",
				"localhost",
			},
			expectedName:        "client.global.nomad",
			expectedextKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			expectedPrefix:      "global-client-nomad",
		},
		{
			name:                "cli0",
			certType:            "cli",
			regionName:          "global",
			domain:              "nomad",
			dnsNames:            []string{},
			ipAddresses:         []string{},
			expectedipAddresses: []net.IP(nil),
			expectedDNSNames: []string{
				"cli.global.nomad",
				"localhost",
			},
			expectedName:        "cli.global.nomad",
			expectedextKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
			expectedPrefix:      "global-cli-nomad",
		},
	}

	for _, tc := range cases {
		tc := tc
		require.True(t, t.Run(tc.name, func(t *testing.T) {
			var ipAddresses []net.IP
			for _, i := range tc.ipAddresses {
				if len(i) > 0 {
					ipAddresses = append(ipAddresses, net.ParseIP(strings.TrimSpace(i)))
				}
			}

			ipAddresses, dnsNames, name, extKeyUsage, prefix := recordPreparation(tc.certType, tc.regionName, tc.domain, tc.dnsNames, ipAddresses)
			require.Equal(t, tc.expectedipAddresses, ipAddresses)
			require.Equal(t, tc.expectedDNSNames, dnsNames)
			require.Equal(t, tc.expectedName, name)
			require.Equal(t, tc.expectedextKeyUsage, extKeyUsage)
			require.Equal(t, tc.expectedPrefix, prefix)
		}))
	}
}
