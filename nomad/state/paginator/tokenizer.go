package paginator

import (
	"fmt"
	"strings"
)

// Tokenizer is the interface that must be implemented to provide pagination
// tokens to the Paginator.
type Tokenizer interface {
	// GetToken returns the pagination token for the given element.
	GetToken(interface{}) string
}

// IDGetter is the interface that must be implemented by structs that need to
// have their ID as part of the pagination token.
type IDGetter interface {
	GetID() string
}

// NamespaceGetter is the interface that must be implemented by structs that
// need to have their Namespace as part of the pagination token.
type NamespaceGetter interface {
	GetNamespace() string
}

// CreateIndexGetter is the interface that must be implemented by structs that
// need to have their CreateIndex as part of the pagination token.
type CreateIndexGetter interface {
	GetCreateIndex() uint64
}

// StructsTokenizerOptions is the configuration provided to a StructsTokenizer.
type StructsTokenizerOptions struct {
	WithCreateIndex bool
	WithNamespace   bool
	WithID          bool
}

// StructsTokenizer is an pagination token generator that can create different
// formats of pagination tokens based on common fields found in the structs
// package.
type StructsTokenizer struct {
	opts StructsTokenizerOptions
}

// NewStructsTokenizer returns a new StructsTokenizer.
func NewStructsTokenizer(it Iterator, opts StructsTokenizerOptions) StructsTokenizer {
	return StructsTokenizer{
		opts: opts,
	}
}

func (it StructsTokenizer) GetToken(raw interface{}) string {
	if raw == nil {
		return ""
	}

	parts := []string{}

	if it.opts.WithCreateIndex {
		token := raw.(CreateIndexGetter).GetCreateIndex()
		parts = append(parts, fmt.Sprintf("%v", token))
	}

	if it.opts.WithNamespace {
		token := raw.(NamespaceGetter).GetNamespace()
		parts = append(parts, token)
	}

	if it.opts.WithID {
		token := raw.(IDGetter).GetID()
		parts = append(parts, token)
	}

	// Use a character that is not part of validNamespaceName as separator to
	// avoid accidentally generating collisions.
	// For example, namespace `a` and job `b-c` would collide with namespace
	// `a-b` and job `c` into the same token `a-b-c`, since `-` is an allowed
	// character in namespace.
	return strings.Join(parts, ".")
}
