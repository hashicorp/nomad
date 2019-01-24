package client

import (
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/client/structs"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
)

// ClientMetadata endpoint is used for manipulating metadata for a client
type ClientMetadata struct {
	c *Client
}

// Metadata is used to retrieve the Clients metadata.
func (m *ClientMetadata) Metadata(args *nstructs.NodeSpecificRequest, reply *structs.ClientMetadataResponse) error {
	defer metrics.MeasureSince([]string{"client", "client_metadata", "metadata"}, time.Now())

	// Check node write permissions
	if aclObj, err := m.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return nstructs.ErrPermissionDenied
	}

	meta := m.c.Node().Meta
	if meta == nil {
		meta = make(map[string]string)
	}

	reply.Metadata = meta
	reply.Index = m.c.Node().ModifyIndex

	return nil
}

// UpdateMetadata is used to partially update the Clients metadata.
func (m *ClientMetadata) UpdateMetadata(req *structs.ClientMetadataUpdateRequest, reply *structs.ClientMetadataUpdateResponse) error {
	defer metrics.MeasureSince([]string{"client", "client_metadata", "update_metadata"}, time.Now())

	// Check node write permissions
	if aclObj, err := m.c.ResolveToken(req.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return nstructs.ErrPermissionDenied
	}

	_, updated := m.c.updateNodeMetadata(req)
	reply.Updated = updated

	return nil
}

// ReplaceMetadata is used to entirely overwrite the Clients metadata.
func (m *ClientMetadata) ReplaceMetadata(req *structs.ClientMetadataReplaceRequest, reply *structs.ClientMetadataUpdateResponse) error {
	defer metrics.MeasureSince([]string{"client", "client_metadata", "replace_metadata"}, time.Now())

	// Check node write permissions
	if aclObj, err := m.c.ResolveToken(req.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return nstructs.ErrPermissionDenied
	}

	m.c.replaceNodeMetadata(req)
	reply.Updated = true

	return nil
}
