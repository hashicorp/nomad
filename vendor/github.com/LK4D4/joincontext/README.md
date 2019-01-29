# joincontext

[![Build Status](https://travis-ci.org/LK4D4/joincontext.svg?branch=master)](https://travis-ci.org/LK4D4/joincontext)
[![GoDoc](https://godoc.org/github.com/LK4D4/joincontext?status.svg)](https://godoc.org/github.com/LK4D4/joincontext)

Package joincontext provides a way to combine two contexts.
For example it might be useful for grpc server to cancel all handlers in
addition to provided handler context.

For additional info see [godoc page](https://godoc.org/github.com/LK4D4/joincontext)

## Example
```go
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2 := context.Background()

	ctx, cancel := joincontext.Join(ctx1, ctx2)
	defer cancel()
	select {
	case <-ctx.Done():
	default:
		fmt.Println("context alive")
	}

	cancel1()

	// give some time to propagate
	time.Sleep(100 * time.Millisecond)

	select {
	case <-ctx.Done():
		fmt.Println(ctx.Err())
	default:
	}

	// Output: context alive
	// context canceled
```
