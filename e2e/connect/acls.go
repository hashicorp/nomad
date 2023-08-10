// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"os"
	"regexp"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	uuidparse "github.com/hashicorp/go-uuid"
	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/stretchr/testify/require"
)

type ConnectACLsE2ETest struct {
	framework.TC

	// used to store the root token so we can reset the client back to
	// it as needed
	consulManagementToken string

	// things to cleanup after each test case
	jobIDs          []string
	consulPolicyIDs []string
	consulTokenIDs  []string
}

func (tc *ConnectACLsE2ETest) BeforeAll(f *framework.F) {
	// Wait for Nomad to be ready before doing anything.
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 2)

	// Validate the consul root token exists, otherwise tests are just
	// going to be a train wreck.
	tc.consulManagementToken = os.Getenv(envConsulToken)

	_, err := uuidparse.ParseUUID(tc.consulManagementToken)
	f.NoError(err, "CONSUL_HTTP_TOKEN not set")

	// ensure SI tokens from previous test cases were removed
	f.Eventually(func() bool {
		siTokens := tc.countSITokens(f.T())
		f.T().Log("cleanup: checking for remaining SI tokens:", siTokens)
		return len(siTokens) == 0
	}, 2*time.Minute, 2*time.Second, "SI tokens did not get removed")
}

// AfterEach does cleanup of Consul ACL objects that were created during each
// test case. Each test case may assume it is starting from a "fresh" state -
// as if the consul ACL bootstrap process had just taken place.
func (tc *ConnectACLsE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	t := f.T()

	// cleanup jobs
	for _, id := range tc.jobIDs {
		t.Log("cleanup: deregister nomad job id:", id)
		_, _, err := tc.Nomad().Jobs().Deregister(id, true, nil)
		f.NoError(err)
	}

	// cleanup consul tokens
	for _, id := range tc.consulTokenIDs {
		t.Log("cleanup: delete consul token id:", id)
		_, err := tc.Consul().ACL().TokenDelete(id, &consulapi.WriteOptions{Token: tc.consulManagementToken})
		f.NoError(err)
	}

	// cleanup consul policies
	for _, id := range tc.consulPolicyIDs {
		t.Log("cleanup: delete consul policy id:", id)
		_, err := tc.Consul().ACL().PolicyDelete(id, &consulapi.WriteOptions{Token: tc.consulManagementToken})
		f.NoError(err)
	}

	// do garbage collection
	err := tc.Nomad().System().GarbageCollect()
	f.NoError(err)

	// assert there are no leftover SI tokens, which may take a minute to be
	// cleaned up
	f.Eventually(func() bool {
		siTokens := tc.countSITokens(t)
		t.Log("cleanup: checking for remaining SI tokens:", siTokens)
		return len(siTokens) == 0
	}, 2*time.Minute, 2*time.Second, "SI tokens did not get removed")

	tc.jobIDs = []string{}
	tc.consulTokenIDs = []string{}
	tc.consulPolicyIDs = []string{}
}

// todo(shoenig): follow up refactor with e2eutil.ConsulPolicy
type consulPolicy struct {
	Name  string // e.g. nomad-operator
	Rules string // e.g. service "" { policy="write" }
}

// todo(shoenig): follow up refactor with e2eutil.ConsulPolicy
func (tc *ConnectACLsE2ETest) createConsulPolicy(p consulPolicy, f *framework.F) string {
	result, _, err := tc.Consul().ACL().PolicyCreate(&consulapi.ACLPolicy{
		Name:        p.Name,
		Description: "test policy " + p.Name,
		Rules:       p.Rules,
	}, &consulapi.WriteOptions{Token: tc.consulManagementToken})
	f.NoError(err, "failed to create consul policy")
	tc.consulPolicyIDs = append(tc.consulPolicyIDs, result.ID)
	return result.ID
}

// todo(shoenig): follow up refactor with e2eutil.ConsulPolicy
func (tc *ConnectACLsE2ETest) createOperatorToken(policyID string, f *framework.F) string {
	token, _, err := tc.Consul().ACL().TokenCreate(&consulapi.ACLToken{
		Description: "operator token",
		Policies:    []*consulapi.ACLTokenPolicyLink{{ID: policyID}},
	}, &consulapi.WriteOptions{Token: tc.consulManagementToken})
	f.NoError(err, "failed to create operator token")
	tc.consulTokenIDs = append(tc.consulTokenIDs, token.AccessorID)
	return token.SecretID
}

func (tc *ConnectACLsE2ETest) TestConnectACLsRegisterMasterToken(f *framework.F) {
	t := f.T()

	t.Log("test register Connect job w/ ACLs enabled w/ master token")

	jobID := "connect" + uuid.Generate()[0:8]
	tc.jobIDs = append(tc.jobIDs, jobID)

	jobAPI := tc.Nomad().Jobs()

	job, err := jobspec.ParseFile(demoConnectJob)
	f.NoError(err)

	// Set the job file to use the consul master token.
	// One should never do this in practice, but, it should work.
	// https://www.consul.io/docs/acl/acl-system.html#builtin-tokens
	job.ConsulToken = &tc.consulManagementToken
	job.ID = &jobID

	// Avoid using Register here, because that would actually create and run the
	// Job which runs the task, creates the SI token, which all needs to be
	// given time to settle and cleaned up. That is all covered in the big slow
	// test at the bottom.
	resp, _, err := jobAPI.Plan(job, false, nil)
	f.NoError(err)
	f.NotNil(resp)
}

func (tc *ConnectACLsE2ETest) TestConnectACLsRegisterMissingOperatorToken(f *framework.F) {
	t := f.T()

	t.Skip("we don't have consul.allow_unauthenticated=false set because it would required updating every E2E test to pass a Consul token")

	t.Log("test register Connect job w/ ACLs enabled w/o operator token")

	jobID := "connect" + uuid.Short()
	tc.jobIDs = append(tc.jobIDs, jobID) // need to clean up if the test fails

	job, err := jobspec.ParseFile(demoConnectJob)
	f.NoError(err)
	jobAPI := tc.Nomad().Jobs()

	// Explicitly show the ConsulToken is not set
	job.ConsulToken = nil
	job.ID = &jobID

	_, _, err = jobAPI.Register(job, nil)
	f.Error(err)

	t.Log("job correctly rejected, with error:", err)
}

func (tc *ConnectACLsE2ETest) TestConnectACLsRegisterFakeOperatorToken(f *framework.F) {
	t := f.T()

	t.Skip("we don't have consul.allow_unauthenticated=false set because it would required updating every E2E test to pass a Consul token")

	t.Log("test register Connect job w/ ACLs enabled w/ operator token")

	policyID := tc.createConsulPolicy(consulPolicy{
		Name:  "nomad-operator-policy-" + uuid.Short(),
		Rules: `service "count-api" { policy = "write" } service "count-dashboard" { policy = "write" }`,
	}, f)
	t.Log("created operator policy:", policyID)

	// generate a fake consul token token
	fakeToken := uuid.Generate()

	jobID := "connect" + uuid.Short()
	tc.jobIDs = append(tc.jobIDs, jobID) // need to clean up if the test fails

	job := tc.parseJobSpecFile(t, demoConnectJob)

	jobAPI := tc.Nomad().Jobs()

	// deliberately set the fake Consul token
	job.ConsulToken = &fakeToken
	job.ID = &jobID

	// should fail, because the token is fake
	_, _, err := jobAPI.Register(job, nil)
	f.Error(err)
	t.Log("job correctly rejected, with error:", err)
}

func (tc *ConnectACLsE2ETest) TestConnectACLsConnectDemo(f *framework.F) {
	t := f.T()

	t.Log("test register Connect job w/ ACLs enabled w/ operator token")

	// === Setup ACL policy and mint Operator token ===

	// create a policy allowing writes of services "count-api" and "count-dashboard"
	policyID := tc.createConsulPolicy(consulPolicy{
		Name:  "nomad-operator-policy-" + uuid.Short(),
		Rules: `service "count-api" { policy = "write" } service "count-dashboard" { policy = "write" }`,
	}, f)
	t.Log("created operator policy:", policyID)

	// create a Consul "operator token" blessed with the above policy
	operatorToken := tc.createOperatorToken(policyID, f)
	t.Log("created operator token:", operatorToken)

	jobID := connectJobID()
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), demoConnectJob, jobID, operatorToken)
	f.Equal(2, len(allocs), "expected 2 allocs for connect demo", allocs)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	f.Equal(2, len(allocIDs), "expected 2 allocIDs for connect demo", allocIDs)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)

	// === Check Consul SI tokens were generated for sidecars ===
	foundSITokens := tc.countSITokens(t)
	f.Equal(2, len(foundSITokens), "expected 2 SI tokens total: %v", foundSITokens)
	f.Equal(1, foundSITokens["connect-proxy-count-api"], "expected 1 SI token for connect-proxy-count-api: %v", foundSITokens)
	f.Equal(1, foundSITokens["connect-proxy-count-dashboard"], "expected 1 SI token for connect-proxy-count-dashboard: %v", foundSITokens)

	t.Log("connect legacy job with ACLs enable finished")
}

func (tc *ConnectACLsE2ETest) TestConnectACLsConnectNativeDemo(f *framework.F) {
	t := f.T()

	t.Log("test register Connect job w/ ACLs enabled w/ operator token")

	// === Setup ACL policy and mint Operator token ===

	// create a policy allowing writes of services "uuid-fe" and "uuid-api"
	policyID := tc.createConsulPolicy(consulPolicy{
		Name:  "nomad-operator-policy-" + uuid.Short(),
		Rules: `service "uuid-fe" { policy = "write" } service "uuid-api" { policy = "write" }`,
	}, f)
	t.Log("created operator policy:", policyID)

	// create a Consul "operator token" blessed with the above policy
	operatorToken := tc.createOperatorToken(policyID, f)
	t.Log("created operator token:", operatorToken)

	jobID := connectJobID()
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), demoConnectNativeJob, jobID, operatorToken)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)

	// === Check Consul SI tokens were generated for native tasks ===
	foundSITokens := tc.countSITokens(t)
	f.Equal(2, len(foundSITokens), "expected 2 SI tokens total: %v", foundSITokens)
	f.Equal(1, foundSITokens["frontend"], "expected 1 SI token for frontend: %v", foundSITokens)
	f.Equal(1, foundSITokens["generate"], "expected 1 SI token for generate: %v", foundSITokens)

	t.Log("connect native job with ACLs enabled finished")
}

func (tc *ConnectACLsE2ETest) TestConnectACLsConnectIngressGatewayDemo(f *framework.F) {
	t := f.T()

	t.Log("test register Connect Ingress Gateway job w/ ACLs enabled")

	// setup ACL policy and mint operator token

	policyID := tc.createConsulPolicy(consulPolicy{
		Name:  "nomad-operator-policy-" + uuid.Short(),
		Rules: `service "my-ingress-service" { policy = "write" } service "uuid-api" { policy = "write" }`,
	}, f)
	operatorToken := tc.createOperatorToken(policyID, f)
	t.Log("created operator token:", operatorToken)

	jobID := connectJobID()
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), demoConnectIngressGateway, jobID, operatorToken)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)

	foundSITokens := tc.countSITokens(t)
	f.Equal(2, len(foundSITokens), "expected 2 SI tokens total: %v", foundSITokens)
	f.Equal(1, foundSITokens["connect-ingress-my-ingress-service"], "expected 1 SI token for connect-ingress-my-ingress-service: %v", foundSITokens)
	f.Equal(1, foundSITokens["generate"], "expected 1 SI token for generate: %v", foundSITokens)

	t.Log("connect ingress gateway job with ACLs enabled finished")
}

func (tc *ConnectACLsE2ETest) TestConnectACLsConnectTerminatingGatewayDemo(f *framework.F) {
	t := f.T()

	t.Log("test register Connect Terminating Gateway job w/ ACLs enabled")

	// setup ACL policy and mint operator token

	policyID := tc.createConsulPolicy(consulPolicy{
		Name:  "nomad-operator-policy-" + uuid.Short(),
		Rules: `service "api-gateway" { policy = "write" } service "count-dashboard" { policy = "write" }`,
	}, f)
	operatorToken := tc.createOperatorToken(policyID, f)
	t.Log("created operator token:", operatorToken)

	jobID := connectJobID()
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), demoConnectTerminatingGateway, jobID, operatorToken)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)

	foundSITokens := tc.countSITokens(t)
	f.Equal(2, len(foundSITokens), "expected 2 SI tokens total: %v", foundSITokens)
	f.Equal(1, foundSITokens["connect-terminating-api-gateway"], "expected 1 SI token for connect-terminating-api-gateway: %v", foundSITokens)
	f.Equal(1, foundSITokens["connect-proxy-count-dashboard"], "expected 1 SI token for count-dashboard: %v", foundSITokens)

	t.Log("connect terminating gateway job with ACLs enabled finished")
}

var (
	siTokenRe = regexp.MustCompile(`_nomad_si \[[\w-]{36}] \[[\w-]{36}] \[([\S]+)]`)
)

func (tc *ConnectACLsE2ETest) serviceofSIToken(description string) string {
	if m := siTokenRe.FindStringSubmatch(description); len(m) == 2 {
		return m[1]
	}
	return ""
}

func (tc *ConnectACLsE2ETest) countSITokens(t *testing.T) map[string]int {
	aclAPI := tc.Consul().ACL()
	tokens, _, err := aclAPI.TokenList(&consulapi.QueryOptions{
		Token: tc.consulManagementToken,
	})
	require.NoError(t, err)

	// count the number of SI tokens matching each service name
	foundSITokens := make(map[string]int)
	for _, token := range tokens {
		if service := tc.serviceofSIToken(token.Description); service != "" {
			foundSITokens[service]++
		}
	}

	return foundSITokens
}

func (tc *ConnectACLsE2ETest) parseJobSpecFile(t *testing.T, filename string) *nomadapi.Job {
	job, err := jobspec.ParseFile(filename)
	require.NoError(t, err)
	return job
}
