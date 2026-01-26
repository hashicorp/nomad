// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package lang

import (
	"context"
	"io"
)

// CtxReader is a context-aware io.Reader, see
// https://pace.dev/blog/2020/02/03/context-aware-ioreader-for-golang-by-mat-ryer.html
type CtxReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *CtxReader) Read(p []byte) (n int, err error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func NewCtxReader(ctx context.Context, r io.Reader) io.Reader {
	return &CtxReader{ctx: ctx, r: r}
}
