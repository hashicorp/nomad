// Package addrs contains types that represent "addresses", which are
// references to specific objects within a Packer configuration.
//
// All addresses have string representations based on HCL traversal syntax
// which should be used in the user-interface, and also in-memory
// representations that can be used internally.
//
// All types within this package should be treated as immutable, even if this
// is not enforced by the Go compiler. It is always an implementation error
// to modify an address object in-place after it is initially constructed.
package addrs
