// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"testing"
	"time"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestConnect(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 2)

	t.Cleanup(func() {
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	t.Run("ConnectDemo", testConnectDemo)
	t.Run("ConnectCustomSidecarExposed", testConnectCustomSidecarExposed)
	t.Run("ConnectNativeDemo", testConnectNativeDemo)
	t.Run("ConnectIngressGatewayDemo", testConnectIngressGatewayDemo)
	t.Run("ConnectMultiIngress", testConnectMultiIngressGateway)
	t.Run("ConnectTerminatingGateway", testConnectTerminatingGateway)
	t.Run("ConnectMultiService", testConnectMultiService)
	t.Run("ConnectTransparentProxy", testConnectTransparentProxy)
}

// testConnectDemo tests the demo job file used in Connect Integration examples.
func testConnectDemo(t *testing.T) {
	sub, _ := jobs3.Submit(t, "./input/demo.nomad", jobs3.Timeout(time.Second*60))

	cc := e2eutil.ConsulClient(t)

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

	logs := sub.Exec("dashboard", "dashboard",
		[]string{"/bin/sh", "-c", "wget -O /dev/null http://${NOMAD_UPSTREAM_ADDR_count_api}"})
	must.StrContains(t, logs.Stderr, "saving to")
}

// testConnectCustomSidecarExposed tests that a connect sidecar with custom task
// definition can also make use of the expose service check feature.
func testConnectCustomSidecarExposed(t *testing.T) {
	jobs3.Submit(t, "./input/expose-custom.nomad", jobs3.Timeout(time.Second*60))
}

// testConnectNativeDemo tests the demo job file used in Connect Native
// Integration examples.
func testConnectNativeDemo(t *testing.T) {
	jobs3.Submit(t, "./input/native-demo.nomad", jobs3.Timeout(time.Second*60))
}

// testConnectIngressGatewayDemo tests a job with an ingress gateway
func testConnectIngressGatewayDemo(t *testing.T) {
	jobs3.Submit(t, "./input/ingress-gateway.nomad", jobs3.Timeout(time.Second*60))
}

// testConnectMultiIngressGateway tests a job with multiple ingress gateways
func testConnectMultiIngressGateway(t *testing.T) {
	jobs3.Submit(t, "./input/multi-ingress.nomad", jobs3.Timeout(time.Second*60))
}

// testConnectTerminatingGateway tests a job with a terminating gateway
func testConnectTerminatingGateway(t *testing.T) {
	jobs3.Submit(t, "./input/terminating-gateway.nomad", jobs3.Timeout(time.Second*60))

	cc := e2eutil.ConsulClient(t)

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
}

// testConnectMultiService tests a job with multiple Connect blocks in the same
// group
func testConnectMultiService(t *testing.T) {
	jobs3.Submit(t, "./input/multi-service.nomad", jobs3.Timeout(time.Second*60))

	cc := e2eutil.ConsulClient(t)
	assertServiceOk(t, cc, "echo1-sidecar-proxy")
	assertServiceOk(t, cc, "echo2-sidecar-proxy")
}

// testConnectTransparentProxy tests the Connect Transparent Proxy integration
func testConnectTransparentProxy(t *testing.T) {
	sub, _ := jobs3.Submit(t, "./input/tproxy.nomad.hcl", jobs3.Timeout(time.Second*60))

	cc := e2eutil.ConsulClient(t)

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

	logs := sub.Exec("dashboard", "dashboard",
		[]string{"wget", "-O", "/dev/null", "count-api.virtual.consul"})
	must.StrContains(t, logs.Stderr, "saving to")
}

// assertServiceOk is a test helper to assert a service is passing health checks, if any
func assertServiceOk(t *testing.T, cc *capi.Client, name string) {
	t.Helper()
	services, _, err := cc.Health().Service(name, "", false, nil)
	must.NoError(t, err)
	must.Greater(t, 0, len(services), must.Sprintf("found no services for %q", name))

	status := services[0].Checks.AggregatedStatus()
	must.Eq(t, "passing", status)
}
