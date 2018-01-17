package client

import (
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/client/structs"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
)

// FileSystem endpoint is used for accessing the logs and filesystem of
// allocations.
type FileSystem struct {
	c *Client
}

// Stats is used to retrieve the Clients stats.
func (fs *FileSystem) Logs(args *structs.ClientStatsRequest, reply *structs.ClientStatsResponse) error {
	defer metrics.MeasureSince([]string{"client", "client_stats", "stats"}, time.Now())

	// Check node read permissions
	if aclObj, err := fs.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil {
		readfs := aclObj.AllowNsOp(args.Namespace, acl.NamespaceCapabilityReadFS)
		logs := aclObj.AllowNsOp(args.Namespace, acl.NamespaceCapabilityReadLogs)
		if !readfs && !logs {
			return nstructs.ErrPermissionDenied
		}
	}

	return nil
}
