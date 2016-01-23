package structs

import (
	"fmt"
	"strings"

	"github.com/mitchellh/hashstructure"
)

const (
	// NodeUniqueNamespace is a prefix that can be appended to node meta or
	// attribute keys to mark them for exclusion in computed node class.
	NodeUniqueNamespace = "unique."
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

// HashIncludeMap is used to blacklist uniquely identifying node map keys from being
// included in the computed node class.
func (n Node) HashIncludeMap(field string, k, v interface{}) (bool, error) {
	key, ok := k.(string)
	if !ok {
		return false, fmt.Errorf("map key %v not a string")
	}

	// Check if the key is unique.
	isUnique := strings.HasPrefix(key, NodeUniqueNamespace)

	switch field {
	case "Meta", "Attributes":
		return !isUnique, nil
	default:
		return false, fmt.Errorf("unexpected map field: %v", field)
	}
}
