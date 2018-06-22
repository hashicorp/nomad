---
layout: "docs"
page_title: "server_join Stanza - Agent Configuration"
sidebar_current: "docs-configuration--server-join"
description: |-
  The "server_join" stanza specifies how the Nomad agent will discover and connect to Nomad servers.
---

# `server_join` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>server -> **server_join**</code>
      <br>
      <code>client -> **server_join**</code>
    </td>
  </tr>
</table>

The `server_join` stanza specifies how the Nomad agent will discover and connect
to Nomad servers.

```hcl
server_join {
  retry_join = [ "1.1.1.1", "2.2.2.2" ]
  retry_max = 3
  retry_interval = "15s"
}
```

## `server_join` Parameters

-   `retry_join` `(array<string>: [])` - Specifies a list of server addresses to
  join. This is similar to [`start_join`](#start_join), but will continue to
  be attempted even if the initial join attempt fails, up to
  [retry_max](#retry_max). Further, `retry_join` is available to
  both Nomad servers and clients, while `start_join` is only defined for Nomad
  servers.  This is useful for cases where we know the address will become
  available eventually.  Use `retry_join` with an array as a replacement for
  `start_join`, **do not use both options**.

    Address format includes both using IP addresses as well as an interface to the
  [go-discover](https://github.com/hashicorp/go-discover) library for doing
  automated cluster joining using cloud metadata. See the [Cloud Auto-join](#cloud-auto-join) 
  section below for more information.

    ```
  server_join {
    retry_join = [ "1.1.1.1", "2.2.2.2" ]
  }
  ```

    Using the `go-discover` interface, this can be defined both in a client or
  server configuration as well as provided as a command-line argument.

    ```
  server_join {
    retry_join = [ "provider=aws tag_key=..." ]
  }
  ```

    See the [server address format](#server-address-format) for more information
  about expected server address formats.

- `retry_interval` `(string: "30s")` - Specifies the time to wait between retry
  join attempts.

- `retry_max` `(int: 0)` - Specifies the maximum number of join attempts to be
  made before exiting with a return code of 1. By default, this is set to 0
  which is interpreted as infinite retries.

- `start_join` `(array<string>: [])` - Specifies a list of server addresses to
  join on startup. If Nomad is unable to join with any of the specified
  addresses, agent startup will fail. See the
  [server address format](#server-address-format) section for more information
  on the format of the string. This field is defined only for Nomad servers and
  will result in a configuration parse error if included in a client
  configuration.

## Server Address Format

This section describes the acceptable syntax and format for describing the
location of a Nomad server. There are many ways to reference a Nomad server,
including directly by IP address and resolving through DNS.

### Directly via IP Address

It is possible to address another Nomad server using its IP address. This is
done in the `ip:port` format, such as:

```
1.2.3.4:5678
```

If the port option is omitted, it defaults to the Serf port, which is 4648
unless configured otherwise:

```
1.2.3.4 => 1.2.3.4:4648
```

### Via Domains or DNS

It is possible to address another Nomad server using its DNS address. This is
done in the `address:port` format, such as:

```
nomad-01.company.local:5678
```

If the port option is omitted, it defaults to the Serf port, which is 4648
unless configured otherwise:

```
nomad-01.company.local => nomad-01.company.local:4648
```

### Via the go-discover interface

As of Nomad 0.8.4, `retry_join` accepts a unified interface using the
[go-discover](https://github.com/hashicorp/go-discover) library for doing
automated cluster joining using cloud metadata. See [Cloud
Auto-join][cloud_auto_join] for more information.

```
"provider=aws tag_key=..." => 1.2.3.4:4648
```

## Cloud Auto-join

The following sections describe the Cloud Auto-join `retry_join` options that are specific 
to a subset of supported cloud providers. For information on all providers, see further 
documentation in [go-discover](https://github.com/hashicorp/go-discover).

### Amazon EC2

This returns the first private IP address of all servers in the given
region which have the given `tag_key` and `tag_value`.


```json
{
  "retry_join": ["provider=aws tag_key=... tag_value=..."]
}
```

- `provider` (required) - the name of the provider ("aws" in this case).
- `tag_key` (required) - the key of the tag to auto-join on.
- `tag_value` (required) - the value of the tag to auto-join on.
- `region` (optional) - the AWS region to authenticate in.
- `addr_type` (optional) - the type of address to discover: `private_v4`, `public_v4`, `public_v6`. Default is `private_v4`. (>= 1.0)
- `access_key_id` (optional) - the AWS access key for authentication (see below for more information about authenticating).
- `secret_access_key` (optional) - the AWS secret access key for authentication (see below for more information about authenticating).

#### Authentication &amp; Precedence

- Static credentials `access_key_id=... secret_access_key=...`
- Environment variables (`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`)
- Shared credentials file (`~/.aws/credentials` or the path specified by `AWS_SHARED_CREDENTIALS_FILE`)
- ECS task role metadata (container-specific).
- EC2 instance role metadata.

  The only required IAM permission is `ec2:DescribeInstances`, and it is
  recommended that you make a dedicated key used only for auto-joining. If the
  region is omitted it will be discovered through the local instance's [EC2
  metadata
  endpoint](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html).

### Microsoft Azure

  This returns the first private IP address of all servers in the given region
  which have the given `tag_key` and `tag_value` in the tenant and subscription, or in
  the given `resource_group` of a `vm_scale_set` for Virtual Machine Scale Sets.


  ```json
{
  "retry_join": ["provider=azure tag_name=... tag_value=... tenant_id=... client_id=... subscription_id=... secret_access_key=..."]
}
```

- `provider` (required) - the name of the provider ("azure" in this case).
- `tenant_id` (required) - the tenant to join machines in.
- `client_id` (required) - the client to authenticate with.
- `secret_access_key` (required) - the secret client key.

Use these configuration parameters when using tags:
- `tag_name` - the name of the tag to auto-join on.
- `tag_value` - the value of the tag to auto-join on.

Use these configuration parameters when using Virtual Machine Scale Sets (Consul 1.0.3 and later):
- `resource_group` - the name of the resource group to filter on.
- `vm_scale_set` - the name of the virtual machine scale set to filter on.

    When using tags the only permission needed is the `ListAll` method for `NetworkInterfaces`. When using
    Virtual Machine Scale Sets the only role action needed is `Microsoft.Compute/virtualMachineScaleSets/*/read`.

### Google Compute Engine

This returns the first private IP address of all servers in the given
project which have the given `tag_value`.
```

```json
{
"retry_join": ["provider=gce project_name=... tag_value=..."]
}
```

- `provider` (required) - the name of the provider ("gce" in this case).
- `tag_value` (required) - the value of the tag to auto-join on.
- `project_name` (optional) - the name of the project to auto-join on. Discovered if not set.
- `zone_pattern` (optional) - the list of zones can be restricted through an RE2 compatible regular expression. If omitted, servers in all zones are returned.
- `credentials_file` (optional) - the credentials file for authentication. See below for more information.

#### Authentication &amp; Precedence

- Use credentials from `credentials_file`, if provided.
- Use JSON file from `GOOGLE_APPLICATION_CREDENTIALS` environment variable.
- Use JSON file in a location known to the gcloud command-line tool.
- On Windows, this is `%APPDATA%/gcloud/application_default_credentials.json`.
- On other systems, `$HOME/.config/gcloud/application_default_credentials.json`.
- On Google Compute Engine, use credentials from the metadata
server. In this final case any provided scopes are ignored.

Discovery requires a [GCE Service
Account](https://cloud.google.com/compute/docs/access/service-accounts).
Credentials are searched using the following paths, in order of precedence.

