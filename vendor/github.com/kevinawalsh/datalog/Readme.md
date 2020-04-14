Datalog
=======

This library implements a [datalog
system](http://www.ccs.neu.edu/home/ramsdell/tools/datalog/) in Go. The library
is split into three packages:

* datalog -- The core datalog types and prover.
* datalog/dlengine -- A text-based intepreter that serves as a front-end to the
  datalog prover.
* datalog/dlprim -- Custom datalog primitives, like the Equals predicate.

Setup
-----

After installing a suitable version of Go, run:

`go get github.com/kevinawalsh/datalog`

Documentation
-------------

See the sources, or view the documentation here:

* [datalog](http://godoc.org/github.com/kevinawalsh/datalog)
* [datalog/dlengine](http://godoc.org/github.com/kevinawalsh/datalog/dlengine)
* [datalog/dlprim](http://godoc.org/github.com/kevinawalsh/datalog/dlprim)

