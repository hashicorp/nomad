// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/raft"

	"github.com/hashicorp/nomad/api"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// OperatorRequest is used route operator/raft API requests to the implementing
// functions.
func (s *HTTPServer) OperatorRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/operator/raft/")
	switch {
	case strings.HasPrefix(path, "configuration"):
		return s.OperatorRaftConfiguration(resp, req)
	case strings.HasPrefix(path, "peer"):
		return s.OperatorRaftPeer(resp, req)
	case strings.HasPrefix(path, "transfer-leadership"):
		return s.OperatorRaftTransferLeadership(resp, req)
	default:
		return nil, CodedError(404, ErrInvalidMethod)
	}
}

// OperatorRaftConfiguration is used to inspect the current Raft configuration.
// This supports the stale query mode in case the cluster doesn't have a leader.
func (s *HTTPServer) OperatorRaftConfiguration(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return nil, nil
	}

	var args structs.GenericRequest
	if done := s.parse(resp, req, &args.Region, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.RaftConfigurationResponse
	if err := s.agent.RPC("Operator.RaftGetConfiguration", &args, &reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// OperatorRaftPeer supports actions on Raft peers.
func (s *HTTPServer) OperatorRaftPeer(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodDelete {
		return nil, CodedError(404, ErrInvalidMethod)
	}

	params := req.URL.Query()
	_, hasID := params["id"]
	_, hasAddress := params["address"]

	if hasAddress {
		return nil, CodedError(http.StatusBadRequest, "Removing a peer by address is not supported in the current raft protocol version")
	}
	if !hasID {
		return nil, CodedError(http.StatusBadRequest, "Must specify the peer's ID to remove")
	}

	var args structs.RaftPeerByIDRequest
	s.parseWriteRequest(req, &args.WriteRequest)

	var reply struct{}
	args.ID = raft.ServerID(params.Get("id"))
	if err := s.agent.RPC("Operator.RaftRemovePeerByID", &args, &reply); err != nil {
		return nil, err
	}

	return nil, nil
}

// OperatorRaftTransferLeadership supports actions on Raft peers.
func (s *HTTPServer) OperatorRaftTransferLeadership(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodPost && req.Method != http.MethodPut {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	params := req.URL.Query()

	// Using the params map directly
	id, hasID := params["id"]
	addr, hasAddress := params["address"]

	// There are some items that we can parse for here that are more unwieldy in
	// the Validate() func on the RPC request object, like repeated query params.
	switch {
	case !hasID && !hasAddress:
		return nil, CodedError(http.StatusBadRequest, "must specify id or address")
	case hasID && hasAddress:
		return nil, CodedError(http.StatusBadRequest, "must specify either id or address")
	case hasID && id[0] == "":
		return nil, CodedError(http.StatusBadRequest, "id must be non-empty")
	case hasID && len(id) > 1:
		return nil, CodedError(http.StatusBadRequest, "must specify only one id")
	case hasAddress && addr[0] == "":
		return nil, CodedError(http.StatusBadRequest, "address must be non-empty")
	case hasAddress && len(addr) > 1:
		return nil, CodedError(http.StatusBadRequest, "must specify only one address")
	}

	var out structs.LeadershipTransferResponse
	args := &structs.RaftPeerRequest{}
	s.parseWriteRequest(req, &args.WriteRequest)

	if hasID {
		args.ID = raft.ServerID(id[0])
	} else {
		args.Address = raft.ServerAddress(addr[0])
	}

	if err := args.Validate(); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	err := s.agent.RPC("Operator.TransferLeadershipToPeer", &args, &out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// OperatorAutopilotConfiguration is used to inspect the current Autopilot configuration.
// This supports the stale query mode in case the cluster doesn't have a leader.
func (s *HTTPServer) OperatorAutopilotConfiguration(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Switch on the method
	switch req.Method {
	case http.MethodGet:
		var args structs.GenericRequest
		if done := s.parse(resp, req, &args.Region, &args.QueryOptions); done {
			return nil, nil
		}

		var reply structs.AutopilotConfig
		if err := s.agent.RPC("Operator.AutopilotGetConfiguration", &args, &reply); err != nil {
			return nil, err
		}

		out := api.AutopilotConfiguration{
			CleanupDeadServers:      reply.CleanupDeadServers,
			LastContactThreshold:    reply.LastContactThreshold,
			MaxTrailingLogs:         reply.MaxTrailingLogs,
			MinQuorum:               reply.MinQuorum,
			ServerStabilizationTime: reply.ServerStabilizationTime,
			EnableRedundancyZones:   reply.EnableRedundancyZones,
			DisableUpgradeMigration: reply.DisableUpgradeMigration,
			EnableCustomUpgrades:    reply.EnableCustomUpgrades,
			CreateIndex:             reply.CreateIndex,
			ModifyIndex:             reply.ModifyIndex,
		}

		return out, nil

	case http.MethodPut:
		var args structs.AutopilotSetConfigRequest
		s.parseWriteRequest(req, &args.WriteRequest)

		var conf api.AutopilotConfiguration
		if err := decodeBody(req, &conf); err != nil {
			return nil, CodedError(http.StatusBadRequest, fmt.Sprintf("Error parsing autopilot config: %v", err))
		}

		args.Config = structs.AutopilotConfig{
			CleanupDeadServers:      conf.CleanupDeadServers,
			LastContactThreshold:    conf.LastContactThreshold,
			MaxTrailingLogs:         conf.MaxTrailingLogs,
			MinQuorum:               conf.MinQuorum,
			ServerStabilizationTime: conf.ServerStabilizationTime,
			EnableRedundancyZones:   conf.EnableRedundancyZones,
			DisableUpgradeMigration: conf.DisableUpgradeMigration,
			EnableCustomUpgrades:    conf.EnableCustomUpgrades,
		}

		// Check for cas value
		params := req.URL.Query()
		if _, ok := params["cas"]; ok {
			casVal, err := strconv.ParseUint(params.Get("cas"), 10, 64)
			if err != nil {
				return nil, CodedError(http.StatusBadRequest, fmt.Sprintf("Error parsing cas value: %v", err))
			}
			args.Config.ModifyIndex = casVal
			args.CAS = true
		}

		var reply bool
		if err := s.agent.RPC("Operator.AutopilotSetConfiguration", &args, &reply); err != nil {
			return nil, err
		}

		// Only use the out value if this was a CAS
		if !args.CAS {
			return true, nil
		}
		return reply, nil

	default:
		return nil, CodedError(404, ErrInvalidMethod)
	}
}

// OperatorServerHealth is used to get the health of the servers in the given Region.
func (s *HTTPServer) OperatorServerHealth(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(404, ErrInvalidMethod)
	}

	var args structs.GenericRequest
	if done := s.parse(resp, req, &args.Region, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.OperatorHealthReply
	if err := s.agent.RPC("Operator.ServerHealth", &args, &reply); err != nil {
		return nil, err
	}

	// Reply with status 429 if something is unhealthy
	if !reply.Healthy {
		resp.WriteHeader(http.StatusTooManyRequests)
	}

	out := &api.OperatorHealthReply{
		Healthy:          reply.Healthy,
		FailureTolerance: reply.FailureTolerance,
		Voters:           reply.Voters,
		Leader:           reply.Leader,
	}
	for _, server := range reply.Servers {
		out.Servers = append(out.Servers, api.ServerHealth{
			ID:          server.ID,
			Name:        server.Name,
			Address:     server.Address,
			Version:     server.Version,
			Leader:      server.Leader,
			SerfStatus:  server.SerfStatus.String(),
			LastContact: server.LastContact,
			LastTerm:    server.LastTerm,
			LastIndex:   server.LastIndex,
			Healthy:     server.Healthy,
			Voter:       server.Voter,
			StableSince: server.StableSince.Round(time.Second).UTC(),
		})
	}

	// Modify the reply to include Enterprise response
	autopilotToAPIEntState(reply, out)

	return out, nil
}

// OperatorSchedulerConfiguration is used to inspect the current Scheduler configuration.
// This supports the stale query mode in case the cluster doesn't have a leader.
func (s *HTTPServer) OperatorSchedulerConfiguration(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Switch on the method
	switch req.Method {
	case http.MethodGet:
		return s.schedulerGetConfig(resp, req)

	case http.MethodPut, http.MethodPost:
		return s.schedulerUpdateConfig(resp, req)

	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) schedulerGetConfig(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.GenericRequest
	if done := s.parse(resp, req, &args.Region, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.SchedulerConfigurationResponse
	if err := s.agent.RPC("Operator.SchedulerGetConfiguration", &args, &reply); err != nil {
		return nil, err
	}
	setMeta(resp, &reply.QueryMeta)

	return reply, nil
}

func (s *HTTPServer) schedulerUpdateConfig(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.SchedulerSetConfigRequest
	s.parseWriteRequest(req, &args.WriteRequest)

	var conf api.SchedulerConfiguration
	if err := decodeBody(req, &conf); err != nil {
		return nil, CodedError(http.StatusBadRequest, fmt.Sprintf("Error parsing scheduler config: %v", err))
	}

	args.Config = structs.SchedulerConfiguration{
		SchedulerAlgorithm:            structs.SchedulerAlgorithm(conf.SchedulerAlgorithm),
		MemoryOversubscriptionEnabled: conf.MemoryOversubscriptionEnabled,
		RejectJobRegistration:         conf.RejectJobRegistration,
		PauseEvalBroker:               conf.PauseEvalBroker,
		PreemptionConfig: structs.PreemptionConfig{
			SystemSchedulerEnabled:   conf.PreemptionConfig.SystemSchedulerEnabled,
			SysBatchSchedulerEnabled: conf.PreemptionConfig.SysBatchSchedulerEnabled,
			BatchSchedulerEnabled:    conf.PreemptionConfig.BatchSchedulerEnabled,
			ServiceSchedulerEnabled:  conf.PreemptionConfig.ServiceSchedulerEnabled,
		},
	}

	if err := args.Config.Validate(); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	// Check for cas value
	params := req.URL.Query()
	if _, ok := params["cas"]; ok {
		casVal, err := strconv.ParseUint(params.Get("cas"), 10, 64)
		if err != nil {
			return nil, CodedError(http.StatusBadRequest, fmt.Sprintf("Error parsing cas value: %v", err))
		}
		args.Config.ModifyIndex = casVal
		args.CAS = true
	}

	var reply structs.SchedulerSetConfigurationResponse
	if err := s.agent.RPC("Operator.SchedulerSetConfiguration", &args, &reply); err != nil {
		return nil, err
	}
	setIndex(resp, reply.Index)
	return reply, nil
}

func (s *HTTPServer) SnapshotRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case http.MethodGet:
		return s.snapshotSaveRequest(resp, req)
	case http.MethodPut, http.MethodPost:
		return s.snapshotRestoreRequest(resp, req)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}

}

func (s *HTTPServer) snapshotSaveRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := &structs.SnapshotSaveRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var handler structs.StreamingRpcHandler
	var handlerErr error

	if server := s.agent.Server(); server != nil {
		handler, handlerErr = server.StreamingRpcHandler("Operator.SnapshotSave")
	} else if client := s.agent.Client(); client != nil {
		handler, handlerErr = client.RemoteStreamingRpcHandler("Operator.SnapshotSave")
	} else {
		handlerErr = fmt.Errorf("misconfigured connection")
	}

	if handlerErr != nil {
		return nil, CodedError(500, handlerErr.Error())
	}

	httpPipe, handlerPipe := net.Pipe()
	decoder := codec.NewDecoder(httpPipe, structs.MsgpackHandle)
	encoder := codec.NewEncoder(httpPipe, structs.MsgpackHandle)

	// Create a goroutine that closes the pipe if the connection closes.
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	go func() {
		<-ctx.Done()
		httpPipe.Close()
	}()

	errCh := make(chan HTTPCodedError, 2)
	go func() {
		defer cancel()

		// Send the request
		if err := encoder.Encode(args); err != nil {
			errCh <- CodedError(500, err.Error())
			return
		}

		var res structs.SnapshotSaveResponse
		if err := decoder.Decode(&res); err != nil {
			errCh <- CodedError(500, err.Error())
			return
		}

		if res.ErrorMsg != "" {
			errCh <- CodedError(res.ErrorCode, res.ErrorMsg)
			return
		}

		resp.Header().Add("Digest", res.SnapshotChecksum)

		_, err := io.Copy(resp, httpPipe)
		if err != nil &&
			err != io.EOF &&
			!strings.Contains(err.Error(), "closed") &&
			!strings.Contains(err.Error(), "EOF") {
			errCh <- CodedError(500, err.Error())
			return
		}

		errCh <- nil
	}()

	handler(handlerPipe)
	cancel()
	codedErr := <-errCh

	return nil, codedErr
}

func (s *HTTPServer) snapshotRestoreRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := &structs.SnapshotRestoreRequest{}
	s.parseWriteRequest(req, &args.WriteRequest)

	var handler structs.StreamingRpcHandler
	var handlerErr error

	if server := s.agent.Server(); server != nil {
		handler, handlerErr = server.StreamingRpcHandler("Operator.SnapshotRestore")
	} else if client := s.agent.Client(); client != nil {
		handler, handlerErr = client.RemoteStreamingRpcHandler("Operator.SnapshotRestore")
	} else {
		handlerErr = fmt.Errorf("misconfigured connection")
	}

	if handlerErr != nil {
		return nil, CodedError(500, handlerErr.Error())
	}

	httpPipe, handlerPipe := net.Pipe()
	decoder := codec.NewDecoder(httpPipe, structs.MsgpackHandle)
	encoder := codec.NewEncoder(httpPipe, structs.MsgpackHandle)

	// Create a goroutine that closes the pipe if the connection closes.
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	go func() {
		<-ctx.Done()
		httpPipe.Close()
	}()

	errCh := make(chan HTTPCodedError, 2)
	go func() {
		defer cancel()

		// Send the request
		if err := encoder.Encode(args); err != nil {
			errCh <- CodedError(500, err.Error())
			return
		}

		go func() {
			var wrapper cstructs.StreamErrWrapper
			bytes := make([]byte, 1024)

			for {
				n, err := req.Body.Read(bytes)
				if n > 0 {
					wrapper.Payload = bytes[:n]
					err := encoder.Encode(wrapper)
					if err != nil {
						errCh <- CodedError(500, err.Error())
						return
					}
				}
				if err != nil {
					wrapper.Payload = nil
					wrapper.Error = &cstructs.RpcError{Message: err.Error()}
					err := encoder.Encode(wrapper)
					if err != nil {
						errCh <- CodedError(500, err.Error())
					}
					return
				}
			}
		}()

		var res structs.SnapshotRestoreResponse
		if err := decoder.Decode(&res); err != nil {
			errCh <- CodedError(500, err.Error())
			return
		}

		if res.ErrorMsg != "" {
			errCh <- CodedError(res.ErrorCode, res.ErrorMsg)
			return
		}

		errCh <- nil
	}()

	handler(handlerPipe)
	cancel()
	codedErr := <-errCh

	return nil, codedErr
}

func (s *HTTPServer) UpgradeCheckRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/operator/upgrade-check")
	switch {
	case strings.HasSuffix(path, "/vault-workload-identity"):
		return s.upgradeCheckVaultWorkloadIdentity(resp, req)
	default:
		return nil, CodedError(http.StatusNotFound, fmt.Sprintf("Path %s not found", req.URL.Path))
	}
}

func (s *HTTPServer) upgradeCheckVaultWorkloadIdentity(resp http.ResponseWriter, req *http.Request) (any, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.UpgradeCheckVaultWorkloadIdentityRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.UpgradeCheckVaultWorkloadIdentityResponse
	if err := s.agent.RPC("Operator.UpgradeCheckVaultWorkloadIdentity", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out, nil
}
