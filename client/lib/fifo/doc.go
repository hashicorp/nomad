// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

/*
Package fifo implements functions to create and open a fifo for inter-process
communication in an OS agnostic way. A few assumptions should be made when using
this package. First, New() must always be called before Open(). Second Open()
returns an io.ReadWriteCloser that is only connected with the io.ReadWriteCloser
returned from New().

On Unix, all exported functions use os.Root under the hood to avoid chasing
symlinks out of their parent directory. On Windows, this is unnecessary because
named pipes exist in their own namespace and not the filesystem.
*/
package fifo
