# Buf

> `buf` is a high-performance `protoc` replacement.

## Installation

Use `make bootstrap` in the root of this repo to install the version of `buf` used by Nomad.

## Usage
`make proto` in the root of this repo will invoke `buf` using the configuration in this directory.

## Why use `buf` instead of `protoc`?

Buf is a user-friendly tool to work with Protobuf that outperforms `protoc` in every conceivable way.
It was written by the author(s) of [`prototool`](https://github.com/uber/prototool), another tool
that made generating Protobuf easier, but which is now deprecated in favor of `buf`. Buf also does
linting and breaking-change detection.
