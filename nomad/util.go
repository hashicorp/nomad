// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	memdb "github.com/hashicorp/go-memdb"
	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/serf/serf"
)

const (
	// Deprecated: Through Nomad v1.2 these values were configurable but
	// functionally unused. We still need to advertise them in Serf for
	// compatibility with v1.2 and earlier.
	deprecatedAPIMajorVersion    = 1
	deprecatedAPIMajorVersionStr = "1"
)

// MinVersionPlanNormalization is the minimum version to support the
// normalization of Plan in SubmitPlan, and the denormalization raft log entry committed
// in ApplyPlanResultsRequest
var MinVersionPlanNormalization = version.Must(version.NewVersion("0.9.2"))

// ensurePath is used to make sure a path exists
func ensurePath(path string, dir bool) error {
	if !dir {
		path = filepath.Dir(path)
	}
	return os.MkdirAll(path, 0755)
}

// serverParts is used to return the parts of a server role
type serverParts struct {
	Name        string
	ID          string
	Region      string
	Datacenter  string
	Port        int
	Bootstrap   bool
	Expect      int
	Build       version.Version
	RaftVersion int
	Addr        net.Addr
	RPCAddr     net.Addr
	Status      serf.MemberStatus
	NonVoter    bool

	// Deprecated: Functionally unused but needs to always be set by 1 for
	// compatibility with v1.2.x and earlier.
	MajorVersion int
}

func (s *serverParts) String() string {
	return fmt.Sprintf("%s (Addr: %s) (DC: %s)",
		s.Name, s.Addr, s.Datacenter)
}

func (s *serverParts) Copy() *serverParts {
	ns := new(serverParts)
	*ns = *s
	return ns
}

// Returns if a member is a Nomad server. Returns a boolean,
// and a struct with the various important components
func isNomadServer(m serf.Member) (bool, *serverParts) {
	if m.Tags["role"] != "nomad" {
		return false, nil
	}

	id := "unknown"
	if v, ok := m.Tags["id"]; ok {
		id = v
	}
	region := m.Tags["region"]
	datacenter := m.Tags["dc"]
	_, bootstrap := m.Tags["bootstrap"]

	expect := 0
	expectStr, ok := m.Tags["expect"]
	var err error
	if ok {
		expect, err = strconv.Atoi(expectStr)
		if err != nil {
			return false, nil
		}
	}

	// If the server is missing the rpc_addr tag, default to the serf advertise addr
	rpcIP := net.ParseIP(m.Tags["rpc_addr"])
	if rpcIP == nil {
		rpcIP = m.Addr
	}

	portStr := m.Tags["port"]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false, nil
	}

	buildVersion, err := version.NewVersion(m.Tags["build"])
	if err != nil {
		return false, nil
	}

	raftVsn := 0
	raftVsnString, ok := m.Tags["raft_vsn"]
	if ok {
		raftVsn, err = strconv.Atoi(raftVsnString)
		if err != nil {
			return false, nil
		}
	}

	// Check if the server is a non voter
	_, nonVoter := m.Tags["nonvoter"]

	addr := &net.TCPAddr{IP: m.Addr, Port: port}
	rpcAddr := &net.TCPAddr{IP: rpcIP, Port: port}
	parts := &serverParts{
		Name:         m.Name,
		ID:           id,
		Region:       region,
		Datacenter:   datacenter,
		Port:         port,
		Bootstrap:    bootstrap,
		Expect:       expect,
		Addr:         addr,
		RPCAddr:      rpcAddr,
		Build:        *buildVersion,
		RaftVersion:  raftVsn,
		Status:       m.Status,
		NonVoter:     nonVoter,
		MajorVersion: deprecatedAPIMajorVersion,
	}
	return true, parts
}

const AllRegions = ""

// ServersMeetMinimumVersion returns whether the Nomad servers are at least on the
// given Nomad version. The checkFailedServers parameter specifies whether version
// for the failed servers should be verified.
func ServersMeetMinimumVersion(members []serf.Member, region string, minVersion *version.Version, checkFailedServers bool) bool {
	for _, member := range members {
		valid, parts := isNomadServer(member)
		if valid &&
			(parts.Region == region || region == AllRegions) &&
			(parts.Status == serf.StatusAlive || (checkFailedServers && parts.Status == serf.StatusFailed)) {
			// Check if the versions match - version.LessThan will return true for
			// 0.8.0-rc1 < 0.8.0, so we want to ignore the metadata
			versionsMatch := slices.Equal(minVersion.Segments(), parts.Build.Segments())
			if parts.Build.LessThan(minVersion) && !versionsMatch {
				return false
			}
		}
	}

	return true
}

// shuffleStrings randomly shuffles the list of strings
func shuffleStrings(list []string) {
	for i := range list {
		j := rand.Intn(i + 1)
		list[i], list[j] = list[j], list[i]
	}
}

// partitionAll splits a slice of strings into a slice of slices of strings, each with a max
// size of `size`. All entries from the original slice are preserved. The last slice may be
// smaller than `size`. The input slice is unmodified
func partitionAll(size int, xs []string) [][]string {
	if size < 1 {
		return [][]string{xs}
	}

	out := [][]string{}

	for i := 0; i < len(xs); i += size {
		j := i + size
		if j > len(xs) {
			j = len(xs)
		}
		out = append(out, xs[i:j])
	}

	return out
}

// maxUint64 returns the maximum value
func maxUint64(inputs ...uint64) uint64 {
	l := len(inputs)
	if l == 0 {
		return 0
	} else if l == 1 {
		return inputs[0]
	}

	max := inputs[0]
	for i := 1; i < l; i++ {
		cur := inputs[i]
		if cur > max {
			max = cur
		}
	}
	return max
}

// getNodeForRpc returns a Node struct if the Node supports Node RPC. Otherwise
// an error is returned.
func getNodeForRpc(snap *state.StateSnapshot, nodeID string) (*structs.Node, error) {
	node, err := snap.NodeByID(nil, nodeID)
	if err != nil {
		return nil, err
	}

	if node == nil {
		return nil, fmt.Errorf("%w %s", structs.ErrUnknownNode, nodeID)
	}

	if err := nodeSupportsRpc(node); err != nil {
		return nil, err
	}

	return node, nil
}

var minNodeVersionSupportingRPC = version.Must(version.NewVersion("0.8.0-rc1"))

// nodeSupportsRpc returns a non-nil error if a Node does not support RPC.
func nodeSupportsRpc(node *structs.Node) error {
	rawNodeVer, ok := node.Attributes["nomad.version"]
	if !ok {
		return structs.ErrUnknownNomadVersion
	}

	nodeVer, err := version.NewVersion(rawNodeVer)
	if err != nil {
		return structs.ErrUnknownNomadVersion
	}

	if nodeVer.LessThan(minNodeVersionSupportingRPC) {
		return structs.ErrNodeLacksRpc
	}

	return nil
}

// AllocGetter is an interface for retrieving allocations by ID. It is
// satisfied by *state.StateStore and *state.StateSnapshot.
type AllocGetter interface {
	AllocByID(ws memdb.WatchSet, id string) (*structs.Allocation, error)
}

// getAlloc retrieves an allocation by ID and namespace. If the allocation is
// nil, an error is returned.
func getAlloc(state AllocGetter, allocID string) (*structs.Allocation, error) {
	if allocID == "" {
		return nil, structs.ErrMissingAllocID
	}

	alloc, err := state.AllocByID(nil, allocID)
	if err != nil {
		return nil, err
	}

	if alloc == nil {
		return nil, structs.NewErrUnknownAllocation(allocID)
	}

	return alloc, nil
}

// tlsCertificateLevel represents a role level for mTLS certificates.
type tlsCertificateLevel int8

const (
	tlsCertificateLevelServer tlsCertificateLevel = iota
	tlsCertificateLevelClient
)

// validateTLSCertificateLevel checks if the provided RPC connection was
// initiated with a certificate that matches the given TLS role level.
//
// - tlsCertificateLevelServer requires a server certificate.
// - tlsCertificateLevelServer requires a client or server certificate.
func validateTLSCertificateLevel(srv *Server, ctx *RPCContext, lvl tlsCertificateLevel) error {
	switch lvl {
	case tlsCertificateLevelClient:
		err := validateLocalClientTLSCertificate(srv, ctx)
		if err != nil {
			return validateLocalServerTLSCertificate(srv, ctx)
		}
		return nil
	case tlsCertificateLevelServer:
		return validateLocalServerTLSCertificate(srv, ctx)
	}

	return fmt.Errorf("invalid TLS certificate level %v", lvl)
}

// validateLocalClientTLSCertificate checks if the provided RPC connection was
// initiated by a client in the same region as the target server.
func validateLocalClientTLSCertificate(srv *Server, ctx *RPCContext) error {
	expected := fmt.Sprintf("client.%s.nomad", srv.Region())

	err := validateTLSCertificate(srv, ctx, expected)
	if err != nil {
		return fmt.Errorf("invalid client connection in region %s: %v", srv.Region(), err)
	}
	return nil
}

// validateLocalServerTLSCertificate checks if the provided RPC connection was
// initiated by a server in the same region as the target server.
func validateLocalServerTLSCertificate(srv *Server, ctx *RPCContext) error {
	expected := fmt.Sprintf("server.%s.nomad", srv.Region())

	err := validateTLSCertificate(srv, ctx, expected)
	if err != nil {
		return fmt.Errorf("invalid server connection in region %s: %v", srv.Region(), err)
	}
	return nil
}

// validateTLSCertificate checks if the RPC connection mTLS certificates are
// valid for the given name.
func validateTLSCertificate(srv *Server, ctx *RPCContext, name string) error {
	if srv.config.TLSConfig == nil || !srv.config.TLSConfig.VerifyServerHostname {
		return nil
	}

	return ctx.ValidateCertificateForName(name)
}
