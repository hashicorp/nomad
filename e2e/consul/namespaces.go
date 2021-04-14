package consul

import (
	"fmt"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper"
	"github.com/stretchr/testify/require"
)

// Job files used to test Consul Namespaces. Each job should run on Nomad OSS
// and Nomad ENT with expectations set accordingly.
//
// All tests require Consul Enterprise.
const (
	cnsJobGroupServices      = "consul/input/namespaces/services_group.nomad"
	cnsJobTaskServices       = "consul/input/namespaces/services_task.nomad"
	cnsJobTemplateKV         = "consul/input/namespaces/template_kv.nomad"
	cnsJobConnectSidecars    = "consul/input/namespaces/connect_sidecars.nomad"
	cnsJobConnectIngress     = "consul/input/namespaces/connect_ingress.nomad"
	cnsJobConnectTerminating = "consul/input/namespaces/connect_terminating.nomad"
	cnsJobScriptChecksTask   = "consul/input/namespaces/script_checks_task.nomad"
	cnsJobScriptChecksGroup  = "consul/input/namespaces/script_checks_group.nomad"
)

var (
	// consulNamespaces represents the custom consul namespaces we create and
	// can make use of in tests, but usefully so only in Nomad Enterprise
	consulNamespaces = []string{"apple", "banana", "cherry"}

	// allConsulNamespaces represents all namespaces we expect in consul after
	// creating consulNamespaces, which then includes "default", which is the
	// only namespace accessed by Nomad OSS (outside of agent configuration)
	allConsulNamespaces = append(consulNamespaces, "default")
)

type ConsulNamespacesE2ETest struct {
	framework.TC
	jobIDs []string
}

func (tc *ConsulNamespacesE2ETest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)

	// create a set of consul namespaces in which to register services
	e2eutil.CreateConsulNamespaces(f.T(), tc.Consul(), consulNamespaces)

	// insert a key of the same name into KV for each namespace, where the value
	// contains the namespace name making it easy to determine which namespace
	// consul template actually accessed
	for _, namespace := range allConsulNamespaces {
		value := fmt.Sprintf("ns_%s", namespace)
		e2eutil.PutConsulKey(f.T(), tc.Consul(), namespace, "ns-kv-example", value)
	}
}

func (tc *ConsulNamespacesE2ETest) AfterAll(f *framework.F) {
	e2eutil.DeleteConsulNamespaces(f.T(), tc.Consul(), consulNamespaces)
}

func (tc *ConsulNamespacesE2ETest) TestNamespacesExist(f *framework.F) {
	// make sure our namespaces exist + default
	namespaces := e2eutil.ListConsulNamespaces(f.T(), tc.Consul())
	require.True(f.T(), helper.CompareSliceSetString(namespaces, append(consulNamespaces, "default")))
}
