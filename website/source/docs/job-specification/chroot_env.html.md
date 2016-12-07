---
layout: "docs"
page_title: "chroot_env Stanza - Job Specification"
sidebar_current: "docs-job-specification-chroot_env"
description: |-
  The "chroot_env" stanza configures a key/value mapping that defines the chroot environment for
  the task before starting. This task chroot env has to be a subset of agent chroot env (if configured).
---

# `chroot_env` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> task -> **chroot_env**</code>
    </td>
  </tr>
</table>

The `chroot_env` stanza configures a key/value mapping that defines the chroot environment for
the task before starting. This task chroot env has to be a subset of agent chroot env (if configured).

```hcl
job "docs" {
  group "example" {
    task "server" {
      env {
        my-key = "my-value"
      }
      chroot_env {
          "/bin/ls" = "/bin/ls"
          "/etc/ld.so.cache" = "/etc/ld.so.cache"
          "/etc/ld.so.conf" = "/etc/ld.so.conf"
          "/etc/ld.so.conf.d" = "/etc/ld.so.conf.d"
          "/lib" = "/lib"
          "/lib64" = "/lib64"
      }
    }
  }
}
```
