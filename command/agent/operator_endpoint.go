package agent

import (
	"net/http"
	"strings"

	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
)

func (s *HTTPServer) OperatorRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/operator/raft/")
	switch {
	case strings.HasPrefix(path, "configuration"):
		return s.OperatorRaftConfiguration(resp, req)
	case strings.HasPrefix(path, "peer"):
		return s.OperatorRaftPeer(resp, req)
	default:
		return nil, CodedError(404, ErrInvalidMethod)
	}
}

// OperatorRaftConfiguration is used to inspect the current Raft configuration.
// This supports the stale query mode in case the cluster doesn't have a leader.
func (s *HTTPServer) OperatorRaftConfiguration(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
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

// OperatorRaftPeer supports actions on Raft peers. Currently we only support
// removing peers by address.
func (s *HTTPServer) OperatorRaftPeer(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "DELETE" {
		return nil, CodedError(404, ErrInvalidMethod)
	}

	params := req.URL.Query()
	_, hasID := params["id"]
	_, hasAddress := params["address"]

	if !hasID && !hasAddress {
		return nil, CodedError(http.StatusBadRequest, "Must specify either ?id with the server's ID or ?address with IP:port of peer to remove")
	}
	if hasID && hasAddress {
		return nil, CodedError(http.StatusBadRequest, "Must specify only one of ?id or ?address")
	}

	if hasID {
		var args structs.RaftPeerByIDRequest
		s.parseWriteRequest(req, &args.WriteRequest)

		var reply struct{}
		args.ID = raft.ServerID(params.Get("id"))
		if err := s.agent.RPC("Operator.RaftRemovePeerByID", &args, &reply); err != nil {
			return nil, err
		}
	} else {
		var args structs.RaftPeerByAddressRequest
		s.parseWriteRequest(req, &args.WriteRequest)

		var reply struct{}
		args.Address = raft.ServerAddress(params.Get("address"))
		if err := s.agent.RPC("Operator.RaftRemovePeerByAddress", &args, &reply); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// OperatorAutopilotConfiguration is used to inspect the current Autopilot configuration.
// This supports the stale query mode in case the cluster doesn't have a leader.
func (s *HTTPServer) OperatorAutopilotConfiguration(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Switch on the method
	switch req.Method {
	case "GET":
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
			ServerStabilizationTime: reply.ServerStabilizationTime,
			EnableRedundancyZones:   reply.EnableRedundancyZones,
			DisableUpgradeMigration: reply.DisableUpgradeMigration,
			EnableCustomUpgrades:    reply.EnableCustomUpgrades,
			CreateIndex:             reply.CreateIndex,
			ModifyIndex:             reply.ModifyIndex,
		}

		return out, nil

	case "PUT":
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
	if req.Method != "GET" {
		return nil, CodedError(404, ErrInvalidMethod)
	}

	var args structs.GenericRequest
	if done := s.parse(resp, req, &args.Region, &args.QueryOptions); done {
		return nil, nil
	}

	var reply autopilot.OperatorHealthReply
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

	return out, nil
}
