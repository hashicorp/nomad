// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

/*
Package fifo implements functions to create and open a fifo for inter-process
communication in an OS agnostic way. A few assumptions should be made when
using this package. First, New() must always be called before Open(). Second
Open() returns an io.ReadWriteCloser that is only connected with the
io.ReadWriteCloser returned from New().
*/
package fifo
