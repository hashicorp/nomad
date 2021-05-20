# flex - CSS flexbox layout implementation in Go

Go implementation of [flexbox CSS](https://www.w3.org/TR/css-flexbox-1/) layout algorithm.

A pure Go port of [Facebook's Yoga](https://github.com/facebook/yoga).

## How to use

Read [tutorial](https://blog.kowalczyk.info/article/9/tutorial-on-using-github.comkjkflex-go-package.html) or look at `_test.go` files.

## Status

The port is finished. The code works and passess all Yoga tests.

The API is awkward by Go standards but it's the best I could do given that I want to stay close to C version.

Logic is currently synced up to  https://github.com/facebook/yoga/commit/f45059e1e696727c1282742b89d2c8bf06345254

## How the port was made

You can read a [detailed story](https://blog.kowalczyk.info/article/wN9R/experience-porting-4.5k-loc-of-c-to-go-facebooks-css-flexbox-implementation-yoga.html).

In short:

* manually ported [C code](https://github.com/facebook/yoga/tree/master/yoga) to Go, line-by-line
* manually ported [tests](https://github.com/facebook/yoga/tree/master/tests) to Go
* tweak the API from C style to be more Go like. The structure and logic still is very close to C code (this makes porting future C changes easy)
