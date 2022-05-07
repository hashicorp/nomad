# CGO

Nomad requires the use of CGO on Linux.

Issue [#5643](https://github.com/hashicorp/nomad/issues/5643) tracks the desire for Nomad to not require CGO.

One of the core features of Nomad (the exec driver) depends on [nsenter](https://pkg.go.dev/github.com/opencontainers/runc/libcontainer/nsenter).
Until `nsenter` no longer requires CGO, the standalone Nomad executable on Linux will not be able to ship without depending on CGO.
