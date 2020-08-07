package agent

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) CSIVolumesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	// Type filters volume lists to a specific type. When support for non-CSI volumes is
	// introduced, we'll need to dispatch here
	query := req.URL.Query()
	qtype, ok := query["type"]
	if !ok {
		return []*structs.CSIVolListStub{}, nil
	}
	if qtype[0] != "csi" {
		return nil, nil
	}

	args := structs.CSIVolumeListRequest{}

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	if plugin, ok := query["plugin_id"]; ok {
		args.PluginID = plugin[0]
	}
	if node, ok := query["node_id"]; ok {
		args.NodeID = node[0]
	}

	var out structs.CSIVolumeListResponse
	if err := s.agent.RPC("CSIVolume.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Volumes, nil
}

// CSIVolumeSpecificRequest dispatches GET and PUT
func (s *HTTPServer) CSIVolumeSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Tokenize the suffix of the path to get the volume id
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/volume/csi/")
	tokens := strings.Split(reqSuffix, "/")
	if len(tokens) > 2 || len(tokens) < 1 {
		return nil, CodedError(404, resourceNotFoundErr)
	}
	id := tokens[0]

	switch req.Method {
	case "GET":
		return s.csiVolumeGet(id, resp, req)
	case "PUT":
		return s.csiVolumePut(id, resp, req)
	case "DELETE":
		return s.csiVolumeDelete(id, resp, req)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) csiVolumeGet(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.CSIVolumeGetRequest{
		ID: id,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.CSIVolumeGetResponse
	if err := s.agent.RPC("CSIVolume.Get", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Volume == nil {
		return nil, CodedError(404, "volume not found")
	}

	// remove sensitive fields, as our redaction mechanism doesn't
	// help serializing here
	out.Volume.Secrets = nil
	out.Volume.MountOptions = nil
	return out.Volume, nil
}

func (s *HTTPServer) csiVolumePut(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args0 := structs.CSIVolumeRegisterRequest{}
	if err := decodeBody(req, &args0); err != nil {
		return err, CodedError(400, err.Error())
	}

	args := structs.CSIVolumeRegisterRequest{
		Volumes: args0.Volumes,
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.CSIVolumeRegisterResponse
	if err := s.agent.RPC("CSIVolume.Register", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)

	return nil, nil
}

func (s *HTTPServer) csiVolumeDelete(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "DELETE" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	raw := req.URL.Query().Get("detach")
	var detach bool
	if raw != "" {
		var err error
		detach, err = strconv.ParseBool(raw)
		if err != nil {
			return nil, CodedError(400, "invalid detach value")
		}
	}

	raw = req.URL.Query().Get("force")
	var force bool
	if raw != "" {
		var err error
		force, err = strconv.ParseBool(raw)
		if err != nil {
			return nil, CodedError(400, "invalid force value")
		}
	}

	if detach {
		nodeID := req.URL.Query().Get("node")
		if nodeID == "" {
			return nil, CodedError(400, "detach requires node ID")
		}

		args := structs.CSIVolumeUnpublishRequest{
			VolumeID: id,
			Claim: &structs.CSIVolumeClaim{
				NodeID: nodeID,
				Mode:   structs.CSIVolumeClaimRelease,
			},
		}
		s.parseWriteRequest(req, &args.WriteRequest)

		var out structs.CSIVolumeUnpublishResponse
		if err := s.agent.RPC("CSIVolume.Unpublish", &args, &out); err != nil {
			return nil, err
		}

		setMeta(resp, &out.QueryMeta)
		return nil, nil
	}

	args := structs.CSIVolumeDeregisterRequest{
		VolumeIDs: []string{id},
		Force:     force,
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.CSIVolumeDeregisterResponse
	if err := s.agent.RPC("CSIVolume.Deregister", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)

	return nil, nil
}

// CSIPluginsRequest lists CSI plugins
func (s *HTTPServer) CSIPluginsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	// Type filters plugin lists to a specific type. When support for non-CSI plugins is
	// introduced, we'll need to dispatch here
	query := req.URL.Query()
	qtype, ok := query["type"]
	if !ok {
		return []*structs.CSIPluginListStub{}, nil
	}
	if qtype[0] != "csi" {
		return nil, nil
	}

	args := structs.CSIPluginListRequest{}

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.CSIPluginListResponse
	if err := s.agent.RPC("CSIPlugin.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Plugins, nil
}

// CSIPluginSpecificRequest list the job with CSIInfo
func (s *HTTPServer) CSIPluginSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	// Tokenize the suffix of the path to get the plugin id
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/plugin/csi/")
	tokens := strings.Split(reqSuffix, "/")
	if len(tokens) > 2 || len(tokens) < 1 {
		return nil, CodedError(404, resourceNotFoundErr)
	}
	id := tokens[0]

	args := structs.CSIPluginGetRequest{ID: id}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.CSIPluginGetResponse
	if err := s.agent.RPC("CSIPlugin.Get", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Plugin == nil {
		return nil, CodedError(404, "plugin not found")
	}

	return out.Plugin, nil
}
