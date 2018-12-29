---
layout: "docs"
page_title: "artifact Stanza - Job Specification"
sidebar_current: "docs-job-specification-artifact"
description: |-
  The "artifact" stanza instructs Nomad to fetch and unpack a remote resource,
  such as a file, tarball, or binary, and permits downloading artifacts from a
  variety of locations using a URL as the input source.
---

# `artifact` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> task -> **artifact**</code>
    </td>
  </tr>
</table>

The `artifact` stanza instructs Nomad to fetch and unpack a remote resource,
such as a file, tarball, or binary. Nomad downloads artifacts using the popular
[`go-getter`][go-getter] library, which permits downloading artifacts from a
variety of locations using a URL as the input source.

```hcl
job "docs" {
  group "example" {
    task "server" {
      artifact {
        source      = "https://example.com/file.tar.gz"
        destination = "/tmp/directory"
        options {
          checksum = "md5:df6a4178aec9fbdc1d6d7e3634d1bc33"
        }
      }
    }
  }
}
```

Nomad supports downloading `http`, `https`, `git`, `hg` and `S3` artifacts. If
these artifacts are archived (`zip`, `tgz`, `bz2`, `xz`), they are
automatically unarchived before the starting the task.

## `artifact` Parameters

- `destination` `(string: "local/")` - Specifies the directory path to download
  the artifact, relative to the root of the task's directory. If omitted, the
  default value is to place the artifact in `local/`. The destination is treated
  as a directory unless `mode` is set to `file`. Source files will be downloaded
  into that directory path.

- `mode` `(string: "any")` - One of `any`, `file`, or `dir`. If set to `file`
  the `destination` must be a file, not a directory. By default the
  `destination` will be `local/<filename>`.

- `options` `(map<string|string>: nil)` - Specifies configuration parameters to
  fetch the artifact. The key-value pairs map directly to parameters appended to
  the supplied `source` URL. Please see the [`go-getter`
  documentation][go-getter] for a complete list of options and examples

- `source` `(string: <required>)` - Specifies the URL of the artifact to download.
  See [`go-getter`][go-getter] for details.

## `artifact` Examples

The following examples only show the `artifact` stanzas. Remember that the
`artifact` stanza is only valid in the placements listed above.

### Download File

This example downloads the artifact from the provided URL and places it in
`local/file.txt`. The `local/` path is relative to the task's directory.

```hcl
artifact {
  source = "https://example.com/file.txt"
}
```

### Download with Custom Destination

This example downloads the artifact from the provided URL and places it at
`/tmp/example/file.txt`, as specified by the optional `destination` parameter.

```hcl
artifact {
  source      = "https://example.com/file.txt"
  destination = "/tmp/example"
}
```

### Download using git

This example downloads the artifact from the provided GitHub URL and places it at
`local/repo`, as specified by the optional `destination` parameter.

```hcl
artifact {
  source      = "git::https://github.com/example/nomad-examples"
  destination = "local/repo"
}
```

To download from private repo, sshkey need to be set. The key must be
base64-encoded string. Run `base64 -w0 <file>`

```hcl
artifact {
  source      = "git@github.com:example/nomad-examples"
  destination = "local/repo"
  options {
    sshkey = "<string>"
  }
}
```

### Download and Unarchive

This example downloads and unarchives the result in `local/file`. Because the
source URL is an archive extension, Nomad will automatically decompress it:

```hcl
artifact {
  source = "https://example.com/file.tar.gz"
}
```

To disable automatic unarchiving, set the `archive` option to false:

```hcl
artifact {
  source = "https://example.com/file.tar.gz"
  options {
    archive = false
  }
}
```

### Download and Verify Checksums

This example downloads an artifact and verifies the resulting artifact's
checksum before proceeding. If the checksum is invalid, an error will be
returned.

```hcl
artifact {
  source = "https://example.com/file.zip"

  options {
    checksum = "md5:df6a4178aec9fbdc1d6d7e3634d1bc33"
  }
}
```

### Download from an S3-compatible Bucket

These examples download artifacts from Amazon S3. There are several different
types of [S3 bucket addressing][s3-bucket-addr] and [S3 region-specific
endpoints][s3-region-endpoints]. As of Nomad 0.6 non-Amazon S3-compatible
endpoints like [Minio] are supported, but you must explicitly set the "s3::"
prefix.

This example uses path-based notation on a publicly-accessible bucket:

```hcl
artifact {
  source = "https://s3-us-west-2.amazonaws.com/my-bucket-example/my_app.tar.gz"
}
```

If a bucket requires authentication, it may be supplied via the `options`
parameter:

```hcl
artifact {
  options {
    aws_access_key_id     = "<id>"
    aws_access_key_secret = "<secret>"
    aws_access_token      = "<token>"
  }
}
```

To force the S3-specific syntax, use the `s3::` prefix:

```hcl
artifact {
  source = "s3::https://s3-eu-west-1.amazonaws.com/my-bucket-example/my_app.tar.gz"
}
```

Alternatively you can use virtual hosted style:

```hcl
artifact {
  source = "https://my-bucket-example.s3-eu-west-1.amazonaws.com/my_app.tar.gz"
}
```

[go-getter]: https://github.com/hashicorp/go-getter "HashiCorp go-getter Library"
[Minio]: https://www.minio.io/
[s3-bucket-addr]: http://docs.aws.amazon.com/AmazonS3/latest/dev/UsingBucket.html#access-bucket-intro "Amazon S3 Bucket Addressing"
[s3-region-endpoints]: http://docs.aws.amazon.com/general/latest/gr/rande.html#s3_region "Amazon S3 Region Endpoints"
