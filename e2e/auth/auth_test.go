// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

var validPolicySpec = `
namespace "%s" {
  policy = "read"
  variables {
    path "test/*" {
      capabilities = [ "write", "destroy" ]
	}
  }
}

node {
  policy = "write"
}
`

// TestAuth verifies that we're correctly enforcing ACLs with different
// combinations of tokens, policies, API types, and topologies.
func TestAuth(t *testing.T) {

	// Wait until we have a usable cluster before running the tests.
	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 1)

	nodes, _, err := nomadClient.Nodes().List(nil)
	must.NoError(t, err, must.Sprint("expected no error from root client"))
	must.Greater(t, 0, len(nodes))
	node, _, err := nomadClient.Nodes().Info(nodes[0].ID, nil)

	ns := uuid.Generate()
	validPolicyName := uuid.Generate()
	invalidPolicyName := uuid.Generate()

	setupAuthTest(t, nomadClient, ns, validPolicyName, invalidPolicyName)

	// Test cases that exercise requests directly to the server
	t.Run("AnonServerRequests", testAnonServerRequests(node, ns))
	t.Run("BogusServerRequests", testBogusServerRequests(nomadClient, node, ns))
	t.Run("InvalidPermissionsServerRequests",
		testInvalidPermissionsServerRequests(nomadClient, node, ns, invalidPolicyName))
	t.Run("ValidPermissionsServerRequests",
		testValidPermissionsServerRequests(nomadClient, node, ns, validPolicyName))

	// Test cases that exercise requests forwarded from the client
	t.Run("AnonClientRequests", testAnonClientRequests(node, ns))
	t.Run("BogusClientRequests", testBogusClientRequests(nomadClient, node, ns))
	t.Run("InvalidPermissionsClientRequests",
		testInvalidPermissionsClientRequests(nomadClient, node, ns, invalidPolicyName))
	t.Run("ValidPermissionsClientRequests",
		testValidPermissionsClientRequests(nomadClient, node, ns, validPolicyName))
}

func testAnonServerRequests(node *api.Node, ns string) func(t *testing.T) {
	return func(t *testing.T) {
		nomadClient := e2eutil.NomadClient(t)
		nomadClient.SetSecretID("")

		testReadNamespaceAPI(t, nomadClient, ns, "", true)
		testNodeAPI(t, nomadClient, node.ID, "", true)
		testVariablesAPI(t, nomadClient, ns, "", true, true)
	}
}

func testBogusServerRequests(nomadClient *api.Client,
	node *api.Node, ns string) func(t *testing.T) {
	return func(t *testing.T) {
		authToken := uuid.Generate()

		testReadNamespaceAPI(t, nomadClient, ns, authToken, true)
		testNodeAPI(t, nomadClient, node.ID, authToken, true)
		testVariablesAPI(t, nomadClient, ns, authToken, true, true)
	}
}

func testInvalidPermissionsServerRequests(nomadClient *api.Client,
	node *api.Node, ns, policyName string) func(t *testing.T) {
	return func(t *testing.T) {
		token, _, err := nomadClient.ACLTokens().Create(&api.ACLToken{
			Name:          policyName,
			Type:          "client",
			Policies:      []string{policyName},
			ExpirationTTL: time.Minute,
		}, nil)
		must.NoError(t, err)
		authToken := token.SecretID

		testReadNamespaceAPI(t, nomadClient, ns, authToken, true)
		testNodeAPI(t, nomadClient, node.ID, authToken, true)
		testVariablesAPI(t, nomadClient, ns, authToken, true, true)
	}
}

func testValidPermissionsServerRequests(nomadClient *api.Client,
	node *api.Node, ns, policyName string) func(t *testing.T) {
	return func(t *testing.T) {
		token, _, err := nomadClient.ACLTokens().Create(&api.ACLToken{
			Name:          policyName,
			Type:          "client",
			Policies:      []string{policyName},
			ExpirationTTL: time.Minute,
		}, nil)
		must.NoError(t, err)
		authToken := token.SecretID

		testReadNamespaceAPI(t, nomadClient, ns, authToken, false)
		testNodeAPI(t, nomadClient, node.ID, authToken, false)
		testVariablesAPI(t, nomadClient, ns, authToken, false, true)
	}
}

func testAnonClientRequests(node *api.Node, ns string) func(t *testing.T) {
	return func(t *testing.T) {
		config := api.DefaultConfig()
		config.Address = addressForNode(node)
		nomadClient, err := api.NewClient(config)
		nomadClient.SetSecretID("")
		must.NoError(t, err)

		testReadNamespaceAPI(t, nomadClient, ns, "", true)
		testNodeAPI(t, nomadClient, node.ID, "", true)
		testVariablesAPI(t, nomadClient, ns, "", true, true)
	}
}

func testBogusClientRequests(rootClient *api.Client,
	node *api.Node, ns string) func(t *testing.T) {
	return func(t *testing.T) {
		config := api.DefaultConfig()
		config.Address = addressForNode(node)
		nomadClient, err := api.NewClient(config)
		must.NoError(t, err)

		authToken := uuid.Generate()

		testReadNamespaceAPI(t, nomadClient, ns, authToken, true)
		testNodeAPI(t, nomadClient, node.ID, authToken, true)
		testVariablesAPI(t, nomadClient, ns, authToken, true, true)
	}
}

func testInvalidPermissionsClientRequests(rootClient *api.Client,
	node *api.Node, ns, policyName string) func(t *testing.T) {
	return func(t *testing.T) {
		token, _, err := rootClient.ACLTokens().Create(&api.ACLToken{
			Name:          policyName,
			Type:          "client",
			Policies:      []string{policyName},
			ExpirationTTL: time.Minute,
		}, nil)
		must.NoError(t, err)

		config := api.DefaultConfig()
		config.Address = addressForNode(node)
		nomadClient, err := api.NewClient(config)
		must.NoError(t, err)

		authToken := token.SecretID

		testReadNamespaceAPI(t, nomadClient, ns, authToken, true)
		testNodeAPI(t, nomadClient, node.ID, authToken, true)
		testVariablesAPI(t, nomadClient, ns, authToken, true, true)
	}
}

func testValidPermissionsClientRequests(rootClient *api.Client,
	node *api.Node, ns, policyName string) func(t *testing.T) {
	return func(t *testing.T) {
		token, _, err := rootClient.ACLTokens().Create(&api.ACLToken{
			Name:          policyName,
			Type:          "client",
			Policies:      []string{policyName},
			ExpirationTTL: time.Minute,
		}, nil)
		must.NoError(t, err)

		config := api.DefaultConfig()
		config.Address = addressForNode(node)
		nomadClient, err := api.NewClient(config)
		must.NoError(t, err)

		authToken := token.SecretID

		testReadNamespaceAPI(t, nomadClient, ns, authToken, false)
		testNodeAPI(t, nomadClient, node.ID, authToken, false)
		testVariablesAPI(t, nomadClient, ns, authToken, false, true)
	}
}

// testReadNamespaceAPI exercises an API that requires any namespace capability
func testReadNamespaceAPI(t *testing.T, nomadClient *api.Client, ns, authToken string, expectErr bool) {
	t.Helper()
	opts := &api.QueryOptions{AuthToken: authToken}
	_, _, err := nomadClient.Namespaces().Info(ns, opts)
	if expectErr {
		must.Error(t, err, must.Sprint("expected error when reading namespace"))
	} else {
		must.NoError(t, err, must.Sprint("expected no error reading namespace"))
	}
}

// testNodeAPI exercises an API that requires the node:write permission
func testNodeAPI(t *testing.T, nomadClient *api.Client, nodeID, authToken string, expectErr bool) {
	t.Helper()
	opts := &api.WriteOptions{AuthToken: authToken}
	_, _, err := nomadClient.Nodes().ForceEvaluate(nodeID, opts)
	if expectErr {
		must.Error(t, err, must.Sprint("expected error when force-evaluating node"))
	} else {
		must.NoError(t, err, must.Sprint("expected no error force-evaluating node"))
	}
}

// testVariablesAPI exercises an API that requires namespace capabilities for
// variables
func testVariablesAPI(t *testing.T, nomadClient *api.Client, ns, authToken string, expectErrTestPath, expectErrOutsidePath bool) {
	t.Helper()
	opts := &api.WriteOptions{Namespace: ns, AuthToken: authToken}

	_, _, err := nomadClient.Variables().Create(&api.Variable{
		Namespace: ns,
		Path:      "test/" + t.Name(),
		Items:     map[string]string{"foo": t.Name()},
	}, opts)

	if expectErrTestPath {
		must.Error(t, err, must.Sprint("expected error when writing variable"))
	} else {
		must.NoError(t, err, must.Sprint("expected no error writing variable"))
	}
	t.Cleanup(func() {
		_, err := nomadClient.Variables().Delete("test/"+t.Name(), opts)
		if !expectErrTestPath {
			must.NoError(t, err, must.Sprint("expected no error cleaning up variable"))
		}
	})

	_, _, err = nomadClient.Variables().Create(&api.Variable{
		Namespace: ns,
		Path:      "other/" + t.Name(),
		Items:     map[string]string{"foo": t.Name()},
	}, opts)

	if expectErrOutsidePath {
		must.Error(t, err, must.Sprint("expected error when writing variable"))
	} else {
		must.NoError(t, err, must.Sprint("expected no error writing variable"))
	}
	t.Cleanup(func() {
		// no test should ever write this variable, so we don't expect delete to
		// work either but need it for cleanup just in case we did write it
		nomadClient.Variables().Delete("other/"+t.Name(), opts)
	})

}

func setupAuthTest(t *testing.T, nomadClient *api.Client,
	ns, validPolicyName, invalidPolicyName string) {
	t.Helper()

	_, err := nomadClient.Namespaces().Register(&api.Namespace{Name: ns}, nil)
	must.NoError(t, err, must.Sprint("expected no error when registering namespace"))

	t.Cleanup(func() {
		_, err := nomadClient.Namespaces().Delete(ns, nil)
		must.NoError(t, err, must.Sprint("expected no error cleaning up namespace"))
	})

	// Create a valid and useful policy
	_, err = nomadClient.ACLPolicies().Upsert(&api.ACLPolicy{
		Name:  validPolicyName,
		Rules: fmt.Sprintf(validPolicySpec, ns),
	}, nil)
	must.NoError(t, err, must.Sprint("expected no error when registering policy"))

	t.Cleanup(func() {
		_, err := nomadClient.ACLPolicies().Delete(validPolicyName, nil)
		must.NoError(t, err, must.Sprint("expected no error cleaning up ACL policy"))
	})

	// Create a useless policy
	_, err = nomadClient.ACLPolicies().Upsert(&api.ACLPolicy{
		Name:  invalidPolicyName,
		Rules: `plugin { policy = "read" }`,
	}, nil)
	must.NoError(t, err, must.Sprint("expected no error when registering policy"))

	t.Cleanup(func() {
		_, err := nomadClient.ACLPolicies().Delete(invalidPolicyName, nil)
		must.NoError(t, err, must.Sprint("expected no error cleaning up ACL policy"))
	})
}

// addressForNode is a hacky way of getting the address with or without
// mTLS. The test code can't read the api.Client's internals to see if we're in
// mTLS mode, so we assume if the environment is set up for mTLS that we're
// using it. We also need to make sure we're using the AWS public IP address for
// machines running in the nightly E2E environment, and that address isn't the
// advertised address
func addressForNode(node *api.Node) string {
	if publicIP, ok := node.Attributes["unique.platform.aws.public-ipv4"]; ok {
		if v := os.Getenv("NOMAD_CACERT"); v != "" {
			return fmt.Sprintf("https://%s:4646", publicIP)
		} else {
			return fmt.Sprintf("http://%s:4646", publicIP)
		}
	}

	if v := os.Getenv("NOMAD_CACERT"); v != "" {
		return fmt.Sprintf("https://%s", node.HTTPAddr)
	}
	return fmt.Sprintf("http://%s", node.HTTPAddr)
}
