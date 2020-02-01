package connect

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	capi "github.com/hashicorp/consul/api"
	napi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/consulacls"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

const (
	// envConsulToken is the consul http token environment variable
	envConsulToken = "CONSUL_HTTP_TOKEN"

	// demoConnectJob is the example connect enabled job useful for testing
	demoConnectJob = "connect/input/demo.nomad"
)

type ConnectACLsE2ETest struct {
	framework.TC

	// manageConsulACLs is used to 'enable' and 'disable' Consul ACLs in the
	// Consul Cluster that has been setup for e2e testing.
	manageConsulACLs consulacls.Manager
	// consulMasterToken is set to the generated Consul ACL token after using
	// the consul-acls-manage.sh script to enable ACLs.
	consulMasterToken string

	// things to cleanup after each test case
	jobIDs          []string
	consulPolicyIDs []string
	consulTokenIDs  []string
}

func (tc *ConnectACLsE2ETest) BeforeAll(f *framework.F) {
	// Wait for Nomad to be ready before doing anything.
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 2)

	// Now enable Consul ACLs, the bootstrapping process for which will be
	// managed automatically if needed.
	var err error
	tc.manageConsulACLs, err = consulacls.New(consulacls.DefaultTFStateFile)
	require.NoError(f.T(), err)
	tc.enableConsulACLs(f)

	// Sanity check the consul master token exists, otherwise tests are just
	// going to be a train wreck.
	tokenLength := len(tc.consulMasterToken)
	require.Equal(f.T(), 36, tokenLength, "consul master token wrong length")

	// Sanity check the CONSUL_HTTP_TOKEN is NOT set, because that will cause
	// the agent checks to fail (which do not allow having a token set (!)).
	consulTokenEnv := os.Getenv(envConsulToken)
	require.Empty(f.T(), consulTokenEnv)

	// Wait for Nomad to be ready _again_, since everything was restarted during
	// the bootstrap process.
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 2)
}

// enableConsulACLs effectively executes `consul-acls-manage.sh enable`, which
// will activate Consul ACLs, going through the bootstrap process if necessary.
func (tc *ConnectACLsE2ETest) enableConsulACLs(f *framework.F) {
	tc.consulMasterToken = tc.manageConsulACLs.Enable(f.T())
}

// AfterAll runs after all tests are complete.
//
// We disable ConsulACLs in here to isolate the use of Consul ACLs only to
// test suites that explicitly want to test with them enabled.
func (tc *ConnectACLsE2ETest) AfterAll(f *framework.F) {
	tc.disableConsulACLs(f)
}

// disableConsulACLs effectively executes `consul-acls-manage.sh disable`, which
// will de-activate Consul ACLs.
func (tc *ConnectACLsE2ETest) disableConsulACLs(f *framework.F) {
	tc.manageConsulACLs.Disable(f.T())
}

// AfterEach does cleanup of Consul ACL objects that were created during each
// test case. Each test case may assume it is starting from a "fresh" state -
// as if the consul ACL bootstrap process had just taken place.
func (tc *ConnectACLsE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	t := f.T()
	r := require.New(t)

	// cleanup jobs
	for _, id := range tc.jobIDs {
		t.Log("cleanup: deregister nomad job id:", id)
		_, _, err := tc.Nomad().Jobs().Deregister(id, true, nil)
		r.NoError(err)
	}

	// cleanup consul tokens
	for _, id := range tc.consulTokenIDs {
		t.Log("cleanup: delete consul token id:", id)
		_, err := tc.Consul().ACL().TokenDelete(id, &capi.WriteOptions{Token: tc.consulMasterToken})
		r.NoError(err)
	}

	// cleanup consul policies
	for _, id := range tc.consulPolicyIDs {
		t.Log("cleanup: delete consul policy id:", id)
		_, err := tc.Consul().ACL().PolicyDelete(id, &capi.WriteOptions{Token: tc.consulMasterToken})
		r.NoError(err)
	}

	// do garbage collection
	err := tc.Nomad().System().GarbageCollect()
	r.NoError(err)

	// assert there are no leftover SI tokens, which may take a minute to be
	// cleaned up
	r.Eventually(func() bool {
		siTokens := tc.countSITokens(t)
		t.Log("cleanup: checking for remaining SI tokens:", siTokens)
		return len(siTokens) == 0
	}, 2*time.Minute, 2*time.Second, "SI tokens did not get removed")

	tc.jobIDs = []string{}
	tc.consulTokenIDs = []string{}
	tc.consulPolicyIDs = []string{}
}

type consulPolicy struct {
	Name  string // e.g. nomad-operator
	Rules string // e.g. service "" { policy="write" }
}

func (tc *ConnectACLsE2ETest) createConsulPolicy(p consulPolicy, f *framework.F) string {
	r := require.New(f.T())
	result, _, err := tc.Consul().ACL().PolicyCreate(&capi.ACLPolicy{
		Name:        p.Name,
		Description: "test policy " + p.Name,
		Rules:       p.Rules,
	}, &capi.WriteOptions{Token: tc.consulMasterToken})
	r.NoError(err, "failed to create consul policy")
	tc.consulPolicyIDs = append(tc.consulPolicyIDs, result.ID)
	return result.ID
}

func (tc *ConnectACLsE2ETest) createOperatorToken(policyID string, f *framework.F) string {
	r := require.New(f.T())
	token, _, err := tc.Consul().ACL().TokenCreate(&capi.ACLToken{
		Description: "operator token",
		Policies:    []*capi.ACLTokenPolicyLink{{ID: policyID}},
	}, &capi.WriteOptions{Token: tc.consulMasterToken})
	r.NoError(err, "failed to create operator token")
	tc.consulTokenIDs = append(tc.consulTokenIDs, token.AccessorID)
	return token.SecretID
}

func (tc *ConnectACLsE2ETest) TestConnectACLsRegisterMasterToken(f *framework.F) {
	t := f.T()
	r := require.New(t)

	t.Log("test register Connect job w/ ACLs enabled w/ master token")

	jobID := "connect" + uuid.Generate()[0:8]
	tc.jobIDs = append(tc.jobIDs, jobID)

	jobAPI := tc.Nomad().Jobs()

	job, err := jobspec.ParseFile(demoConnectJob)
	r.NoError(err)

	// Set the job file to use the consul master token.
	// One should never do this in practice, but, it should work.
	// https://www.consul.io/docs/acl/acl-system.html#builtin-tokens
	job.ConsulToken = &tc.consulMasterToken

	// Avoid using Register here, because that would actually create and run the
	// Job which runs the task, creates the SI token, which all needs to be
	// given time to settle and cleaned up. That is all covered in the big slow
	// test at the bottom.
	resp, _, err := jobAPI.Plan(job, false, nil)
	r.NoError(err)
	r.NotNil(resp)
	fmt.Println("resp:", resp)
}

func (tc *ConnectACLsE2ETest) TestConnectACLsRegisterMissingOperatorToken(f *framework.F) {
	t := f.T()
	r := require.New(t)

	t.Log("test register Connect job w/ ACLs enabled w/o operator token")

	job, err := jobspec.ParseFile(demoConnectJob)
	r.NoError(err)

	jobAPI := tc.Nomad().Jobs()

	// Explicitly show the ConsulToken is not set
	job.ConsulToken = nil

	_, _, err = jobAPI.Register(job, nil)
	r.Error(err)

	t.Log("job correctly rejected, with error:", err)
}

func (tc *ConnectACLsE2ETest) TestConnectACLsRegisterFakeOperatorToken(f *framework.F) {
	t := f.T()
	r := require.New(t)

	t.Log("test register Connect job w/ ACLs enabled w/ operator token")

	policyID := tc.createConsulPolicy(consulPolicy{
		Name:  "nomad-operator-policy",
		Rules: `service "count-api" { policy = "write" } service "count-dashboard" { policy = "write" }`,
	}, f)
	t.Log("created operator policy:", policyID)

	// generate a fake consul token token
	fakeToken := uuid.Generate()
	job := tc.parseJobSpecFile(t, demoConnectJob)

	jobAPI := tc.Nomad().Jobs()

	// deliberately set the fake Consul token
	job.ConsulToken = &fakeToken

	// should fail, because the token is fake
	_, _, err := jobAPI.Register(job, nil)
	r.Error(err)
	t.Log("job correctly rejected, with error:", err)
}

func (tc *ConnectACLsE2ETest) TestConnectACLsConnectDemo(f *framework.F) {
	t := f.T()
	r := require.New(t)

	t.Log("test register Connect job w/ ACLs enabled w/ operator token")

	// === Setup ACL policy and token ===

	// create a policy allowing writes of services "count-api" and "count-dashboard"
	policyID := tc.createConsulPolicy(consulPolicy{
		Name:  "nomad-operator-policy",
		Rules: `service "count-api" { policy = "write" } service "count-dashboard" { policy = "write" }`,
	}, f)
	t.Log("created operator policy:", policyID)

	// create a Consul "operator token" blessed with the above policy
	operatorToken := tc.createOperatorToken(policyID, f)
	t.Log("created operator token:", operatorToken)

	// === Register the Nomad job ===
	jobID := "connectACL_connect_demo"
	//{
	//
	//	nomadClient := tc.Nomad()
	//	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, demoConnectJob, jobID, operatorToken)
	//	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	//	e2eutil.WaitForAllocsRunning(t, nomadClient, allocIDs)
	//}

	var allocs []*napi.AllocationListStub
	allocIDs := make(map[string]bool, 2)
	{
		// jobID := "connect" + uuid.Generate()[0:8] (nicer name now)

		// parse the example connect jobspec file
		tc.jobIDs = append(tc.jobIDs, jobID)
		job := tc.parseJobSpecFile(t, demoConnectJob)
		job.ID = &jobID
		jobAPI := tc.Nomad().Jobs()

		// set the valid consul operator token
		job.ConsulToken = &operatorToken

		// registering the job should succeed
		resp, _, err := jobAPI.Register(job, nil)
		r.NoError(err)
		r.NotNil(resp)
		r.Empty(resp.Warnings)
		t.Log("job has been registered with evalID:", resp.EvalID)

		// === Make sure the evaluation actually succeeds ===
	EVAL:
		qOpts := &napi.QueryOptions{WaitIndex: resp.EvalCreateIndex}
		evalAPI := tc.Nomad().Evaluations()
		eval, qMeta, err := evalAPI.Info(resp.EvalID, qOpts)
		r.NoError(err)
		qOpts.WaitIndex = qMeta.LastIndex

		switch eval.Status {
		case "pending":
			goto EVAL
		case "complete":
		// ok!
		case "failed", "canceled", "blocked":
			r.Failf("eval %s\n%s\n", eval.Status, pretty.Sprint(eval))
		default:
			r.Failf("unknown eval status: %s\n%s\n", eval.Status, pretty.Sprint(eval))
		}

		// assert there were no placement failures
		r.Zero(eval.FailedTGAllocs, pretty.Sprint(eval.FailedTGAllocs))
		r.Len(eval.QueuedAllocations, 2, pretty.Sprint(eval.QueuedAllocations))

		// === Assert allocs are running ===
		// var allocs []*napi.AllocationListStub // move scope

		for i := 0; i < 20; i++ {
			allocs, qMeta, err = evalAPI.Allocations(eval.ID, qOpts)
			r.NoError(err)
			r.Len(allocs, 2)
			qOpts.WaitIndex = qMeta.LastIndex

			running := 0
			for _, alloc := range allocs {
				switch alloc.ClientStatus {
				case "running":
					running++
				case "pending":
					// keep trying
				default:
					r.Failf("alloc failed", "alloc: %s", pretty.Sprint(alloc))
				}
			}

			if running == len(allocs) {
				t.Log("running:", running, "allocs:", allocs)
				break
			}

			time.Sleep(500 * time.Millisecond)
		}

		for _, a := range allocs {
			if a.ClientStatus != "running" || a.DesiredStatus != "run" {
				r.Failf("terminal alloc", "alloc %s (%s) terminal; client=%s desired=%s", a.TaskGroup, a.ID, a.ClientStatus, a.DesiredStatus)
			}
			allocIDs[a.ID] = true
		}
	}

	// === Check Consul service health ===
	agentAPI := tc.Consul().Agent()

	failing := map[string]*capi.AgentCheck{}
	for i := 0; i < 60; i++ {
		checks, err := agentAPI.Checks()
		require.NoError(t, err)

		// filter out checks for other services
		for cid, check := range checks {
			found := false
			// for _, allocID := range allocIDs { // list
			for allocID := range allocIDs { // map
				if strings.Contains(check.ServiceID, allocID) {
					found = true
					break
				}
			}

			if !found {
				delete(checks, cid)
			}
		}

		// ensure checks are all passing
		failing = map[string]*capi.AgentCheck{}
		for _, check := range checks {
			if check.Status != "passing" {
				failing[check.CheckID] = check
				break
			}
		}

		if len(failing) == 0 {
			break
		}

		t.Logf("still %d checks not passing", len(failing))

		time.Sleep(time.Second)
	}

	require.Len(t, failing, 0, pretty.Sprint(failing))

	// === Check Consul SI tokens were generated for sidecars ===
	foundSITokens := tc.countSITokens(t)
	r.Equal(2, len(foundSITokens), "expected 2 SI tokens total: %v", foundSITokens)
	r.Equal(1, foundSITokens["connect-proxy-count-api"], "expected 1 SI token for connect-proxy-count-api: %v", foundSITokens)
	r.Equal(1, foundSITokens["connect-proxy-count-dashboard"], "expected 1 SI token for connect-proxy-count-dashboard: %v", foundSITokens)

	t.Log("connect job with ACLs enable finished")
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
	tokens, _, err := aclAPI.TokenList(&capi.QueryOptions{
		Token: tc.consulMasterToken,
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

func (tc *ConnectACLsE2ETest) parseJobSpecFile(t *testing.T, filename string) *napi.Job {
	job, err := jobspec.ParseFile(filename)
	require.NoError(t, err)
	return job
}
