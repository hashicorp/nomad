package nomad

import (
	"testing"

	"github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestServiceRegistration_Upsert(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		serverFn func(t *testing.T) (*Server, *structs.ACLToken, func())
		testFn   func(t *testing.T, s *Server, token *structs.ACLToken)
		name     string
	}{
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations and ensure
				// they are in the same namespace.
				services := mock.ServiceRegistrations()
				services[1].Namespace = services[0].Namespace

				// Attempt to upsert without a token.
				serviceRegReq := &structs.ServiceRegistrationUpsertRequest{
					Services: services,
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						Namespace: services[0].Namespace,
					},
				}
				var serviceRegResp structs.ServiceRegistrationUpsertResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationUpsertRPCMethod, serviceRegReq, &serviceRegResp)
				require.Error(t, err)
				require.Contains(t, err.Error(), "node lookup by SecretID failed")

				// Generate a node and retry the upsert.
				node := mock.Node()
				require.NoError(t, s.State().UpsertNode(structs.MsgTypeTestSetup, 10, node))

				ws := memdb.NewWatchSet()
				node, err = s.State().NodeByID(ws, node.ID)
				require.NoError(t, err)
				require.NotNil(t, node)

				serviceRegReq.WriteRequest.AuthToken = node.SecretID
				err = msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationUpsertRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.Greater(t, serviceRegResp.Index, uint64(1))
			},
			name: "ACLs disabled without node secret",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations and ensure
				// they are in the same namespace.
				services := mock.ServiceRegistrations()
				services[1].Namespace = services[0].Namespace

				// Generate a node.
				node := mock.Node()
				require.NoError(t, s.State().UpsertNode(structs.MsgTypeTestSetup, 10, node))

				ws := memdb.NewWatchSet()
				node, err := s.State().NodeByID(ws, node.ID)
				require.NoError(t, err)
				require.NotNil(t, node)

				serviceRegReq := &structs.ServiceRegistrationUpsertRequest{
					Services: services,
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						Namespace: services[0].Namespace,
						AuthToken: node.SecretID,
					},
				}
				var serviceRegResp structs.ServiceRegistrationUpsertResponse
				err = msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationUpsertRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.Greater(t, serviceRegResp.Index, uint64(1))
			},
			name: "ACLs disabled with node secret",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations and ensure
				// they are in the same namespace.
				services := mock.ServiceRegistrations()
				services[1].Namespace = services[0].Namespace

				// Attempt to upsert without a token.
				serviceRegReq := &structs.ServiceRegistrationUpsertRequest{
					Services: services,
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						Namespace: services[0].Namespace,
					},
				}
				var serviceRegResp structs.ServiceRegistrationUpsertResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationUpsertRPCMethod, serviceRegReq, &serviceRegResp)
				require.Error(t, err)
				require.Contains(t, err.Error(), "node lookup by SecretID failed")

				// Generate a node and retry the upsert.
				node := mock.Node()
				require.NoError(t, s.State().UpsertNode(structs.MsgTypeTestSetup, 10, node))

				ws := memdb.NewWatchSet()
				node, err = s.State().NodeByID(ws, node.ID)
				require.NoError(t, err)
				require.NotNil(t, node)

				serviceRegReq.WriteRequest.AuthToken = node.SecretID
				err = msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationUpsertRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.Greater(t, serviceRegResp.Index, uint64(1))
			},
			name: "ACLs enabled without node secret",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations and ensure
				// they are in the same namespace.
				services := mock.ServiceRegistrations()
				services[1].Namespace = services[0].Namespace

				// Generate a node.
				node := mock.Node()
				require.NoError(t, s.State().UpsertNode(structs.MsgTypeTestSetup, 10, node))

				ws := memdb.NewWatchSet()
				node, err := s.State().NodeByID(ws, node.ID)
				require.NoError(t, err)
				require.NotNil(t, node)

				serviceRegReq := &structs.ServiceRegistrationUpsertRequest{
					Services: services,
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						Namespace: services[0].Namespace,
						AuthToken: node.SecretID,
					},
				}
				var serviceRegResp structs.ServiceRegistrationUpsertResponse
				err = msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationUpsertRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.Greater(t, serviceRegResp.Index, uint64(1))
			},
			name: "ACLs enabled with node secret",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, aclToken, cleanup := tc.serverFn(t)
			defer cleanup()
			tc.testFn(t, server, aclToken)
		})
	}
}

func TestServiceRegistration_DeleteByID(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		serverFn func(t *testing.T) (*Server, *structs.ACLToken, func())
		testFn   func(t *testing.T, s *Server, token *structs.ACLToken)
		name     string
	}{
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Attempt to delete a service registration that does not
				// exist.
				serviceRegReq := &structs.ServiceRegistrationDeleteByIDRequest{
					ID: "this-is-not-the-service-you're-looking-for",
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						Namespace: "default",
					},
				}

				var serviceRegResp structs.ServiceRegistrationDeleteByIDResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationDeleteByIDRPCMethod, serviceRegReq, &serviceRegResp)
				require.Error(t, err)
				require.Contains(t, err.Error(), "service registration not found")
			},
			name: "ACLs disabled unknown service",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, services))

				// Try and delete one of the services that exist.
				serviceRegReq := &structs.ServiceRegistrationDeleteByIDRequest{
					ID: services[0].ID,
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						Namespace: services[0].Namespace,
					},
				}

				var serviceRegResp structs.ServiceRegistrationDeleteByIDResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationDeleteByIDRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
			},
			name: "ACLs disabled known service",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, services))

				// Try and delete one of the services that exist.
				serviceRegReq := &structs.ServiceRegistrationDeleteByIDRequest{
					ID: services[0].ID,
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						Namespace: services[0].Namespace,
						AuthToken: token.SecretID,
					},
				}

				var serviceRegResp structs.ServiceRegistrationDeleteByIDResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationDeleteByIDRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
			},
			name: "ACLs enabled known service with management token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, services))

				// Try and delete one of the services that exist but don't set
				// an auth token.
				serviceRegReq := &structs.ServiceRegistrationDeleteByIDRequest{
					ID: services[0].ID,
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						Namespace: services[0].Namespace,
					},
				}

				var serviceRegResp structs.ServiceRegistrationDeleteByIDResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationDeleteByIDRPCMethod, serviceRegReq, &serviceRegResp)
				require.Error(t, err)
				require.Contains(t, err.Error(), "Permission denied")
			},
			name: "ACLs enabled known service without token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, services))

				// Create a token using submit-job capability.
				authToken := mock.CreatePolicyAndToken(t, s.State(), 30, "test-service-reg-delete",
					mock.NamespacePolicy(services[0].Namespace, "", []string{acl.NamespaceCapabilitySubmitJob})).SecretID

				// Try and delete one of the services that exist but don't set
				// an auth token.
				serviceRegReq := &structs.ServiceRegistrationDeleteByIDRequest{
					ID: services[0].ID,
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						Namespace: services[0].Namespace,
						AuthToken: authToken,
					},
				}

				var serviceRegResp structs.ServiceRegistrationDeleteByIDResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationDeleteByIDRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
			},
			name: "ACLs enabled known service with submit-job namespace token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, services))

				// Create a token using submit-job capability.
				authToken := mock.CreatePolicyAndToken(t, s.State(), 30, "test-service-reg-delete",
					mock.NamespacePolicy(services[0].Namespace, "", []string{acl.NamespaceCapabilityReadJob})).SecretID

				// Try and delete one of the services that exist but don't set
				// an auth token.
				serviceRegReq := &structs.ServiceRegistrationDeleteByIDRequest{
					ID: services[0].ID,
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						Namespace: services[0].Namespace,
						AuthToken: authToken,
					},
				}

				var serviceRegResp structs.ServiceRegistrationDeleteByIDResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationDeleteByIDRPCMethod, serviceRegReq, &serviceRegResp)
				require.Error(t, err)
				require.Contains(t, err.Error(), "Permission denied")
			},
			name: "ACLs enabled known service with read-job namespace token",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, aclToken, cleanup := tc.serverFn(t)
			defer cleanup()
			tc.testFn(t, server, aclToken)
		})
	}
}

func TestServiceRegistration_List(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		serverFn func(t *testing.T) (*Server, *structs.ACLToken, func())
		testFn   func(t *testing.T, s *Server, token *structs.ACLToken)
		name     string
	}{
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, services))

				// Test a request without setting an ACL token.
				serviceRegReq := &structs.ServiceRegistrationListRequest{
					QueryOptions: structs.QueryOptions{
						Namespace: structs.AllNamespacesSentinel,
						Region:    DefaultRegion,
					},
				}
				var serviceRegResp structs.ServiceRegistrationListResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationListRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, []*structs.ServiceRegistrationListStub{
					{
						Namespace: "default",
						Services: []*structs.ServiceRegistrationStub{
							{
								ServiceName: "example-cache",
								Tags:        []string{"foo"},
							},
						}},
					{
						Namespace: "platform",
						Services: []*structs.ServiceRegistrationStub{
							{
								ServiceName: "countdash-api",
								Tags:        []string{"bar"},
							},
						}},
				}, serviceRegResp.Services)
			},
			name: "ACLs disabled wildcard ns",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, services))

				// Test a request without setting an ACL token.
				serviceRegReq := &structs.ServiceRegistrationListRequest{
					QueryOptions: structs.QueryOptions{
						Namespace: "platform",
						Region:    DefaultRegion,
					},
				}
				var serviceRegResp structs.ServiceRegistrationListResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationListRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, []*structs.ServiceRegistrationListStub{
					{
						Namespace: "platform",
						Services: []*structs.ServiceRegistrationStub{
							{
								ServiceName: "countdash-api",
								Tags:        []string{"bar"},
							},
						},
					},
				}, serviceRegResp.Services)
			},
			name: "ACLs disabled platform ns",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Test a request without setting an ACL token.
				serviceRegReq := &structs.ServiceRegistrationListRequest{
					QueryOptions: structs.QueryOptions{
						Namespace: "platform",
						Region:    DefaultRegion,
					},
				}
				var serviceRegResp structs.ServiceRegistrationListResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationListRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, []*structs.ServiceRegistrationListStub{}, serviceRegResp.Services)
			},
			name: "ACLs disabled no services",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, services))

				// Test a request without setting an ACL token.
				serviceRegReq := &structs.ServiceRegistrationListRequest{
					QueryOptions: structs.QueryOptions{
						Namespace: structs.AllNamespacesSentinel,
						Region:    DefaultRegion,
					},
				}
				var serviceRegResp structs.ServiceRegistrationListResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationListRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, []*structs.ServiceRegistrationListStub{}, serviceRegResp.Services)
			},
			name: "ACLs enabled wildcard ns without token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, services))

				// Test a request without setting an ACL token.
				serviceRegReq := &structs.ServiceRegistrationListRequest{
					QueryOptions: structs.QueryOptions{
						Namespace: "default",
						Region:    DefaultRegion,
					},
				}
				var serviceRegResp structs.ServiceRegistrationListResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationListRPCMethod, serviceRegReq, &serviceRegResp)
				require.Error(t, err)
				require.Contains(t, err.Error(), "Permission denied")
			},
			name: "ACLs enabled default ns without token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, services))

				// Test a request without setting an ACL token.
				serviceRegReq := &structs.ServiceRegistrationListRequest{
					QueryOptions: structs.QueryOptions{
						Namespace: structs.AllNamespacesSentinel,
						Region:    DefaultRegion,
						AuthToken: token.SecretID,
					},
				}
				var serviceRegResp structs.ServiceRegistrationListResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationListRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, []*structs.ServiceRegistrationListStub{
					{
						Namespace: "default",
						Services: []*structs.ServiceRegistrationStub{
							{
								ServiceName: "example-cache",
								Tags:        []string{"foo"},
							},
						}},
					{
						Namespace: "platform",
						Services: []*structs.ServiceRegistrationStub{
							{
								ServiceName: "countdash-api",
								Tags:        []string{"bar"},
							},
						}},
				}, serviceRegResp.Services)
			},
			name: "ACLs enabled wildcard with management token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, services))

				// Test a request without setting an ACL token.
				serviceRegReq := &structs.ServiceRegistrationListRequest{
					QueryOptions: structs.QueryOptions{
						Namespace: "default",
						Region:    DefaultRegion,
						AuthToken: token.SecretID,
					},
				}
				var serviceRegResp structs.ServiceRegistrationListResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationListRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, []*structs.ServiceRegistrationListStub{
					{
						Namespace: "default",
						Services: []*structs.ServiceRegistrationStub{
							{
								ServiceName: "example-cache",
								Tags:        []string{"foo"},
							},
						}},
				}, serviceRegResp.Services)
			},
			name: "ACLs enabled default ns with management token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Create a policy and grab the token which has the read-job
				// capability on the platform namespace.
				customToken := mock.CreatePolicyAndToken(t, s.State(), 5, "test-valid-autoscaler",
					mock.NamespacePolicy("platform", "", []string{acl.NamespaceCapabilityReadJob})).SecretID

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, services))

				// Test a request without setting an ACL token.
				serviceRegReq := &structs.ServiceRegistrationListRequest{
					QueryOptions: structs.QueryOptions{
						Namespace: "platform",
						Region:    DefaultRegion,
						AuthToken: customToken,
					},
				}
				var serviceRegResp structs.ServiceRegistrationListResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationListRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, []*structs.ServiceRegistrationListStub{
					{
						Namespace: "platform",
						Services: []*structs.ServiceRegistrationStub{
							{
								ServiceName: "countdash-api",
								Tags:        []string{"bar"},
							},
						}},
				}, serviceRegResp.Services)
			},
			name: "ACLs enabled with read-job policy token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Create a namespace as this is needed when using an ACL like
				// we do in this test.
				ns := &structs.Namespace{
					Name:        "platform",
					Description: "test namespace",
					CreateIndex: 5,
					ModifyIndex: 5,
				}
				ns.SetHash()
				require.NoError(t, s.State().UpsertNamespaces(5, []*structs.Namespace{ns}))

				// Create a policy and grab the token which has the read-job
				// capability on the platform namespace.
				customToken := mock.CreatePolicyAndToken(t, s.State(), 10, "test-valid-autoscaler",
					mock.NamespacePolicy("platform", "", []string{acl.NamespaceCapabilityReadJob})).SecretID

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 20, services))

				// Test a request without setting an ACL token.
				serviceRegReq := &structs.ServiceRegistrationListRequest{
					QueryOptions: structs.QueryOptions{
						Namespace: structs.AllNamespacesSentinel,
						Region:    DefaultRegion,
						AuthToken: customToken,
					},
				}
				var serviceRegResp structs.ServiceRegistrationListResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationListRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, []*structs.ServiceRegistrationListStub{
					{
						Namespace: "platform",
						Services: []*structs.ServiceRegistrationStub{
							{
								ServiceName: "countdash-api",
								Tags:        []string{"bar"},
							},
						}},
				}, serviceRegResp.Services)
			},
			name: "ACLs enabled wildcard ns with restricted token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Create a namespace as this is needed when using an ACL like
				// we do in this test.
				ns := &structs.Namespace{
					Name:        "platform",
					Description: "test namespace",
					CreateIndex: 5,
					ModifyIndex: 5,
				}
				ns.SetHash()
				require.NoError(t, s.State().UpsertNamespaces(5, []*structs.Namespace{ns}))

				// Create a policy and grab the token which has the read policy
				// on the platform namespace.
				customToken := mock.CreatePolicyAndToken(t, s.State(), 10, "test-valid-autoscaler",
					mock.NamespacePolicy("platform", "read", nil)).SecretID

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 20, services))

				// Test a request without setting an ACL token.
				serviceRegReq := &structs.ServiceRegistrationListRequest{
					QueryOptions: structs.QueryOptions{
						Namespace: structs.AllNamespacesSentinel,
						Region:    DefaultRegion,
						AuthToken: customToken,
					},
				}
				var serviceRegResp structs.ServiceRegistrationListResponse
				err := msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationListRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, []*structs.ServiceRegistrationListStub{
					{
						Namespace: "platform",
						Services: []*structs.ServiceRegistrationStub{
							{
								ServiceName: "countdash-api",
								Tags:        []string{"bar"},
							},
						}},
				}, serviceRegResp.Services)
			},
			name: "ACLs enabled with read namespace policy token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Create a namespace as this is needed when using an ACL like
				// we do in this test.
				ns := &structs.Namespace{
					Name:        "platform",
					Description: "test namespace",
					CreateIndex: 5,
					ModifyIndex: 5,
				}
				ns.SetHash()
				require.NoError(t, s.State().UpsertNamespaces(5, []*structs.Namespace{ns}))

				// Generate a node.
				node := mock.Node()
				require.NoError(t, s.State().UpsertNode(structs.MsgTypeTestSetup, 10, node))

				ws := memdb.NewWatchSet()
				node, err := s.State().NodeByID(ws, node.ID)
				require.NoError(t, err)
				require.NotNil(t, node)

				// Generate and upsert some service registrations.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 20, services))

				// Test a request while setting the auth token to the node
				// secret ID.
				serviceRegReq := &structs.ServiceRegistrationListRequest{
					QueryOptions: structs.QueryOptions{
						Namespace: "platform",
						Region:    DefaultRegion,
						AuthToken: node.SecretID,
					},
				}
				var serviceRegResp structs.ServiceRegistrationListResponse
				err = msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationListRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, []*structs.ServiceRegistrationListStub{
					{
						Namespace: "platform",
						Services: []*structs.ServiceRegistrationStub{
							{
								ServiceName: "countdash-api",
								Tags:        []string{"bar"},
							},
						}},
				}, serviceRegResp.Services)
			},
			name: "ACLs enabled with node secret toekn",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, aclToken, cleanup := tc.serverFn(t)
			defer cleanup()
			tc.testFn(t, server, aclToken)
		})
	}
}

func TestServiceRegistration_GetService(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		serverFn func(t *testing.T) (*Server, *structs.ACLToken, func())
		testFn   func(t *testing.T, s *Server, token *structs.ACLToken)
		name     string
	}{
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate mock services then upsert them individually using different indexes.
				services := mock.ServiceRegistrations()

				require.NoError(t, s.fsm.State().UpsertServiceRegistrations(
					structs.MsgTypeTestSetup, 10, []*structs.ServiceRegistration{services[0]}))

				require.NoError(t, s.fsm.State().UpsertServiceRegistrations(
					structs.MsgTypeTestSetup, 20, []*structs.ServiceRegistration{services[1]}))

				// Lookup the first registration.
				serviceRegReq := &structs.ServiceRegistrationByNameRequest{
					ServiceName: services[0].ServiceName,
					QueryOptions: structs.QueryOptions{
						Namespace: services[0].Namespace,
						Region:    s.Region(),
					},
				}
				var serviceRegResp structs.ServiceRegistrationByNameResponse
				err := msgpackrpc.CallWithCodec(codec, structs.ServiceRegistrationGetServiceRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.Equal(t, uint64(10), serviceRegResp.Services[0].CreateIndex)
				require.Equal(t, uint64(20), serviceRegResp.Index)
				require.Len(t, serviceRegResp.Services, 1)

				// Lookup the second registration.
				serviceRegReq2 := &structs.ServiceRegistrationByNameRequest{
					ServiceName: services[1].ServiceName,
					QueryOptions: structs.QueryOptions{
						Namespace: services[1].Namespace,
						Region:    s.Region(),
					},
				}
				var serviceRegResp2 structs.ServiceRegistrationByNameResponse
				err = msgpackrpc.CallWithCodec(codec, structs.ServiceRegistrationGetServiceRPCMethod, serviceRegReq2, &serviceRegResp2)
				require.NoError(t, err)
				require.Equal(t, uint64(20), serviceRegResp2.Services[0].CreateIndex)
				require.Equal(t, uint64(20), serviceRegResp.Index)
				require.Len(t, serviceRegResp2.Services, 1)

				// Perform a lookup with namespace and service name that shouldn't produce
				// results.
				serviceRegReq3 := &structs.ServiceRegistrationByNameRequest{
					ServiceName: services[0].ServiceName,
					QueryOptions: structs.QueryOptions{
						Namespace: services[1].Namespace,
						Region:    s.Region(),
					},
				}
				var serviceRegResp3 structs.ServiceRegistrationByNameResponse
				err = msgpackrpc.CallWithCodec(codec, structs.ServiceRegistrationGetServiceRPCMethod, serviceRegReq3, &serviceRegResp3)
				require.NoError(t, err)
				require.Len(t, serviceRegResp3.Services, 0)
			},
			name: "ACLs disabled",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate mock services then upsert them individually using different indexes.
				services := mock.ServiceRegistrations()

				require.NoError(t, s.fsm.State().UpsertServiceRegistrations(
					structs.MsgTypeTestSetup, 10, []*structs.ServiceRegistration{services[0]}))

				require.NoError(t, s.fsm.State().UpsertServiceRegistrations(
					structs.MsgTypeTestSetup, 20, []*structs.ServiceRegistration{services[1]}))

				// Lookup the first registration without using an ACL token
				// which should fail.
				serviceRegReq := &structs.ServiceRegistrationByNameRequest{
					ServiceName: services[0].ServiceName,
					QueryOptions: structs.QueryOptions{
						Namespace: services[0].Namespace,
						Region:    s.Region(),
					},
				}
				var serviceRegResp structs.ServiceRegistrationByNameResponse
				err := msgpackrpc.CallWithCodec(codec, structs.ServiceRegistrationGetServiceRPCMethod, serviceRegReq, &serviceRegResp)
				require.Error(t, err)
				require.Contains(t, err.Error(), "Permission denied")

				// Lookup the first registration using the management token.
				serviceRegReq2 := &structs.ServiceRegistrationByNameRequest{
					ServiceName: services[0].ServiceName,
					QueryOptions: structs.QueryOptions{
						Namespace: services[0].Namespace,
						Region:    s.Region(),
						AuthToken: token.SecretID,
					},
				}
				var serviceRegResp2 structs.ServiceRegistrationByNameResponse
				err = msgpackrpc.CallWithCodec(codec, structs.ServiceRegistrationGetServiceRPCMethod, serviceRegReq2, &serviceRegResp2)
				require.Nil(t, err)
				require.Len(t, serviceRegResp2.Services, 1)
				require.EqualValues(t, 20, serviceRegResp2.Index)

				// Create a read policy for the default namespace and test this
				// can correctly read the first service.
				authToken1 := mock.CreatePolicyAndToken(t, s.State(), 30, "test-service-reg-get",
					mock.NamespacePolicy(structs.DefaultNamespace, "read", nil)).SecretID
				serviceRegReq3 := &structs.ServiceRegistrationByNameRequest{
					ServiceName: services[0].ServiceName,
					QueryOptions: structs.QueryOptions{
						Namespace: services[0].Namespace,
						Region:    s.Region(),
						AuthToken: authToken1,
					},
				}
				var serviceRegResp3 structs.ServiceRegistrationByNameResponse
				err = msgpackrpc.CallWithCodec(codec, structs.ServiceRegistrationGetServiceRPCMethod, serviceRegReq3, &serviceRegResp3)
				require.Nil(t, err)
				require.Len(t, serviceRegResp3.Services, 1)
				require.EqualValues(t, 20, serviceRegResp2.Index)

				// Attempting to lookup services in a different namespace should fail.
				serviceRegReq4 := &structs.ServiceRegistrationByNameRequest{
					ServiceName: services[1].ServiceName,
					QueryOptions: structs.QueryOptions{
						Namespace: services[1].Namespace,
						Region:    s.Region(),
						AuthToken: authToken1,
					},
				}
				var serviceRegResp4 structs.ServiceRegistrationByNameResponse
				err = msgpackrpc.CallWithCodec(codec, structs.ServiceRegistrationGetServiceRPCMethod, serviceRegReq4, &serviceRegResp4)
				require.Error(t, err)
				require.Contains(t, err.Error(), "Permission denied")
			},
			name: "ACLs enabled",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate mock services then upsert them individually using different indexes.
				services := mock.ServiceRegistrations()

				require.NoError(t, s.fsm.State().UpsertServiceRegistrations(
					structs.MsgTypeTestSetup, 10, []*structs.ServiceRegistration{services[0]}))

				require.NoError(t, s.fsm.State().UpsertServiceRegistrations(
					structs.MsgTypeTestSetup, 20, []*structs.ServiceRegistration{services[1]}))

				// Generate a node.
				node := mock.Node()
				require.NoError(t, s.State().UpsertNode(structs.MsgTypeTestSetup, 30, node))

				ws := memdb.NewWatchSet()
				node, err := s.State().NodeByID(ws, node.ID)
				require.NoError(t, err)
				require.NotNil(t, node)

				// Test a request while setting the auth token to the node
				// secret ID.
				serviceRegReq := &structs.ServiceRegistrationListRequest{
					QueryOptions: structs.QueryOptions{
						Namespace: "platform",
						Region:    DefaultRegion,
						AuthToken: node.SecretID,
					},
				}
				var serviceRegResp structs.ServiceRegistrationListResponse
				err = msgpackrpc.CallWithCodec(
					codec, structs.ServiceRegistrationListRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, []*structs.ServiceRegistrationListStub{
					{
						Namespace: "platform",
						Services: []*structs.ServiceRegistrationStub{
							{
								ServiceName: "countdash-api",
								Tags:        []string{"bar"},
							},
						}},
				}, serviceRegResp.Services)
			},
			name: "ACLs enabled using node secret",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, aclToken, cleanup := tc.serverFn(t)
			defer cleanup()
			tc.testFn(t, server, aclToken)
		})
	}
}
