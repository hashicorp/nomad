// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/acl"
	"golang.org/x/crypto/blake2b"
)

// RemoveAllocs is used to remove any allocs with the given IDs
// from the list of allocations
func RemoveAllocs(allocs []*Allocation, remove []*Allocation) []*Allocation {
	if len(remove) == 0 {
		return allocs
	}
	// Convert remove into a set
	removeSet := make(map[string]struct{})
	for _, remove := range remove {
		removeSet[remove.ID] = struct{}{}
	}

	r := make([]*Allocation, 0, len(allocs))
	for _, alloc := range allocs {
		if _, ok := removeSet[alloc.ID]; !ok {
			r = append(r, alloc)
		}
	}
	return r
}

func AllocSubset(allocs []*Allocation, subset []*Allocation) bool {
	if len(subset) == 0 {
		return true
	}
	// Convert allocs into a map
	allocMap := make(map[string]struct{})
	for _, alloc := range allocs {
		allocMap[alloc.ID] = struct{}{}
	}

	for _, alloc := range subset {
		if _, ok := allocMap[alloc.ID]; !ok {
			return false
		}
	}
	return true
}

// FilterTerminalAllocs filters out all allocations in a terminal state and
// returns the latest terminal allocations.
func FilterTerminalAllocs(allocs []*Allocation) ([]*Allocation, map[string]*Allocation) {
	terminalAllocsByName := make(map[string]*Allocation)
	n := len(allocs)

	for i := 0; i < n; i++ {
		if allocs[i].TerminalStatus() {

			// Add the allocation to the terminal allocs map if it's not already
			// added or has a higher create index than the one which is
			// currently present.
			alloc, ok := terminalAllocsByName[allocs[i].Name]
			if !ok || alloc.CreateIndex < allocs[i].CreateIndex {
				terminalAllocsByName[allocs[i].Name] = allocs[i]
			}

			// Remove the allocation
			allocs[i], allocs[n-1] = allocs[n-1], nil
			i--
			n--
		}
	}

	return allocs[:n], terminalAllocsByName
}

// SplitTerminalAllocs splits allocs into non-terminal and terminal allocs, with
// the terminal allocs indexed by node->alloc.name.
func SplitTerminalAllocs(allocs []*Allocation) ([]*Allocation, TerminalByNodeByName) {
	var alive []*Allocation
	var terminal = make(TerminalByNodeByName)

	for _, alloc := range allocs {
		if alloc.TerminalStatus() {
			terminal.Set(alloc)
		} else {
			alive = append(alive, alloc)
		}
	}

	return alive, terminal
}

// TerminalByNodeByName is a map of NodeID->Allocation.Name->Allocation used by
// the sysbatch scheduler for locating the most up-to-date terminal allocations.
type TerminalByNodeByName map[string]map[string]*Allocation

func (a TerminalByNodeByName) Set(allocation *Allocation) {
	node := allocation.NodeID
	name := allocation.Name

	if _, exists := a[node]; !exists {
		a[node] = make(map[string]*Allocation)
	}

	if previous, exists := a[node][name]; !exists {
		a[node][name] = allocation
	} else if previous.CreateIndex < allocation.CreateIndex {
		// keep the newest version of the terminal alloc for the coordinate
		a[node][name] = allocation
	}
}

func (a TerminalByNodeByName) Get(nodeID, name string) (*Allocation, bool) {
	if _, exists := a[nodeID]; !exists {
		return nil, false
	}

	if _, exists := a[nodeID][name]; !exists {
		return nil, false
	}

	return a[nodeID][name], true
}

// AllocsFit checks if a given set of allocations will fit on a node.
// The netIdx can optionally be provided if its already been computed.
// If the netIdx is provided, it is assumed that the client has already
// ensured there are no collisions. If checkDevices is set to true, we check if
// there is a device oversubscription.
func AllocsFit(node *Node, allocs []*Allocation, netIdx *NetworkIndex, checkDevices bool) (bool, string, *ComparableResources, error) {
	// Compute the allocs' utilization from zero
	used := new(ComparableResources)

	reservedCores := map[uint16]struct{}{}
	var coreOverlap bool

	// For each alloc, add the resources
	for _, alloc := range allocs {
		// Do not consider the resource impact of terminal allocations
		if alloc.ClientTerminalStatus() {
			continue
		}

		cr := alloc.ComparableResources()
		used.Add(cr)

		// Adding the comparable resource unions reserved core sets, need to check if reserved cores overlap
		for _, core := range cr.Flattened.Cpu.ReservedCores {
			if _, ok := reservedCores[core]; ok {
				coreOverlap = true
			} else {
				reservedCores[core] = struct{}{}
			}
		}
	}

	if coreOverlap {
		return false, "cores", used, nil
	}

	// Check that the node resources (after subtracting reserved) are a
	// super set of those that are being allocated
	available := node.ComparableResources()
	available.Subtract(node.ComparableReservedResources())
	if superset, dimension := available.Superset(used); !superset {
		return false, dimension, used, nil
	}

	// Create the network index if missing
	if netIdx == nil {
		netIdx = NewNetworkIndex()
		defer netIdx.Release()

		if err := netIdx.SetNode(node); err != nil {
			// To maintain backward compatibility with when SetNode
			// returned collision+reason like AddAllocs, return
			// this as a reason instead of an error.
			return false, fmt.Sprintf("reserved node port collision: %v", err), used, nil
		}
		if collision, reason := netIdx.AddAllocs(allocs); collision {
			return false, fmt.Sprintf("reserved alloc port collision: %v", reason), used, nil
		}
	}

	// Check if the network is overcommitted
	if netIdx.Overcommitted() {
		return false, "bandwidth exceeded", used, nil
	}

	// Check devices
	if checkDevices {
		accounter := NewDeviceAccounter(node)
		if accounter.AddAllocs(allocs) {
			return false, "device oversubscribed", used, nil
		}
	}

	// Allocations fit!
	return true, "", used, nil
}

func computeFreePercentage(node *Node, util *ComparableResources) (freePctCpu, freePctRam float64) {
	// COMPAT(0.11): Remove in 0.11
	reserved := node.ComparableReservedResources()
	res := node.ComparableResources()

	// Determine the node availability
	nodeCpu := float64(res.Flattened.Cpu.CpuShares)
	nodeMem := float64(res.Flattened.Memory.MemoryMB)
	if reserved != nil {
		nodeCpu -= float64(reserved.Flattened.Cpu.CpuShares)
		nodeMem -= float64(reserved.Flattened.Memory.MemoryMB)
	}

	// Compute the free percentage
	freePctCpu = 1 - (float64(util.Flattened.Cpu.CpuShares) / nodeCpu)
	freePctRam = 1 - (float64(util.Flattened.Memory.MemoryMB) / nodeMem)
	return freePctCpu, freePctRam
}

// ScoreFitBinPack computes a fit score to achieve pinbacking behavior.
// Score is in [0, 18]
//
// It's the BestFit v3 on the Google work published here:
// http://www.columbia.edu/~cs2035/courses/ieor4405.S13/datacenter_scheduling.ppt
func ScoreFitBinPack(node *Node, util *ComparableResources) float64 {
	freePctCpu, freePctRam := computeFreePercentage(node, util)

	// Total will be "maximized" the smaller the value is.
	// At 100% utilization, the total is 2, while at 0% util it is 20.
	total := math.Pow(10, freePctCpu) + math.Pow(10, freePctRam)

	// Invert so that the "maximized" total represents a high-value
	// score. Because the floor is 20, we simply use that as an anchor.
	// This means at a perfect fit, we return 18 as the score.
	score := 20.0 - total

	// Bound the score, just in case
	// If the score is over 18, that means we've overfit the node.
	if score > 18.0 {
		score = 18.0
	} else if score < 0 {
		score = 0
	}
	return score
}

// ScoreFitSpread computes a fit score to achieve spread behavior.
// Score is in [0, 18]
//
// This is equivalent to Worst Fit of
// http://www.columbia.edu/~cs2035/courses/ieor4405.S13/datacenter_scheduling.ppt
func ScoreFitSpread(node *Node, util *ComparableResources) float64 {
	freePctCpu, freePctRam := computeFreePercentage(node, util)
	total := math.Pow(10, freePctCpu) + math.Pow(10, freePctRam)
	score := total - 2

	if score > 18.0 {
		score = 18.0
	} else if score < 0 {
		score = 0
	}
	return score
}

func CopySliceConstraints(s []*Constraint) []*Constraint {
	l := len(s)
	if l == 0 {
		return nil
	}

	c := make([]*Constraint, l)
	for i, v := range s {
		c[i] = v.Copy()
	}
	return c
}

func CopySliceAffinities(s []*Affinity) []*Affinity {
	l := len(s)
	if l == 0 {
		return nil
	}

	c := make([]*Affinity, l)
	for i, v := range s {
		c[i] = v.Copy()
	}
	return c
}

func CopySliceSpreads(s []*Spread) []*Spread {
	l := len(s)
	if l == 0 {
		return nil
	}

	c := make([]*Spread, l)
	for i, v := range s {
		c[i] = v.Copy()
	}
	return c
}

func CopySliceSpreadTarget(s []*SpreadTarget) []*SpreadTarget {
	l := len(s)
	if l == 0 {
		return nil
	}

	c := make([]*SpreadTarget, l)
	for i, v := range s {
		c[i] = v.Copy()
	}
	return c
}

func CopySliceNodeScoreMeta(s []*NodeScoreMeta) []*NodeScoreMeta {
	l := len(s)
	if l == 0 {
		return nil
	}

	c := make([]*NodeScoreMeta, l)
	for i, v := range s {
		c[i] = v.Copy()
	}
	return c
}

// VaultPoliciesSet takes the structure returned by VaultPolicies and returns
// the set of required policies
func VaultPoliciesSet(policies map[string]map[string]*Vault) []string {
	s := set.New[string](10)
	for _, tgp := range policies {
		for _, tp := range tgp {
			if tp != nil {
				s.InsertAll(tp.Policies)
			}
		}
	}
	return s.List()
}

// VaultNamespaceSet takes the structure returned by VaultPolicies and
// returns a set of required namespaces
func VaultNamespaceSet(policies map[string]map[string]*Vault) []string {
	s := set.New[string](10)
	for _, tgp := range policies {
		for _, tp := range tgp {
			if tp != nil && tp.Namespace != "" {
				s.Insert(tp.Namespace)
			}
		}
	}
	return s.List()
}

// DenormalizeAllocationJobs is used to attach a job to all allocations that are
// non-terminal and do not have a job already. This is useful in cases where the
// job is normalized.
func DenormalizeAllocationJobs(job *Job, allocs []*Allocation) {
	if job != nil {
		for _, alloc := range allocs {
			if alloc.Job == nil && !alloc.TerminalStatus() {
				alloc.Job = job
			}
		}
	}
}

// AllocName returns the name of the allocation given the input.
func AllocName(job, group string, idx uint) string {
	return job + "." + group + "[" + strconv.FormatUint(uint64(idx), 10) + "]"
}

// AllocSuffix returns the alloc index suffix that was added by the AllocName
// function above.
func AllocSuffix(name string) string {
	idx := strings.LastIndex(name, "[")
	if idx == -1 {
		return ""
	}
	suffix := name[idx:]
	return suffix
}

// ACLPolicyListHash returns a consistent hash for a set of policies.
func ACLPolicyListHash(policies []*ACLPolicy) string {
	cacheKeyHash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	for _, policy := range policies {
		_, _ = cacheKeyHash.Write([]byte(policy.Name))
		_ = binary.Write(cacheKeyHash, binary.BigEndian, policy.ModifyIndex)
	}
	cacheKey := string(cacheKeyHash.Sum(nil))
	return cacheKey
}

// CompileACLObject compiles a set of ACL policies into an ACL object with a cache
func CompileACLObject(cache *ACLCache[*acl.ACL], policies []*ACLPolicy) (*acl.ACL, error) {
	// Sort the policies to ensure consistent ordering
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Name < policies[j].Name
	})

	// Determine the cache key
	cacheKey := ACLPolicyListHash(policies)
	entry, ok := cache.Get(cacheKey)
	if ok {
		return entry.Get(), nil
	}

	// Parse the policies
	parsed := make([]*acl.Policy, 0, len(policies))
	for _, policy := range policies {
		p, err := acl.Parse(policy.Rules)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q: %v", policy.Name, err)
		}
		parsed = append(parsed, p)
	}

	// Create the ACL object
	aclObj, err := acl.NewACL(false, parsed)
	if err != nil {
		return nil, fmt.Errorf("failed to construct ACL: %v", err)
	}

	// Update the cache
	cache.Add(cacheKey, aclObj)
	return aclObj, nil
}

// GenerateMigrateToken will create a token for a client to access an
// authenticated volume of another client to migrate data for sticky volumes.
func GenerateMigrateToken(allocID, nodeSecretID string) (string, error) {
	h, err := blake2b.New512([]byte(nodeSecretID))
	if err != nil {
		return "", err
	}

	_, _ = h.Write([]byte(allocID))

	return base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
}

// CompareMigrateToken returns true if two migration tokens can be computed and
// are equal.
func CompareMigrateToken(allocID, nodeSecretID, otherMigrateToken string) bool {
	h, err := blake2b.New512([]byte(nodeSecretID))
	if err != nil {
		return false
	}

	_, _ = h.Write([]byte(allocID))

	otherBytes, err := base64.URLEncoding.DecodeString(otherMigrateToken)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(h.Sum(nil), otherBytes) == 1
}

// ParsePortRanges parses the passed port range string and returns a list of the
// ports. The specification is a comma separated list of either port numbers or
// port ranges. A port number is a single integer and a port range is two
// integers separated by a hyphen. As an example the following spec would
// convert to: ParsePortRanges("10,12-14,16") -> []uint64{10, 12, 13, 14, 16}
func ParsePortRanges(spec string) ([]uint64, error) {
	parts := strings.Split(spec, ",")

	// Hot path the empty case
	if len(parts) == 1 && parts[0] == "" {
		return nil, nil
	}

	ports := make(map[uint64]struct{})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		rangeParts := strings.Split(part, "-")
		l := len(rangeParts)
		switch l {
		case 1:
			if val := rangeParts[0]; val == "" {
				return nil, fmt.Errorf("can't specify empty port")
			} else {
				port, err := strconv.ParseUint(val, 10, 0)
				if err != nil {
					return nil, err
				}

				if port > MaxValidPort {
					return nil, fmt.Errorf("port must be < %d but found %d", MaxValidPort, port)
				}
				ports[port] = struct{}{}
			}
		case 2:
			// We are parsing a range
			start, err := strconv.ParseUint(rangeParts[0], 10, 0)
			if err != nil {
				return nil, err
			}

			end, err := strconv.ParseUint(rangeParts[1], 10, 0)
			if err != nil {
				return nil, err
			}

			if end < start {
				return nil, fmt.Errorf("invalid range: starting value (%v) less than ending (%v) value", end, start)
			}

			// Full range validation is below but prevent creating
			// arbitrarily large arrays here
			if end > MaxValidPort {
				return nil, fmt.Errorf("port must be < %d but found %d", MaxValidPort, end)
			}

			for i := start; i <= end; i++ {
				ports[i] = struct{}{}
			}
		default:
			return nil, fmt.Errorf("can only parse single port numbers or port ranges (ex. 80,100-120,150)")
		}
	}

	var results []uint64
	for port := range ports {
		if port == 0 {
			return nil, fmt.Errorf("port must be > 0")
		}
		if port > MaxValidPort {
			return nil, fmt.Errorf("port must be < %d but found %d", MaxValidPort, port)
		}
		results = append(results, port)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i] < results[j]
	})
	return results, nil
}
