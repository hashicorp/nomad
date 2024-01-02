// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package codec

import (
	"errors"
	"net/rpc"
	"reflect"
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
	sourceValue := reflect.Indirect(reflect.Indirect(reflect.ValueOf(i.Args)))
	dst := reflect.Indirect(reflect.Indirect(reflect.ValueOf(args)))
	dst.Set(sourceValue)
	return nil
}

func (i *InmemCodec) WriteResponse(resp *rpc.Response, reply interface{}) error {
	if resp.Error != "" {
		i.Err = errors.New(resp.Error)
		return nil
	}
	sourceValue := reflect.Indirect(reflect.Indirect(reflect.ValueOf(reply)))
	dst := reflect.Indirect(reflect.Indirect(reflect.ValueOf(i.Reply)))
	dst.Set(sourceValue)
	return nil
}

func (i *InmemCodec) Close() error {
	return nil
}
