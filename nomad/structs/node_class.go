package structs

import (
	"fmt"
	"strings"

	"github.com/gobwas/glob"
	"github.com/mitchellh/hashstructure"
)

const (
	// NodeUniqueSuffix is a suffix that can be appended to node meta or
	// attribute keys to mark them for exclusion in computed node class.
	NodeUniqueSuffix = "_unique"
)

// ComputeClass computes a derived class for the node based on its attributes.
// ComputedClass is a unique id that identifies nodes with a common set of
// attributes and capabilities. Thus, when calculating a node's computed class
// we avoid including any uniquely identifing fields.
func (n *Node) ComputeClass() error {
	hash, err := hashstructure.Hash(n, nil)
	if err != nil {
		return err
	}

	n.ComputedClass = hash
	return nil
}

// HashInclude is used to blacklist uniquely identifying node fields from being
// included in the computed node class.
func (n Node) HashInclude(field string, v interface{}) (bool, error) {
	switch field {
	case "ID", "Name", "Links": // Uniquely identifying
		return false, nil
	case "Drain", "Status", "StatusDescription": // Set by server
		return false, nil
	case "ComputedClass": // Part of computed node class
		return false, nil
	case "CreateIndex", "ModifyIndex": // Raft indexes
		return false, nil
	case "Resources", "Reserved": // Doesn't effect placement capability
		return false, nil
	default:
		return true, nil
	}
}

var (
	// GlobalUniqueAttrs is a set of attributes that uniquely identify all
	// nodes. It is stored once by the server, rather than by each node to
	// reduce storage costs.
	GlobalUniqueAttrs = []glob.Glob{
		glob.MustCompile("consul.name"),
		glob.MustCompile("platform.gce.hostname"),
		glob.MustCompile("platform.gce.id"),
		glob.MustCompile("platform.gce.network.*.ip"),
		glob.MustCompile("platform.gce.network.*.external-ip"),
		glob.MustCompile("platform.aws.ami-id"),
		glob.MustCompile("platform.aws.hostname"),
		glob.MustCompile("platform.aws.instance-id"),
		glob.MustCompile("platform.aws.local*"),
		glob.MustCompile("platform.aws.public*"),
		glob.MustCompile("network.ip-address"),
		glob.MustCompile("storage.*"), // Ignore all storage
	}
)

// excludeAttr returns whether the key should be excluded when calculating
// computed node class.
func excludeAttr(key string) bool {
	for _, g := range GlobalUniqueAttrs {
		if g.Match(key) {
			return true
		}
	}

	return false
}

// HashIncludeMap is used to blacklist uniquely identifying node map keys from being
// included in the computed node class.
func (n Node) HashIncludeMap(field string, k, v interface{}) (bool, error) {
	key, ok := k.(string)
	if !ok {
		return false, fmt.Errorf("map key %v not a string")
	}

	// Check if the user marked the key as unique.
	isUnique := strings.HasSuffix(key, NodeUniqueSuffix)

	switch field {
	case "Attributes":
		return !excludeAttr(key) && !isUnique, nil
	case "Meta":
		return !isUnique, nil
	default:
		return false, fmt.Errorf("unexpected map field: %v", field)
	}
}
