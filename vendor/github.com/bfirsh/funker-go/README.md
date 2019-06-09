# Funker for Go

A Go implementation of [Funker](https://github.com/bfirsh/funker).

## Usage

Defining functions:

```go
package main

import "github.com/bfirsh/funker-go"

type addArgs struct {
  X int `json:"x"`
  Y int `json:"y"`
}

func main() {
    err := funker.Handle(func(args *addArgs) int {
      return args.X + args.Y;
    });
    if err != nil {
      panic(err);
    }
}
```

Calling functions:

```go
ret, err := funker.Call(addArgs{X: 1, Y: 2});
if err != nil {
  panic(err);
}
fmt.PrintLn(ret);
```
