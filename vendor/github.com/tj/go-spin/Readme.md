
# go-spin

 Little terminal spinner lib.

 View the [docs](http://godoc.org/github.com/tj/go-spin).

## Installation

```
$ go get github.com/tj/go-spin
```

## Example

```go
s := spin.New()
for i := 0; i < 30; i++ {
  fmt.Printf("\r  \033[36mcomputing\033[m %s ", s.Next())
  time.Sleep(100 * time.Millisecond)
}
```
## GIF FTW
![](./go-spin.gif)

# License

 MIT