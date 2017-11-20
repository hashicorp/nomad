// Package sockaddr contains a Sentinel plugin for checks on socket addresses.
package sockaddr

import (
	"errors"

	"github.com/hashicorp/errwrap"
	sockaddr "github.com/hashicorp/go-sockaddr"
	"github.com/hashicorp/sentinel-sdk"
	"github.com/hashicorp/sentinel-sdk/framework"
)

var (
	ErrCouldNotParseSockAddr = errors.New("could not parse sockaddr")
)

// New creates a new Import and adheres to sdk/rpc.ImportFunc
func New() sdk.Import {
	return &framework.Import{
		Root: &root{},
	}
}

type root struct{}

// framework.Root impl.
func (m *root) Configure(raw map[string]interface{}) error {
	return nil
}

// framework.Namespace impl.
func (m *root) Get(key string) (interface{}, error) {
	return nil, nil
}

// framework.Call impl.
func (m *root) Func(key string) interface{} {
	switch key {
	case "is_contained":
		return func(outer, inner string) (interface{}, error) {
			outerSA, err := sockaddr.NewSockAddr(outer)
			if err != nil {
				return nil, errwrap.Wrap(ErrCouldNotParseSockAddr, err)
			}
			innerSA, err := sockaddr.NewSockAddr(inner)
			if err != nil {
				return nil, errwrap.Wrap(ErrCouldNotParseSockAddr, err)
			}
			return outerSA.Contains(innerSA), nil
		}

	case "is_equal":
		return func(address1, address2 string) (interface{}, error) {
			sa1, err := sockaddr.NewSockAddr(address1)
			if err != nil {
				return nil, errwrap.Wrap(ErrCouldNotParseSockAddr, err)
			}
			sa2, err := sockaddr.NewSockAddr(address2)
			if err != nil {
				return nil, errwrap.Wrap(ErrCouldNotParseSockAddr, err)
			}
			return sa1.Equal(sa2), nil
		}

	case "is_ipv4":
		return func(address string) (interface{}, error) {
			_, err := sockaddr.NewIPv4Addr(address)
			return err == nil, nil
		}

	case "is_ipv6":
		return func(address string) (interface{}, error) {
			_, err := sockaddr.NewIPv6Addr(address)
			return err == nil, nil
		}
	}

	return nil
}
