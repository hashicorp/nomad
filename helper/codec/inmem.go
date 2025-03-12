// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package codec

import (
	"errors"
	"fmt"
	"net/rpc"
	"reflect"

	"github.com/mitchellh/copystructure"
)

// InmemCodec is used to do an RPC call without going over a network
type InmemCodec struct {
	Method string
	Args   interface{}
	Reply  interface{}
	Err    error
}

func (i *InmemCodec) ReadRequestHeader(req *rpc.Request) error {
	req.ServiceMethod = i.Method
	return nil
}

func (i *InmemCodec) ReadRequestBody(args interface{}) error {
	if args == nil {
		return nil
	}

	// Copy on read to avoid sharing pointers between callers and handlers
	origArgs, err := copystructure.Copy(i.Args)
	if err != nil {
		return fmt.Errorf("error copying arguments to %s rpc: %w", i.Method, err)
	}

	sourceValue := reflect.Indirect(reflect.Indirect(reflect.ValueOf(origArgs)))
	dst := reflect.Indirect(reflect.Indirect(reflect.ValueOf(args)))
	dst.Set(sourceValue)
	return nil
}

func (i *InmemCodec) WriteResponse(resp *rpc.Response, reply interface{}) error {
	if resp.Error != "" {
		i.Err = errors.New(resp.Error)
		return nil
	}

	// Copy on write to avoid sharing pointers between callers and handlers
	replyCopy, err := copystructure.Copy(reply)
	if err != nil {
		return fmt.Errorf("error copying reply from %s rpc: %w", i.Method, err)
	}
	sourceValue := reflect.Indirect(reflect.Indirect(reflect.ValueOf(replyCopy)))
	dst := reflect.Indirect(reflect.Indirect(reflect.ValueOf(i.Reply)))
	dst.Set(sourceValue)
	return nil
}

func (i *InmemCodec) Close() error {
	return nil
}
