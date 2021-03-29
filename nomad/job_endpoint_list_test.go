package nomad

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func benchmarkList(b *testing.B, namespaces, jobs int) {
	runtime.GC()
	b.Logf("Starting %d %d", namespaces, jobs)
	b.Logf("at start memory: %v", memUsage())
	start := time.Now()
	s1, root, cleanupS1 := TestACLServer(b, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()

	codec := rpcClient(b, s1)
	testutil.WaitForLeader(b, s1.RPC)

	nextIdx := uint64(1000)

	nses := make([]*structs.Namespace, 0, namespaces)
	for i := 0; i < namespaces; i++ {
		nses = append(nses, mock.Namespace())
	}
	s1.fsm.State().UpsertNamespaces(nextIdx, nses)

	for i := 0; i < jobs; i++ {
		nextIdx++

		job := mock.Job()
		job.Namespace = nses[i%namespaces].Name
		state := s1.fsm.State()
		err := state.UpsertJob(structs.MsgTypeTestSetup, nextIdx, job)
		require.NoError(b, err)
	}
	b.Logf("job insertion %v: %v", jobs, time.Since(start))

	singleNsToken := mock.CreatePolicyAndToken(b, s1.fsm.State(), nextIdx, "single-ns-reader",
		mock.NamespacePolicy(nses[0].Name, "", []string{acl.NamespaceCapabilityListJobs}))

	singleNsTokenJobs := jobs / namespaces
	if jobs%namespaces != 0 {
		singleNsTokenJobs++
	}
	b.Logf("after job creation: %v", memUsage())

	nonMatchingPrefix := "zzzzzzzzz"

	cases := []struct {
		name      string
		token     *structs.ACLToken
		namespace string
		prefix    string

		expected int
	}{
		{
			name:      "root:all",
			token:     root,
			namespace: "*",
			prefix:    "",
			expected:  jobs,
		},
		{
			name:      "root:none",
			token:     root,
			namespace: "*",
			prefix:    nonMatchingPrefix,
			expected:  0,
		},
		{
			name:      "single_ns:all",
			token:     singleNsToken,
			namespace: "*",
			prefix:    "",
			expected:  singleNsTokenJobs,
		},
		{
			name:      "single_ns:none",
			token:     singleNsToken,
			namespace: "*",
			prefix:    nonMatchingPrefix,
			expected:  0,
		},
		{
			name:      "single_ns_indexed:all",
			token:     singleNsToken,
			namespace: nses[0].Name,
			prefix:    "",
			expected:  singleNsTokenJobs,
		},
		{
			name:      "single_ns_indexed:none",
			token:     singleNsToken,
			namespace: nses[0].Name,
			prefix:    nonMatchingPrefix,
			expected:  0,
		},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				// Lookup the jobs
				get := &structs.JobListRequest{
					QueryOptions: structs.QueryOptions{
						Region:    "global",
						Namespace: c.namespace,
						AuthToken: c.token.SecretID,
					},
				}
				get.Prefix = c.prefix
				var resp2 structs.JobListResponse
				err := msgpackrpc.CallWithCodec(codec, "Job.List", get, &resp2)
				require.NoError(b, err)
				require.Len(b, resp2.Jobs, c.expected)
			}
		})

	}

}

func BenchmarkList(b *testing.B) {
	cases := []struct {
		nses int
		jobs int
	}{
		{4, 100},
		{4, 1_000},
		{4, 100_000},
		{8, 100_000},
		{8, 1_000_000},
		{8, 3_000_000},
		{1000, 1_000_000},
		{1000, 3_000_000},
	}

	for _, c := range cases {
		b.Run(fmt.Sprintf("%d_%d", c.nses, c.jobs), func(b *testing.B) {
			benchmarkList(b, c.nses, c.jobs)
		})
	}
}

func memUsage() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return fmt.Sprintf(
		"Alloc = %v MiB\tTotalAlloc = %v MiB\tSys = %v MiB\tNumGC = %v",
		m.Alloc/1024/1024,
		m.TotalAlloc/1024/1024,
		m.Sys/1024/1024,
		m.NumGC)
}
