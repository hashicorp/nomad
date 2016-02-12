# go-getter

[![Build Status](http://img.shields.io/travis/hashicorp/go-getter.svg?style=flat-square)][travis]
[![Go Documentation](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)][godocs]

[travis]: http://travis-ci.org/hashicorp/go-getter
[godocs]: http://godoc.org/github.com/hashicorp/go-getter

go-getter is a library for Go (golang) for downloading files or directories
from various sources using a URL as the primary form of input.

The power of this library is being flexible in being able to download
from a number of different sources (file paths, Git, HTTP, Mercurial, etc.)
using a single string as input. This removes the burden of knowing how to
download from a variety of sources from the implementer.

The concept of a _detector_ automatically turns invalid URLs into proper
URLs. For example: "github.com/hashicorp/go-getter" would turn into a
Git URL. Or "./foo" would turn into a file URL. These are extensible.

This library is used by [Terraform](https://terraform.io) for
downloading modules, [Otto](https://ottoproject.io) for dependencies and
Appfile imports, and [Nomad](https://nomadproject.io) for downloading
binaries.

## Installation and Usage

Package documentation can be found on
[GoDoc](http://godoc.org/github.com/hashicorp/go-getter).

Installation can be done with a normal `go get`:

```
$ go get github.com/hashicorp/go-getter
```

## URL Format

go-getter uses a single string URL as input to downlaod from a variety of
protocols. go-getter has various "tricks" with this URL to do certain things.
This section documents the URL format.

### Supported Protocols and Detectors

**Protocols** are used to download files/directories using a specific
mechanism. Example protocols are Git and HTTP.

**Detectors** are used to transform a valid or invalid URL into another
URL if it matches a certain pattern. Example: "github.com/user/repo" is
automatically transformed into a fully valid Git URL. This allows go-getter
to be very user friendly.

go-getter out of the box supports the following protocols. Additional protocols
can be augmented at runtime by implementing the `Getter` interface.

  * Local files
  * Git
  * Mercurial
  * HTTP
  * Amazon S3

In addition to the above protocols, go-getter has what are called "detectors."
These take a URL and attempt to automatically choose the best protocol for
it, which might involve even changing the protocol. The following detection
is built-in by default:

  * File paths such as "./foo" are automatically changed to absolute
    file URLs.
  * GitHub URLs, such as "github.com/mitchellh/vagrant" are automatically
    changed to Git protocol over HTTP.
  * BitBucket URLs, such as "bitbucket.org/mitchellh/vagrant" are automatically
    changed to a Git or mercurial protocol using the BitBucket API.

### Forced Protocol

In some cases, the protocol to use is ambiguous depending on the source
URL. For example, "http://github.com/mitchellh/vagrant.git" could reference
an HTTP URL or a Git URL. Forced protocol syntax is used to disambiguate this
URL.

Forced protocol can be done by prefixing the URL with the protocol followed
by double colons. For example: `git::http://github.com/mitchellh/vagrant.git`
would download the given HTTP URL using the Git protocol.

Forced protocols will also override any detectors.

In the absense of a forced protocol, detectors may be run on the URL, transforming
the protocol anyways. The above example would've used the Git protocol either
way since the Git detector would've detected it was a GitHub URL.

### Checksumming

For file downloads of any protocol, go-getter can automatically verify
a checksum for you. Note that checksumming only works for downloading files,
not directories, but checksumming will work for any protocol.

To checksum a file, append a `checksum` query parameter to the URL.
The paramter value should be in the format of `type:value`, where
type is "md5", "sha1", "sha256", or "sha512". The "value" should be
the actual checksum value. go-getter will parse out this query parameter
automatically and use it to verify the checksum. An example URL
is shown below:

```
./foo.txt?checksum=md5:b7d96c89d09d9e204f5fedc4d5d55b21
```

The checksum query parameter is never sent to the backend protocol
implementation. It is used at a higher level by go-getter itself.
