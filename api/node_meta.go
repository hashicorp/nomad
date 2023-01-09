package api

// NodeMetaSetRequest contains the Node meta update.
type NodeMetaSetRequest struct {
	NodeID string
	Meta   map[string]*string
}

// NodeMetaResponse contains the merged Node metadata.
type NodeMetaResponse struct {
	// Meta is the merged static + dynamic Node metadata
	Meta map[string]string

	// Dynamic is the dynamic Node metadata
	Dynamic map[string]*string
}

// NodeMeta is a client for manipulating dynamic Node metadata.
type NodeMeta struct {
	client *Client
}

// Meta returns a NodeMeta client.
func (n *Nodes) Meta() *NodeMeta {
	return &NodeMeta{client: n.client}
}

// Set dynamic Node metadata updates to a Node. If NodeID is unset then Node
// receiving the request is modified.
func (n *NodeMeta) Set(meta *NodeMetaSetRequest, qo *WriteOptions) (*NodeMetaResponse, error) {
	var out NodeMetaResponse
	_, err := n.client.post("/v1/client/metadata", meta, &out, qo)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Read Node metadata (dynamic and static merged) from a Node directly. May
// differ from Node.Info as dynamic Node metadata updates are batched and may
// be delayed up to 10 seconds.
//
// If nodeID is empty then the metadata for the Node receiving the request is
// returned.
func (n *NodeMeta) Read(nodeID string, qo *QueryOptions) (*NodeMetaResponse, error) {
	var out NodeMetaResponse

	url := "/v1/client/metadata"
	if nodeID != "" {
		url += "?node_id=" + nodeID
	}

	_, err := n.client.query(url, &out, qo)
	if err != nil {
		return nil, err
	}

	return &out, nil
}
