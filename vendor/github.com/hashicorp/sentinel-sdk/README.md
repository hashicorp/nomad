# Sentinel Import SDK

This repository contains the [Sentinel](https://www.hashicorp.com/sentinel)
import SDK. This SDK allows developers to extend Sentinel to source external
information for use in their policies.

Sentinel imports can be written in any language, but the recommended
language is [Go](https://golang.org/). We provide a high-level framework
to make writing imports in Go extremely easy. For other languages, imports
can be written by implementing the [protocol](#) over gRPC.

To get started writing a Sentinel import, we recommend reading the
[extending Sentinel](#) guide.
