// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"regexp"
	"testing"
	"time"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

// TestConnect_LegacyACLs tests the workflows where the operator provides their
// own token, rather than using Nomad's token or Workload Identity
func TestConnect_LegacyACLs(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 2)

	t.Cleanup(func() {
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	t.Run("ConnectDemo", testConnectDemoLegacyACLs)
	t.Run("ConnectDemoNamespaced", testConnectDemoLegacyACLsNamespaced)
	t.Run("ConnectNativeDemo", testConnectNativeDemoLegacyACLs)
	t.Run("ConnectIngressGatewayDemo", testConnectIngressGatewayDemoLegacyACLs)
	t.Run("ConnectTerminatingGateway", testConnectTerminatingGatewayLegacyACLs)
}

func createPolicy(t *testing.T, cc *capi.Client, ns, rules string) string {
	policy, _, err := cc.ACL().PolicyCreate(&capi.ACLPolicy{
		Name:      "nomad-operator-policy-" + uuid.Short(),
		Rules:     rules,
		Namespace: ns,
	}, nil)
	must.NoError(t, err)
	t.Cleanup(func() { cc.ACL().PolicyDelete(policy.ID, nil) })
	return policy.ID
}

func createToken(t *testing.T, cc *capi.Client, policyID, ns string) string {
	token, _, err := cc.ACL().TokenCreate(&capi.ACLToken{
		Description: "test token",
		Policies:    []*capi.ACLTokenPolicyLink{{ID: policyID}},
		Namespace:   ns,
	}, nil)
	must.NoError(t, err)
	t.Cleanup(func() { cc.ACL().TokenDelete(token.AccessorID, nil) })
	return token.SecretID
}

// testConnectDemoLegacyACLs tests the demo job file used in Connect Integration examples.
func testConnectDemoLegacyACLs(t *testing.T) {
	cc := e2eutil.ConsulClient(t)

	policyID := createPolicy(t, cc, "default",
		`service "count-api" { policy = "write" } service "count-dashboard" { policy = "write" }`)

	token := createToken(t, cc, policyID, "default")

	sub, _ := jobs3.Submit(t, "./input/demo.nomad",
		jobs3.Timeout(time.Second*60), jobs3.LegacyConsulToken(token))

	ixn := &capi.Intention{
		SourceName:      "count-dashboard",
		DestinationName: "count-api",
		Action:          "allow",
	}
	_, err := cc.Connect().IntentionUpsert(ixn, nil)
	must.NoError(t, err, must.Sprint("could not create intention"))

	t.Cleanup(func() {
		_, err := cc.Connect().IntentionDeleteExact("count-dashboard", "count-api", nil)
		test.NoError(t, err)
	})

	assertServiceOk(t, cc, "count-api-sidecar-proxy")
	assertServiceOk(t, cc, "count-dashboard-sidecar-proxy")
	assertSITokens(t, cc, map[string]int{
		"connect-proxy-count-api": 1, "connect-proxy-count-dashboard": 1})

	logs := sub.Exec("dashboard", "dashboard",
		[]string{"/bin/sh", "-c", "wget -O /dev/null http://${NOMAD_UPSTREAM_ADDR_count_api}"})
	must.StrContains(t, logs.Stderr, "saving to")
}

// testConnectDemoLegacyACLsNamespaced tests the demo job file used in Connect
// Integration examples.
func testConnectDemoLegacyACLsNamespaced(t *testing.T) {
	cc := e2eutil.ConsulClient(t)

	ns := "ns-" + uuid.Short()
	_, _, err := cc.Namespaces().Create(&capi.Namespace{Name: ns}, nil)
	must.NoError(t, err)
	t.Cleanup(func() { cc.Namespaces().Delete(ns, nil) })

	policyID := createPolicy(t, cc, ns,
		`service "count-api" { policy = "write" } service "count-dashboard" { policy = "write" }`)

	token := createToken(t, cc, policyID, ns)

	jobs3.Submit(t, "./input/demo.nomad",
		jobs3.Timeout(time.Second*60), jobs3.LegacyConsulToken(token))

	ixn := &capi.Intention{
		SourceName:      "count-dashboard",
		DestinationName: "count-api",
		Action:          "allow",
	}
	_, err = cc.Connect().IntentionUpsert(ixn, nil)
	must.NoError(t, err, must.Sprint("could not create intention"))

	t.Cleanup(func() {
		_, err := cc.Connect().IntentionDeleteExact("count-dashboard", "count-api", nil)
		test.NoError(t, err)
	})

	assertServiceOk(t, cc, "count-api-sidecar-proxy")
	assertServiceOk(t, cc, "count-dashboard-sidecar-proxy")
	assertSITokens(t, cc, map[string]int{
		"connect-proxy-count-api": 1, "connect-proxy-count-dashboard": 1})

}

// testConnectNativeDemoLegacyACLs tests the demo job file used in Connect Native
// Integration examples.
func testConnectNativeDemoLegacyACLs(t *testing.T) {
	cc := e2eutil.ConsulClient(t)

	policyID := createPolicy(t, cc, "default",
		`service "uuid-fe" { policy = "write" } service "uuid-api" { policy = "write" }`)

	token := createToken(t, cc, policyID, "default")

	jobs3.Submit(t, "./input/native-demo.nomad",
		jobs3.Timeout(time.Second*60), jobs3.LegacyConsulToken(token))

	assertSITokens(t, cc, map[string]int{"frontend": 1, "generate": 1})
}

// testConnectIngressGatewayDemoLegacyACLs tests a job with an ingress gateway
func testConnectIngressGatewayDemoLegacyACLs(t *testing.T) {
	cc := e2eutil.ConsulClient(t)

	policyID := createPolicy(t, cc, "default",
		`service "my-ingress-service" { policy = "write" } service "uuid-api" { policy = "write" }`)

	token := createToken(t, cc, policyID, "default")

	jobs3.Submit(t, "./input/ingress-gateway.nomad",
		jobs3.Timeout(time.Second*60), jobs3.LegacyConsulToken(token))

	assertSITokens(t, cc, map[string]int{"connect-ingress-my-ingress-service": 1, "generate": 1})
}

// testConnectTerminatingGatewayLegacyACLs tests a job with a terminating gateway
func testConnectTerminatingGatewayLegacyACLs(t *testing.T) {
	cc := e2eutil.ConsulClient(t)

	policyID := createPolicy(t, cc, "default",
		`service "api-gateway" { policy = "write" } service "count-dashboard" { policy = "write" }`)

	token := createToken(t, cc, policyID, "default")

	jobs3.Submit(t, "./input/terminating-gateway.nomad",
		jobs3.Timeout(time.Second*60), jobs3.LegacyConsulToken(token))

	ixn := &capi.Intention{
		SourceName:      "count-dashboard",
		DestinationName: "count-api",
		Action:          "allow",
	}
	_, err := cc.Connect().IntentionUpsert(ixn, nil)
	must.NoError(t, err, must.Sprint("could not create intention"))

	t.Cleanup(func() {
		_, err := cc.Connect().IntentionDeleteExact("count-dashboard", "count-api", nil)
		test.NoError(t, err)
	})

	assertServiceOk(t, cc, "api-gateway")
	assertServiceOk(t, cc, "count-dashboard-sidecar-proxy")
	assertServiceOk(t, cc, "count-api")

	assertSITokens(t, cc, map[string]int{
		"connect-terminating-api-gateway": 1, "connect-proxy-count-dashboard": 1})
}

func assertSITokens(t *testing.T, cc *capi.Client, expect map[string]int) {
	tokens, _, err := cc.ACL().TokenList(nil)
	must.NoError(t, err)

	// count the number of SI tokens matching each service name
	foundSITokens := make(map[string]int)
	for _, token := range tokens {
		if service := serviceofSIToken(token.Description); service != "" {
			foundSITokens[service]++
		}
	}
	for expected, count := range expect {
		test.Eq(t, count, foundSITokens[expected], test.Sprintf("expected tokens for %q", expected))
	}
}

func Test_serviceOfSIToken(t *testing.T) {
	try := func(description, exp string) {
		result := serviceofSIToken(description)
		must.Eq(t, exp, result)
	}

	try("", "")
	try("foobarbaz", "")
	try("_nomad_si [8b1a5d3f-7e61-4a5a-8a57-7e7ad91e63b6] [8b1a5d3f-7e61-4a5a-8a57-7e7ad91e63b6] [foo-service]", "foo-service")
}

var (
	siTokenRe = regexp.MustCompile(`_nomad_si \[[\w-]{36}] \[[\w-]{36}] \[([\S]+)]`)
)

func serviceofSIToken(description string) string {
	if m := siTokenRe.FindStringSubmatch(description); len(m) == 2 {
		return m[1]
	}
	return ""
}
