# Consul Template

[![CircleCI](https://circleci.com/gh/hashicorp/consul-template.svg?style=svg)](https://circleci.com/gh/hashicorp/consul-template)
[![Go Documentation](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/hashicorp/consul-template)

This project provides a convenient way to populate values from [Consul][consul]
into the file system using the `consul-template` daemon.

The daemon `consul-template` queries a [Consul][consul] or [Vault][vault]
cluster and updates any number of specified templates on the file system. As an
added bonus, it can optionally run arbitrary commands when the update process
completes. Please see the [examples folder][examples] for some scenarios where
this functionality might prove useful.

---

**The documentation in this README corresponds to the master branch of Consul Template. It may contain unreleased features or different APIs than the most recently released version.**

**Please see the [Git tag](https://github.com/hashicorp/consul-template/releases) that corresponds to your version of Consul Template for the proper documentation.**

---

## Community Support

If you have questions about how consul-template works, its capabilities or
anything other than a bug or feature request (use github's issue tracker for
those), please see our community support resources.

Community portal: https://discuss.hashicorp.com/c/consul

Other resources: https://www.consul.io/community.html

Additionally, for issues and pull requests, we'll be using the :+1: reactions
as a rough voting system to help gauge community priorities. So please add :+1:
to any issue or pull request you'd like to see worked on. Thanks.


## Installation

1. Download a pre-compiled, released version from the [Consul Template releases page][releases].

1. Extract the binary using `unzip` or `tar`.

1. Move the binary into `$PATH`.

To compile from source, please see the instructions in the
[contributing section](#contributing).

## Quick Example

This short example assumes Consul is installed locally.

1. Start a Consul cluster in dev mode:

    ```shell
    $ consul agent -dev
    ```

1. Author a template `in.tpl` to query the kv store:

    ```liquid
    {{ key "foo" }}
    ```

1. Start Consul Template:

    ```shell
    $ consul-template -template "in.tpl:out.txt" -once
    ```

1. Write data to the key in Consul:

    ```shell
    $ consul kv put foo bar
    ```

1. Observe Consul Template has written the file `out.txt`:

    ```shell
    $ cat out.txt
    bar
    ```

For more examples and use cases, please see the [examples folder][examples] in
this repository.

## Usage

For the full list of options:

```shell
$ consul-template -h
```

### Command Line Flags

The CLI interface supports all options in the configuration file and vice-versa. Here are a few examples of common integrations on the command line.

Render the template on disk at `/tmp/template.ctmpl` to `/tmp/result`:

```shell
$ consul-template \
    -template "/tmp/template.ctmpl:/tmp/result"
```

Render multiple templates in the same process. The optional third argument to
the template is a command that will execute each time the template changes.

```shell
$ consul-template \
    -template "/tmp/nginx.ctmpl:/var/nginx/nginx.conf:nginx -s reload" \
    -template "/tmp/redis.ctmpl:/var/redis/redis.conf:service redis restart" \
    -template "/tmp/haproxy.ctmpl:/var/haproxy/haproxy.conf"
```

Render a template using a custom Consul and Vault address:

```shell
$ consul-template \
    -consul-addr "10.4.4.6:8500" \
    -vault-addr "https://10.5.32.5:8200"
```

Render all templates and then spawn and monitor a child process as a supervisor:

```shell
$ consul-template \
  -template "/tmp/in.ctmpl:/tmp/result" \
  -exec "/sbin/my-server"
```

For more information on supervising, please see the
[Consul Template Exec Mode documentation](#exec-mode).

### Configuration File Format

Configuration files are written in the [HashiCorp Configuration Language][hcl].
By proxy, this means the configuration is also JSON compatible.

```hcl
# This denotes the start of the configuration section for Consul. All values
# contained in this section pertain to Consul.
consul {
  # This block specifies the basic authentication information to pass with the
  # request. For more information on authentication, please see the Consul
  # documentation.
  auth {
    enabled  = true
    username = "test"
    password = "test"
  }

  # This is the address of the Consul agent. By default, this is
  # 127.0.0.1:8500, which is the default bind and port for a local Consul
  # agent. It is not recommended that you communicate directly with a Consul
  # server, and instead communicate with the local Consul agent. There are many
  # reasons for this, most importantly the Consul agent is able to multiplex
  # connections to the Consul server and reduce the number of open HTTP
  # connections. Additionally, it provides a "well-known" IP address for which
  # clients can connect.
  address = "127.0.0.1:8500"

  # This is the ACL token to use when connecting to Consul. If you did not
  # enable ACLs on your Consul cluster, you do not need to set this option.
  #
  # This option is also available via the environment variable CONSUL_TOKEN.
  token = "abcd1234"

  # This controls the retry behavior when an error is returned from Consul.
  # Consul Template is highly fault tolerant, meaning it does not exit in the
  # face of failure. Instead, it uses exponential back-off and retry functions
  # to wait for the cluster to become available, as is customary in distributed
  # systems.
  retry {
    # This enabled retries. Retries are enabled by default, so this is
    # redundant.
    enabled = true

    # This specifies the number of attempts to make before giving up. Each
    # attempt adds the exponential backoff sleep time. Setting this to
    # zero will implement an unlimited number of retries.
    attempts = 12

    # This is the base amount of time to sleep between retry attempts. Each
    # retry sleeps for an exponent of 2 longer than this base. For 5 retries,
    # the sleep times would be: 250ms, 500ms, 1s, 2s, then 4s.
    backoff = "250ms"

    # This is the maximum amount of time to sleep between retry attempts.
    # When max_backoff is set to zero, there is no upper limit to the
    # exponential sleep between retry attempts.
    # If max_backoff is set to 10s and backoff is set to 1s, sleep times
    # would be: 1s, 2s, 4s, 8s, 10s, 10s, ...
    max_backoff = "1m"
  }

  # This block configures the SSL options for connecting to the Consul server.
  ssl {
    # This enables SSL. Specifying any option for SSL will also enable it.
    enabled = true

    # This enables SSL peer verification. The default value is "true", which
    # will check the global CA chain to make sure the given certificates are
    # valid. If you are using a self-signed certificate that you have not added
    # to the CA chain, you may want to disable SSL verification. However, please
    # understand this is a potential security vulnerability.
    verify = false

    # This is the path to the certificate to use to authenticate. If just a
    # certificate is provided, it is assumed to contain both the certificate and
    # the key to convert to an X509 certificate. If both the certificate and
    # key are specified, Consul Template will automatically combine them into an
    # X509 certificate for you.
    cert = "/path/to/client/cert"
    key  = "/path/to/client/key"

    # This is the path to the certificate authority to use as a CA. This is
    # useful for self-signed certificates or for organizations using their own
    # internal certificate authority.
    ca_cert = "/path/to/ca"

    # This is the path to a directory of PEM-encoded CA cert files. If both
    # `ca_cert` and `ca_path` is specified, `ca_cert` is preferred.
    ca_path = "path/to/certs/"

    # This sets the SNI server name to use for validation.
    server_name = "my-server.com"
  }
}

# This is the signal to listen for to trigger a reload event. The default
# value is shown below. Setting this value to the empty string will cause CT
# to not listen for any reload signals.
reload_signal = "SIGHUP"

# This is the signal to listen for to trigger a graceful stop. The default
# value is shown below. Setting this value to the empty string will cause CT
# to not listen for any graceful stop signals.
kill_signal = "SIGINT"

# This is the maximum interval to allow "stale" data. By default, only the
# Consul leader will respond to queries; any requests to a follower will
# forward to the leader. In large clusters with many requests, this is not as
# scalable, so this option allows any follower to respond to a query, so long
# as the last-replicated data is within these bounds. Higher values result in
# less cluster load, but are more likely to have outdated data.
max_stale = "10m"

# This is the log level. If you find a bug in Consul Template, please enable
# debug logs so we can help identify the issue. This is also available as a
# command line flag.
log_level = "warn"

# This is the path to store a PID file which will contain the process ID of the
# Consul Template process. This is useful if you plan to send custom signals
# to the process.
pid_file = "/path/to/pid"

# This is the quiescence timers; it defines the minimum and maximum amount of
# time to wait for the cluster to reach a consistent state before rendering a
# template. This is useful to enable in systems that have a lot of flapping,
# because it will reduce the the number of times a template is rendered.
wait {
  min = "5s"
  max = "10s"
}

# This denotes the start of the configuration section for Vault. All values
# contained in this section pertain to Vault.
vault {
  # This is the address of the Vault leader. The protocol (http(s)) portion
  # of the address is required.
  address = "https://vault.service.consul:8200"

  # This is the grace period between lease renewal of periodic secrets and secret
  # re-acquisition. When renewing a secret, if the remaining lease is less than or
  # equal to the configured grace, Consul Template will request a new credential.
  # This prevents Vault from revoking the credential at expiration and Consul
  # Template having a stale credential.
  #
  # Note: If you set this to a value that is higher than your default TTL or
  # max TTL, Consul Template will always read a new secret!
  #
  # This should also be less than or around 1/3 of your TTL for a predictable
  # behaviour. See https://github.com/hashicorp/vault/issues/3414
  grace = "5m"

  # This is a Vault Enterprise namespace to use for reading/writing secrets.
  #
  # This value can also be specified via the environment variable VAULT_NAMESPACE.
  namespace = "foo"

  # This is the token to use when communicating with the Vault server.
  # Like other tools that integrate with Vault, Consul Template makes the
  # assumption that you provide it with a Vault token; it does not have the
  # incorporated logic to generate tokens via Vault's auth methods.
  #
  # This value can also be specified via the environment variable VAULT_TOKEN.
  # When using a token from Vault Agent, the vault_agent_token_file setting
  # should be used instead, as that will take precedence over this field.
  token = "abcd1234"

  # This tells Consul Template to load the Vault token from the contents of a file.
  # If this field is specified:
  # - Consul Template will not try to renew the Vault token.
  # - Consul Template will periodically stat the file and update the token if it has
  # changed.
  # vault_agent_token_file = "/tmp/vault/agent/token"

  # This tells Consul Template that the provided token is actually a wrapped
  # token that should be unwrapped using Vault's cubbyhole response wrapping
  # before being used. Please see Vault's cubbyhole response wrapping
  # documentation for more information.
  unwrap_token = true

  # This option tells Consul Template to automatically renew the Vault token
  # given. If you are unfamiliar with Vault's architecture, Vault requires
  # tokens be renewed at some regular interval or they will be revoked. Consul
  # Template will automatically renew the token at half the lease duration of
  # the token. The default value is true, but this option can be disabled if
  # you want to renew the Vault token using an out-of-band process.
  #
  # Note that secrets specified in a template (using {{secret}} for example)
  # are always renewed, even if this option is set to false. This option only
  # applies to the top-level Vault token itself.
  renew_token = true

  # This section details the retry options for connecting to Vault. Please see
  # the retry options in the Consul section for more information (they are the
  # same).
  retry {
    # ...
  }

  # This section details the SSL options for connecting to the Vault server.
  # Please see the SSL options in the Consul section for more information (they
  # are the same).
  ssl {
    # ...
  }
}

# This block defines the configuration for connecting to a syslog server for
# logging.
syslog {
  # This enables syslog logging. Specifying any other option also enables
  # syslog logging.
  enabled = true

  # This is the name of the syslog facility to log to.
  facility = "LOCAL5"
}

# This block defines the configuration for de-duplication mode. Please see the
# de-duplication mode documentation later in the README for more information
# on how de-duplication mode operates.
deduplicate {
  # This enables de-duplication mode. Specifying any other options also enables
  # de-duplication mode.
  enabled = true

  # This is the prefix to the path in Consul's KV store where de-duplication
  # templates will be pre-rendered and stored.
  prefix = "consul-template/dedup/"
}

# This block defines the configuration for exec mode. Please see the exec mode
# documentation at the bottom of this README for more information on how exec
# mode operates and the caveats of this mode.
exec {
  # This is the command to exec as a child process. There can be only one
  # command per Consul Template process.
  command = "/usr/bin/app"

  # This is a random splay to wait before killing the command. The default
  # value is 0 (no wait), but large clusters should consider setting a splay
  # value to prevent all child processes from reloading at the same time when
  # data changes occur. When this value is set to non-zero, Consul Template
  # will wait a random period of time up to the splay value before reloading
  # or killing the child process. This can be used to prevent the thundering
  # herd problem on applications that do not gracefully reload.
  splay = "5s"

  env {
    # This specifies if the child process should not inherit the parent
    # process's environment. By default, the child will have full access to the
    # environment variables of the parent. Setting this to true will send only
    # the values specified in `custom_env` to the child process.
    pristine = false

    # This specifies additional custom environment variables in the form shown
    # below to inject into the child's runtime environment. If a custom
    # environment variable shares its name with a system environment variable,
    # the custom environment variable takes precedence. Even if pristine,
    # whitelist, or blacklist is specified, all values in this option
    # are given to the child process.
    custom = ["PATH=$PATH:/etc/myapp/bin"]

    # This specifies a list of environment variables to exclusively include in
    # the list of environment variables exposed to the child process. If
    # specified, only those environment variables matching the given patterns
    # are exposed to the child process. These strings are matched using Go's
    # glob function, so wildcards are permitted.
    whitelist = ["CONSUL_*"]

    # This specifies a list of environment variables to exclusively prohibit in
    # the list of environment variables exposed to the child process. If
    # specified, any environment variables matching the given patterns will not
    # be exposed to the child process, even if they are whitelisted. The values
    # in this option take precedence over the values in the whitelist.
    # These strings are matched using Go's glob function, so wildcards are
    # permitted.
    blacklist = ["VAULT_*"]
  }

  # This defines the signal that will be sent to the child process when a
  # change occurs in a watched template. The signal will only be sent after the
  # process is started, and the process will only be started after all
  # dependent templates have been rendered at least once. The default value is
  # nil, which tells Consul Template to stop the child process and spawn a new
  # one instead of sending it a signal. This is useful for legacy applications
  # or applications that cannot properly reload their configuration without a
  # full reload.
  reload_signal = ""

  # This defines the signal sent to the child process when Consul Template is
  # gracefully shutting down. The application should begin a graceful cleanup.
  # If the application does not terminate before the `kill_timeout`, it will
  # be terminated (effectively "kill -9"). The default value is "SIGTERM".
  kill_signal = "SIGINT"

  # This defines the amount of time to wait for the child process to gracefully
  # terminate when Consul Template exits. After this specified time, the child
  # process will be force-killed (effectively "kill -9"). The default value is
  # "30s".
  kill_timeout = "2s"
}

# This block defines the configuration for a template. Unlike other blocks,
# this block may be specified multiple times to configure multiple templates.
# It is also possible to configure templates via the CLI directly.
template {
  # This is the source file on disk to use as the input template. This is often
  # called the "Consul Template template". This option is required if not using
  # the `contents` option.
  source = "/path/on/disk/to/template.ctmpl"

  # This is the destination path on disk where the source template will render.
  # If the parent directories do not exist, Consul Template will attempt to
  # create them, unless create_dest_dirs is false.
  destination = "/path/on/disk/where/template/will/render.txt"

  # This options tells Consul Template to create the parent directories of the
  # destination path if they do not exist. The default value is true.
  create_dest_dirs = true

  # This option allows embedding the contents of a template in the configuration
  # file rather then supplying the `source` path to the template file. This is
  # useful for short templates. This option is mutually exclusive with the
  # `source` option.
  contents = "{{ keyOrDefault \"service/redis/maxconns@east-aws\" \"5\" }}"

  # This is the optional command to run when the template is rendered. The
  # command will only run if the resulting template changes. The command must
  # return within 30s (configurable), and it must have a successful exit code.
  # Consul Template is not a replacement for a process monitor or init system.
  command = "restart service foo"

  # This is the maximum amount of time to wait for the optional command to
  # return. Default is 30s.
  command_timeout = "60s"

  # Exit with an error when accessing a struct or map field/key that does not
  # exist. The default behavior will print "<no value>" when accessing a field
  # that does not exist. It is highly recommended you set this to "true" when
  # retrieving secrets from Vault.
  error_on_missing_key = false

  # This is the permission to render the file. If this option is left
  # unspecified, Consul Template will attempt to match the permissions of the
  # file that already exists at the destination path. If no file exists at that
  # path, the permissions are 0644.
  perms = 0600

  # This option backs up the previously rendered template at the destination
  # path before writing a new one. It keeps exactly one backup. This option is
  # useful for preventing accidental changes to the data without having a
  # rollback strategy.
  backup = true

  # These are the delimiters to use in the template. The default is "{{" and
  # "}}", but for some templates, it may be easier to use a different delimiter
  # that does not conflict with the output file itself.
  left_delimiter  = "{{"
  right_delimiter = "}}"

  # This is the `minimum(:maximum)` to wait before rendering a new template to
  # disk and triggering a command, separated by a colon (`:`). If the optional
  # maximum value is omitted, it is assumed to be 4x the required minimum value.
  # This is a numeric time with a unit suffix ("5s"). There is no default value.
  # The wait value for a template takes precedence over any globally-configured
  # wait.
  wait {
    min = "2s"
    max = "10s"
  }
}

```

Note that not all fields are required. If you are not retrieving secrets from
Vault, you do not need to specify a Vault configuration section. Similarly, if
you are not logging to syslog, you do not need to specify a syslog
configuration.

For additional security, tokens may also be read from the environment using the
`CONSUL_TOKEN` or `VAULT_TOKEN` environment variables respectively. It is highly
recommended that you do not put your tokens in plain-text in a configuration
file.

Instruct Consul Template to use a configuration file with the `-config` flag:

```shell
$ consul-template -config "/my/config.hcl"
```

This argument may be specified multiple times to load multiple configuration
files. The right-most configuration takes the highest precedence. If the path to
a directory is provided (as opposed to the path to a file), all of the files in
the given directory will be merged in
[lexical order](http://golang.org/pkg/path/filepath/#Walk), recursively. Please
note that symbolic links are _not_ followed.

**Commands specified on the CLI take precedence over a config file!**

### Templating Language

Consul Template parses files authored in the [Go Template][text-template]
format. If you are not familiar with the syntax, please read Go's documentation
and examples. In addition to the Go-provided template functions, Consul Template
provides the following functions:

#### API Functions

API functions interact with remote API calls, communicating with external
services like [Consul][consul] and [Vault][vault].

##### `datacenters`

Query [Consul][consul] for all datacenters in its catalog.

```liquid
{{ datacenters }}
```

For example:

```liquid
{{ range datacenters }}
{{ . }}{{ end }}
```

renders

```text
dc1
dc2
```

An optional boolean can be specified which instructs Consul Template to ignore
datacenters which are inaccessible or do not have a current leader. Enabling
this option requires an O(N+1) operation and therefore is not recommended in
environments where performance is a factor.

```liquid
// Ignores datacenters which are inaccessible
{{ datacenters true }}
```

##### `file`

Read and output the contents of a local file on disk. If the file cannot be
read, an error will occur. When the file changes, Consul Template will pick up
the change and re-render the template.

```liquid
{{ file "<PATH>" }}
```

For example:

```liquid
{{ file "/path/to/my/file" }}
```

renders

```text
file contents
```

This does not process nested templates. See
[`executeTemplate`](#executeTemplate) for a way to render nested templates.

##### `key`

Query [Consul][consul] for the value at the given key path. If the key does not
exist, Consul Template will block rendering until the key is present. To avoid
blocking, use `keyOrDefault` or `keyExists`.

```liquid
{{ key "<PATH>@<DATACENTER>" }}
```

The `<DATACENTER>` attribute is optional; if omitted, the local datacenter is
used.

For example:

```liquid
{{ key "service/redis/maxconns" }}
```

renders

```text
15
```

##### `keyExists`

Query [Consul][consul] for the value at the given key path. If the key exists,
this will return true, false otherwise. Unlike `key`, this function will not
block if the key does not exist. This is useful for controlling flow.

```liquid
{{ keyExists "<PATH>@<DATACENTER>" }}
```

The `<DATACENTER>` attribute is optional; if omitted, the local datacenter is
used.

For example:

```liquid
{{ if keyExists "app/beta_active" }}
  # ...
{{ else }}
  # ...
{{ end }}
```

##### `keyOrDefault`

Query [Consul][consul] for the value at the given key path. If the key does not
exist, the default value will be used instead. Unlike `key`, this function will
not block if the key does not exist.

```liquid
{{ keyOrDefault "<PATH>@<DATACENTER>" "<DEFAULT>" }}
```

The `<DATACENTER>` attribute is optional; if omitted, the local datacenter is
used.

For example:

```liquid
{{ keyOrDefault "service/redis/maxconns" "5" }}
```

renders

```text
5
```

Note that Consul Template uses a [multi-phase
execution](#multi-phase-execution). During the first phase of evaluation, Consul
Template will have no data from Consul and thus will _always_ fall back to the
default value. Subsequent reads from Consul will pull in the real value from
Consul (if the key exists) on the next template pass. This is important because
it means that Consul Template will never "block" the rendering of a template due
to a missing key from a `keyOrDefault`. Even if the key exists, if Consul has
not yet returned data for the key, the default value will be used instead.

##### `ls`

Query [Consul][consul] for all top-level kv pairs at the given key path.

```liquid
{{ ls "<PATH>@<DATACENTER>" }}
```

The `<DATACENTER>` attribute is optional; if omitted, the local datacenter is
used.

For example:

```liquid
{{ range ls "service/redis" }}
{{ .Key }}:{{ .Value }}{{ end }}
```

renders

```text
maxconns:15
minconns:5
```

##### `node`

Query [Consul][consul] for a node in the catalog.

```liquid
{{node "<NAME>@<DATACENTER>"}}
```

The `<NAME>` attribute is optional; if omitted, the local agent node is used.

The `<DATACENTER>` attribute is optional; if omitted, the local datacenter is
used.

For example:

```liquid
{{ with node }}
{{ .Node.Address }}{{ end }}
```

renders

```text
10.5.2.6
```

To query a different node:

```liquid
{{ with node "node1@dc2" }}
{{ .Node.Address }}{{ end }}
```

renders

```text
10.4.2.6
```

To access map data such as `TaggedAddresses` or `Meta`, use
[Go's text/template][text-template] map indexing.

##### `nodes`

Query [Consul][consul] for all nodes in the catalog.

```liquid
{{ nodes "@<DATACENTER>~<NEAR>" }}
```

The `<DATACENTER>` attribute is optional; if omitted, the local datacenter is
used.

The `<NEAR>` attribute is optional; if omitted, results are specified in lexical
order. If provided a node name, results are ordered by shortest round-trip time
to the provided node. If provided `_agent`, results are ordered by shortest
round-trip time to the local agent.

For example:

```liquid
{{ range nodes }}
{{ .Address }}{{ end }}
```

renders

```text
10.4.2.13
10.46.2.5
```

To query a different data center and order by shortest trip time to ourselves:

```liquid
{{ range nodes "@dc2~_agent" }}
{{ .Address }}{{ end }}
```

To access map data such as `TaggedAddresses` or `Meta`, use
[Go's text/template][text-template] map indexing.

##### `secret`

Query [Vault][vault] for the secret at the given path.

```liquid
{{ secret "<PATH>" "<DATA>" }}
```

The `<DATA>` attribute is optional; if omitted, the request will be a `vault
read` (HTTP GET) request. If provided, the request will be a `vault write` (HTTP
PUT/POST) request.

For example:

```liquid
{{ with secret "secret/passwords" }}
{{ .Data.wifi }}{{ end }}
```

renders

```text
FORWARDSoneword
```

To access a versioned secret value (for the K/V version 2 backend):

```liquid
{{ with secret "secret/passwords?version=1" }}
{{ .Data.data.wifi }}{{ end }}
```

When omitting the `?version` parameter, the latest version of the secret will be
fetched. Note the nested `.Data.data` syntax when referencing the secret value.
For more information about using the K/V v2 backend, see the 
[Vault Documentation](https://www.vaultproject.io/docs/secrets/kv/kv-v2.html).

When using Vault versions 0.10.0/0.10.1, the secret path will have to be prefixed
with "data", i.e. `secret/data/passwords` for the example above. This is not
necessary for Vault versions after 0.10.1, as consul-template will detect the KV
backend version being used. The version 2 KV backend did not exist prior to 0.10.0,
so these are the only affected versions.

An example using write to generate PKI certificates:

```liquid
{{ with secret "pki/issue/my-domain-dot-com" "common_name=foo.example.com" }}
{{ .Data.certificate }}{{ end }}
```

The parameters must be `key=value` pairs, and each pair must be its own argument
to the function:

Please always consider the security implications of having the contents of a
secret in plain-text on disk. If an attacker is able to get access to the file,
they will have access to plain-text secrets.

Please note that Vault does not support blocking queries. As a result, Consul
Template will not immediately reload in the event a secret is changed as it does
with Consul's key-value store. Consul Template will fetch a new secret at half
the lease duration of the original secret. For example, most items in Vault's
generic secret backend have a default 30 day lease. This means Consul Template
will renew the secret every 15 days. As such, it is recommended that a smaller
lease duration be used when generating the initial secret to force Consul
Template to renew more often.

Also consider enabling `error_on_missing_key` when working with templates that
will interact with Vault. By default, Consul Template uses Go's templating
language. When accessing a struct field or map key that does not exist, it
defaults to printing "<no value>". This may not be the desired behavior,
especially when working with passwords or other data. As such, it is recommended
you set:

```hcl
template {
  error_on_missing_key = true
}
```

You can also guard against empty values using `if` or `with` blocks.

```liquid
{{ with secret "secret/foo"}}
{{ if .Data.password }}
password = "{{ .Data.password }}"
{{ end }}
{{ end }}
```

##### `secrets`

Query [Vault][vault] for the list of secrets at the given path. Not all
endpoints support listing.

```liquid
{{ secrets "<PATH>" }}
```

For example:

```liquid
{{ range secrets "secret/" }}
{{ . }}{{ end }}
```

renders

```text
bar
foo
zip
```

To iterate and list over every secret in the generic secret backend in Vault:

```liquid
{{ range secrets "secret/" }}
{{ with secret (printf "secret/%s" .) }}{{ range $k, $v := .Data }}
{{ $k }}: {{ $v }}
{{ end }}{{ end }}{{ end }}
```

You should probably never do this.

Please also note that Vault does not support
blocking queries. To understand the implications, please read the note at the
end of the `secret` function.

##### `service`

Query [Consul][consul] for services based on their health.

```liquid
{{ service "<TAG>.<NAME>@<DATACENTER>~<NEAR>|<FILTER>" }}
```

The `<TAG>` attribute is optional; if omitted, all nodes will be queried.

The `<DATACENTER>` attribute is optional; if omitted, the local datacenter is
used.

The `<NEAR>` attribute is optional; if omitted, results are specified in lexical
order. If provided a node name, results are ordered by shortest round-trip time
to the provided node. If provided `_agent`, results are ordered by shortest
round-trip time to the local agent.

The `<FILTER>` attribute is optional; if omitted, only health services are
returned. Providing a filter allows for client-side filtering of services.

For example:

The example above is querying Consul for healthy "web" services, in the "east-aws" data center. The tag and data center attributes are optional. To query all nodes of the "web" service (regardless of tag) for the current data center:

```liquid
{{ range service "web" }}
server {{ .Name }}{{ .Address }}:{{ .Port }}{{ end }}
```

renders the IP addresses of all _healthy_ nodes with a logical service named
"web":

```text
server web01 10.5.2.45:2492
server web02 10.2.6.61:2904
```

To access map data such as `NodeTaggedAddresses` or `NodeMeta`, use
[Go's text/template][text-template] map indexing.

By default only healthy services are returned. To list all services, pass the
"any" filter:

```liquid
{{ service "web|any" }}
```

This will return all services registered to the agent, regardless of their
status.

To filter services by a specific set of healths, specify a comma-separated list
of health statuses:

```liquid
{{ service "web|passing,warning" }}
```

This will returns services which are deemed "passing" or "warning" according to
their node and service-level checks defined in Consul. Please note that the
comma implies an "or", not an "and".

**Note:** Due to the use of dot `.` to delimit TAG, the `service` command will
not recognize service names containing dots.

**Note:** There is an architectural difference between the following:

```liquid
{{ service "web" }}
{{ service "web|passing" }}
```

The former will return all services which Consul considers "healthy" and
passing. The latter will return all services registered with the Consul agent
and perform client-side filtering. As a general rule, do not use the "passing"
argument alone if you want only healthy services - simply omit the second
argument instead.


##### `services`

Query [Consul][consul] for all services in the catalog.

```liquid
{{ services "@<DATACENTER>" }}
```

The `<DATACENTER>` attribute is optional; if omitted, the local datacenter is
used.

For example:

```liquid
{{ range services }}
{{ .Name }}: {{ .Tags | join "," }}{{ end }}
```

renders

```text
node01 tag1,tag2,tag3
```

##### `tree`

Query [Consul][consul] for all kv pairs at the given key path.

```liquid
{{ tree "<PATH>@<DATACENTER>" }}
```

The `<DATACENTER>` attribute is optional; if omitted, the local datacenter is
used.

For example:

```liquid
{{ range tree "service/redis" }}
{{ .Key }}:{{ .Value }}{{ end }}
```
renders

```text
minconns 2
maxconns 12
nested/config/value "value"
```

Unlike `ls`, `tree` returns **all** keys under the prefix, just like the Unix
`tree` command.

---

#### Scratch

The scratchpad (or "scratch" for short) is available within the context of a
template to store temporary data or computations. Scratch data is not shared
between templates and is not cached between invocations.

##### `scratch.Key`

Returns a boolean if data exists in the scratchpad at the named key. Even if the
data at that key is "nil", this still returns true.

```liquid
{{ scratch.Key "foo" }}
```

##### `scratch.Get`

Returns the value in the scratchpad at the named key. If the data does not
exist, this will return "nil".

```liquid
{{ scratch.Get "foo" }}
```

##### `scratch.Set`

Saves the given value at the given key. If data already exists at that key, it
is overwritten.

```liquid
{{ scratch.Set "foo" "bar" }}
```

##### `scratch.SetX`

This behaves exactly the same as `Set`, but does not overwrite if the value
already exists.

```liquid
{{ scratch.SetX "foo" "bar" }}
```

##### `scratch.MapSet`

Saves a value in a named key in the map. If data already exists at that key, it
is overwritten.

```liquid
{{ scratch.MapSet "vars" "foo" "bar" }}
```

##### `scratch.MapSetX`

This behaves exactly the same as `MapSet`, but does not overwrite if the value
already exists.

```liquid
{{ scratch.MapSetX "vars" "foo" "bar" }}
```

##### `scratch.MapValues`

Returns a sorted list (by key) of all values in the named map.

```liquid
{{ scratch.MapValues "vars" }}
```

---

#### Helper Functions

Unlike API functions, helper functions do not query remote services. These
functions are useful for parsing data, formatting data, performing math, etc.

##### `base64Decode`

Accepts a base64-encoded string and returns the decoded result, or an error if
the given string is not a valid base64 string.

```liquid
{{ base64Decode "aGVsbG8=" }}
```

renders

```text
hello
```

##### `base64Encode`

Accepts a string and returns a base64-encoded string.

```liquid
{{ base64Encode "hello" }}
```

renders

```text
aGVsbG8=
```

##### `base64URLDecode`

Accepts a base64-encoded URL-safe string and returns the decoded result, or an
error if the given string is not a valid base64 URL-safe string.

```liquid
{{ base64URLDecode "aGVsbG8=" }}
```

renders

```text
hello
```

##### `base64URLEncode`

Accepts a string and returns a base-64 encoded URL-safe string.

```liquid
{{ base64Encode "hello" }}
```

renders

```text
aGVsbG8=
```

##### `byKey`

Accepts a list of pairs returned from a [`tree`](#tree) call and creates a map that groups pairs by their top-level directory.

For example:

```text
groups/elasticsearch/es1
groups/elasticsearch/es2
groups/elasticsearch/es3
services/elasticsearch/check_elasticsearch
services/elasticsearch/check_indexes
```

with the following template

```liquid
{{ range $key, $pairs := tree "groups" | byKey }}{{ $key }}:
{{ range $pair := $pairs }}  {{ .Key }}={{ .Value }}
{{ end }}{{ end }}
```

renders

```text
elasticsearch:
  es1=1
  es2=1
  es3=1
```

Note that the top-most key is stripped from the Key value. Keys that have no
prefix after stripping are removed from the list.

The resulting pairs are keyed as a map, so it is possible to look up a single
value by key:

```liquid
{{ $weights := tree "weights" }}
{{ range service "release.web" }}
  {{ $weight := or (index $weights .Node) 100 }}
  server {{ .Node }} {{ .Address }}:{{ .Port }} weight {{ $weight }}{{ end }}
```

##### `byTag`

Takes the list of services returned by the [`service`](#service) or
[`services`](#services) function and creates a map that groups services by tag.

```liquid
{{ range $tag, $services := service "web" | byTag }}{{ $tag }}
{{ range $services }} server {{ .Name }} {{ .Address }}:{{ .Port }}
{{ end }}{{ end }}
```

##### `contains`

Determines if a needle is within an iterable element.

```liquid
{{ if .Tags | contains "production" }}
# ...
{{ end }}
```

#### `containsAll`

Returns `true` if all needles are within an iterable element, or `false`
otherwise. Returns `true` if the list of needles is empty.

```liquid
{{ if containsAll $requiredTags .Tags }}
# ...
{{ end }}
```

#### `containsAny`

Returns `true` if any needle is within an iterable element, or `false`
otherwise. Returns `false` if the list of needles is empty.

```liquid
{{ if containsAny $acceptableTags .Tags }}
# ...
{{ end }}
```

#### `containsNone`

Returns `true` if no needles are within an iterable element, or `false`
otherwise. Returns `true` if the list of needles is empty.

```liquid
{{ if containsNone $forbiddenTags .Tags }}
# ...
{{ end }}
```

#### `containsNotAll`

Returns `true` if some needle is not within an iterable element, or `false`
otherwise. Returns `false` if the list of needles is empty.

```liquid
{{ if containsNotAll $excludingTags .Tags }}
# ...
{{ end }}
```

##### `env`

Reads the given environment variable accessible to the current process.

```liquid
{{ env "CLUSTER_ID" }}
```

This function can be chained to manipulate the output:

```liquid
{{ env "CLUSTER_ID" | toLower }}
```

Reads the given environment variable and if it does not exist or is blank use a default value, ex `12345`.

```liquid
{{ or (env "CLUSTER_ID") "12345" }}
```

##### `executeTemplate`

Executes and returns a defined template.

```liquid
{{ define "custom" }}my custom template{{ end }}

This is my other template:
{{ executeTemplate "custom" }}

And I can call it multiple times:
{{ executeTemplate "custom" }}

Even with a new context:
{{ executeTemplate "custom" 42 }}

Or save it to a variable:
{{ $var := executeTemplate "custom" }}
```

##### `explode`

Takes the result from a `tree` or `ls` call and converts it into a deeply-nested
map for parsing/traversing.

```liquid
{{ tree "config" | explode }}
```

Note: You will lose any metadata about the key-pair after it has been exploded.
You can also access deeply nested values:

```liquid
{{ with tree "config" | explode }}
{{ .a.b.c }}{{ end }}
```

You will need to have a reasonable format about your data in Consul. Please see
[Go's text/template package][text-template] for more information.


##### `indent`

Indents a block of text by prefixing N number of spaces per line.

```liquid
{{ tree "foo" | explode | toYAML | indent 4 }}
```

##### `in`

Determines if a needle is within an iterable element.

```liquid
{{ if in .Tags "production" }}
# ...
{{ end }}
```

##### `loop`

Accepts varying parameters and differs its behavior based on those parameters.

If `loop` is given one integer, it will return a goroutine that begins at zero
and loops up to but not including the given integer:

```liquid
{{ range loop 5 }}
# Comment{{end}}
```

If given two integers, this function will return a goroutine that begins at
the first integer and loops up to but not including the second integer:

```liquid
{{ range $i := loop 5 8 }}
stanza-{{ $i }}{{ end }}
```

which would render:

```text
stanza-5
stanza-6
stanza-7
```

Note: It is not possible to get the index and the element since the function
returns a goroutine, not a slice. In other words, the following is **not
valid**:

```liquid
# Will NOT work!
{{ range $i, $e := loop 5 8 }}
# ...{{ end }}
```

##### `join`

Takes the given list of strings as a pipe and joins them on the provided string:

```liquid
{{ $items | join "," }}
```

##### `trimSpace`

Takes the provided input and trims all whitespace, tabs and newlines:

```liquid
{{ file "/etc/ec2_version" | trimSpace }}
```

##### `parseBool`

Takes the given string and parses it as a boolean:

```liquid
{{ "true" | parseBool }}
```

This can be combined with a key and a conditional check, for example:

```liquid
{{ if key "feature/enabled" | parseBool }}{{ end }}
```

##### `parseFloat`

Takes the given string and parses it as a base-10 float64:

```liquid
{{ "1.2" | parseFloat }}
```

##### `parseInt`

Takes the given string and parses it as a base-10 int64:

```liquid
{{ "1" | parseInt }}
```

This can be combined with other helpers, for example:

```liquid
{{ range $i := loop key "config/pool_size" | parseInt }}
# ...{{ end }}
```

##### `parseJSON`

Takes the given input (usually the value from a key) and parses the result as
JSON:

```liquid
{{ with $d := key "user/info" | parseJSON }}{{ $d.name }}{{ end }}
```

Note: Consul Template evaluates the template multiple times, and on the first
evaluation the value of the key will be empty (because no data has been loaded
yet). This means that templates must guard against empty responses.

##### `parseUint`

Takes the given string and parses it as a base-10 int64:

```liquid
{{ "1" | parseUint }}
```

##### `plugin`

Takes the name of a plugin and optional payload and executes a Consul Template
plugin.

```liquid
{{ plugin "my-plugin" }}
```

The plugin can take an arbitrary number of string arguments, and can be the 
target of a pipeline that produces strings as well. This is most commonly 
combined with a JSON filter for customization:

```liquid
{{ tree "foo" | explode | toJSON | plugin "my-plugin" }}
```

Please see the [plugins](#plugins) section for more information about plugins.

##### `regexMatch`

Takes the argument as a regular expression and will return `true` if it matches
on the given string, or `false` otherwise.

```liquid
{{ if "foo.bar" | regexMatch "foo([.a-z]+)" }}
# ...
{{ else }}
# ...
{{ end }}
```

##### `regexReplaceAll`

Takes the argument as a regular expression and replaces all occurrences of the
regex with the given string. As in go, you can use variables like $1 to refer to
subexpressions in the replacement string.

```liquid
{{ "foo.bar" | regexReplaceAll "foo([.a-z]+)" "$1" }}
```

##### `replaceAll`

Takes the argument as a string and replaces all occurrences of the given string
with the given string.

```liquid
{{ "foo.bar" | replaceAll "." "_" }}
```

This function can be chained with other functions as well:

```liquid
{{ service "web" }}{{ .Name | replaceAll ":" "_" }}{{ end }}
```

##### `split`

Splits the given string on the provided separator:

```liquid
{{ "foo\nbar\n" | split "\n" }}
```

This can be combined with chained and piped with other functions:

```liquid
{{ key "foo" | toUpper | split "\n" | join "," }}
```

##### `timestamp`

Returns the current timestamp as a string (UTC). If no arguments are given, the
result is the current RFC3339 timestamp:

```liquid
{{ timestamp }} // e.g. 1970-01-01T00:00:00Z
```

If the optional parameter is given, it is used to format the timestamp. The
magic reference date **Mon Jan 2 15:04:05 -0700 MST 2006** can be used to format
the date as required:

```liquid
{{ timestamp "2006-01-02" }} // e.g. 1970-01-01
```

See [Go's `time.Format`](http://golang.org/pkg/time/#Time.Format) for more
information.

As a special case, if the optional parameter is `"unix"`, the unix timestamp in
seconds is returned as a string.

```liquid
{{ timestamp "unix" }} // e.g. 0
```

##### `toJSON`

Takes the result from a `tree` or `ls` call and converts it into a JSON object.

```liquid
{{ tree "config" | explode | toJSON }}
```

renders

```javascript
{"admin":{"port":"1234"},"maxconns":"5","minconns":"2"}
```

Note: Consul stores all KV data as strings. Thus true is "true", 1 is "1", etc.

##### `toJSONPretty`

Takes the result from a `tree` or `ls` call and converts it into a
pretty-printed JSON object, indented by two spaces.

```liquid
{{ tree "config" | explode | toJSONPretty }}
```

renders

```javascript
{
  "admin": {
    "port": "1234"
  },
  "maxconns": "5",
  "minconns": "2",
}
```

Note: Consul stores all KV data as strings. Thus true is "true", 1 is "1", etc.

##### `toLower`

Takes the argument as a string and converts it to lowercase.

```liquid
{{ key "user/name" | toLower }}
```

See [Go's `strings.ToLower`](http://golang.org/pkg/strings/#ToLower) for more
information.

##### `toTitle`

Takes the argument as a string and converts it to titlecase.

```liquid
{{ key "user/name" | toTitle }}
```

See [Go's `strings.Title`](http://golang.org/pkg/strings/#Title) for more
information.

##### `toTOML`

Takes the result from a `tree` or `ls` call and converts it into a TOML object.

```liquid
{{ tree "config" | explode | toTOML }}
```

renders

```toml
maxconns = "5"
minconns = "2"

[admin]
  port = "1134"
```

Note: Consul stores all KV data as strings. Thus true is "true", 1 is "1", etc.

##### `toUpper`

Takes the argument as a string and converts it to uppercase.

```liquid
{{ key "user/name" | toUpper }}
```

See [Go's `strings.ToUpper`](http://golang.org/pkg/strings/#ToUpper) for more
information.

##### `toYAML`

Takes the result from a `tree` or `ls` call and converts it into a
pretty-printed YAML object, indented by two spaces.

```liquid
{{ tree "config" | explode | toYAML }}
```

renders

```yaml
admin:
  port: "1234"
maxconns: "5"
minconns: "2"
```

Note: Consul stores all KV data as strings. Thus true is "true", 1 is "1", etc.

---

#### Math Functions

The following functions are available on floats and integer values.

##### `add`

Returns the sum of the two values.

```liquid
{{ add 1 2 }} // 3
```

This can also be used with a pipe function.

```liquid
{{ 1 | add 2 }} // 3
```

##### `subtract`

Returns the difference of the second value from the first.

```liquid
{{ subtract 2 5 }} // 3
```

This can also be used with a pipe function.

```liquid
{{ 5 | subtract 2 }} // 3
```

Please take careful note of the order of arguments.

##### `multiply`

Returns the product of the two values.

```liquid
{{ multiply 2 2 }} // 4
```

This can also be used with a pipe function.

```liquid
{{ 2 | multiply 2 }} // 4
```

##### `divide`

Returns the division of the second value from the first.

```liquid
{{ divide 2 10 }} // 5
```

This can also be used with a pipe function.

```liquid
{{ 10 | divide 2 }} // 5
```

Please take careful note of the order or arguments.

##### `modulo`

Returns the modulo of the second value from the first.

```liquid
{{ modulo 2 5 }} // 1
```

This can also be used with a pipe function.

```liquid
{{ 5 | modulo 2 }} // 1
```

Please take careful note of the order of arguments.

## Plugins

### Authoring Plugins

For some use cases, it may be necessary to write a plugin that offloads work to
another system. This is especially useful for things that may not fit in the
"standard library" of Consul Template, but still need to be shared across
multiple instances.

Consul Template plugins must have the following API:

```shell
$ NAME [INPUT...]
```

- `NAME` - the name of the plugin - this is also the name of the binary, either
  a full path or just the program name.  It will be executed in a shell with the
  inherited `PATH` so e.g. the plugin `cat` will run the first executable `cat`
  that is found on the `PATH`.

- `INPUT` - input from the template. There will be one INPUT for every argument passed
  to the `plugin` function. If the arguments contain whitespace, that whitespace 
  will be passed as if the argument were quoted by the shell.

#### Important Notes

- Plugins execute user-provided scripts and pass in potentially sensitive data
  from Consul or Vault. Nothing is validated or protected by Consul Template,
  so all necessary precautions and considerations should be made by template
  authors

- Plugin output must be returned as a string on stdout. Only stdout will be
  parsed for output. Be sure to log all errors, debugging messages onto stderr
  to avoid errors when Consul Template returns the value. Note that output to
  stderr will only be output if the plugin returns a non-zero exit code.

- Always `exit 0` or Consul Template will assume the plugin failed to execute

- Ensure the empty input case is handled correctly (see [Multi-phase execution](#multi-phase-execution))

- Data piped into the plugin is appended after any parameters given explicitly (eg `{{ "sample-data" | plugin "my-plugin" "some-parameter"}}` will call `my-plugin some-parameter sample-data`)

Here is a sample plugin in a few different languages that removes any JSON keys
that start with an underscore and returns the JSON string:

```ruby
#! /usr/bin/env ruby
require "json"

if ARGV.empty?
  puts JSON.fast_generate({})
  Kernel.exit(0)
end

hash = JSON.parse(ARGV.first)
hash.reject! { |k, _| k.start_with?("_")  }
puts JSON.fast_generate(hash)
Kernel.exit(0)
```

```go
func main() {
  arg := []byte(os.Args[1])

  var parsed map[string]interface{}
  if err := json.Unmarshal(arg, &parsed); err != nil {
    fmt.Fprintln(os.Stderr, fmt.Sprintf("err: %s", err))
    os.Exit(1)
  }

  for k, _ := range parsed {
    if string(k[0]) == "_" {
      delete(parsed, k)
    }
  }

  result, err := json.Marshal(parsed)
  if err != nil {
    fmt.Fprintln(os.Stderr, fmt.Sprintf("err: %s", err))
    os.Exit(1)
  }

  fmt.Fprintln(os.Stdout, fmt.Sprintf("%s", result))
  os.Exit(0)
}
```

## Caveats

### Dots in Service Names

Using dots `.` in service names will conflict with the use of dots for [TAG
delineation](https://github.com/hashicorp/consul-template#service) in the
template. Dots already [interfere with using
DNS](https://www.consul.io/docs/agent/services.html#service-and-tag-names-with-dns)
for service names, so we recommend avoiding dots wherever possible.

### Once Mode

In Once mode, Consul Template will wait for all dependencies to be rendered. If
a template specifies a dependency (a request) that does not exist in Consul,
once mode will wait until Consul returns data for that dependency. Please note
that "returned data" and "empty data" are not mutually exclusive.

When you query for all healthy services named "foo" (`{{ service "foo" }}`), you
are asking Consul - "give me all the healthy services named foo". If there are
no services named foo, the response is the empty array. This is also the same
response if there are no _healthy_ services named foo.

Consul template processes input templates multiple times, since the first result
could impact later dependencies:

```liquid
{{ range services }}
{{ range service .Name }}
{{ end }}
{{ end }}
```

In this example, we have to process the output of `services` before we can
lookup each `service`, since the inner loops cannot be evaluated until the outer
loop returns a response. Consul Template waits until it gets a response from
Consul for all dependencies before rendering a template. It does not wait until
that response is non-empty though.

**Note:** Once mode implicitly disables any wait/quiescence timers specified in configuration files or passed on the command line.

### Exec Mode

As of version 0.16.0, Consul Template has the ability to maintain an arbitrary
child process (similar to [envconsul](https://github.com/hashicorp/envconsul)).
This mode is most beneficial when running Consul Template in a container or on a
scheduler like [Nomad](https://www.nomadproject.io) or Kubernetes. When
activated, Consul Template will spawn and manage the lifecycle of the child
process.

This mode is best-explained through example. Consider a simple application that
reads a configuration file from disk and spawns a server from that
configuration.

```sh
$ consul-template \
    -template "/tmp/config.ctmpl:/tmp/server.conf" \
    -exec "/bin/my-server -config /tmp/server.conf"
```

When Consul Template starts, it will pull the required dependencies and populate
the `/tmp/server.conf`, which the `my-server` binary consumes. After that
template is rendered completely the first time, Consul Template spawns and
manages a child process. When any of the list templates change, Consul Template
will send a configurable reload signal to the child process. Additionally,
Consul Template will proxy any signals it receives to the child process. This
enables a scheduler to control the lifecycle of the process and also eases the
friction of running inside a container.

A common point of confusion is that the command string behaves the same as the
shell; it does not. In the shell, when you run `foo | bar` or `foo > bar`, that
is actually running as a subprocess of your shell (bash, zsh, csh, etc.). When
Consul Template spawns the exec process, it runs outside of your shell. This
behavior is _different_ from when Consul Template executes the template-specific
reload command. If you want the ability to pipe or redirect in the exec command,
you will need to spawn the process in subshell, for example:

```hcl
exec {
  command = "/bin/bash -c 'my-server > /var/log/my-server.log'"
}
```

Note that when spawning like this, most shells do not proxy signals to their
child by default, so your child process will not receive the signals that Consul
Template sends to the shell. You can avoid this by writing a tiny shell wrapper
and executing that instead:

```bash
#!/usr/bin/env bash
trap "kill -TERM $child" SIGTERM

/bin/my-server -config /tmp/server.conf
child=$!
wait "$child"
```

Alternatively, you can use your shell's exec function directly, if it exists:

```bash
#!/usr/bin/env bash
exec /bin/my-server -config /tmp/server.conf > /var/log/my-server.log
```

There are some additional caveats with Exec Mode, which should be considered
carefully before use:

- If the child process dies, the Consul Template process will also die. Consul
  Template **does not supervise the process!** This is generally the
  responsibility of the scheduler or init system.

- The child process must remain in the foreground. This is a requirement for
  Consul Template to manage the process and send signals.

- The exec command will only start after _all_ templates have been rendered at
  least once. One may have multiple templates for a single Consul Template
  process, all of which must be rendered before the process starts. Consider
  something like an nginx or apache configuration where both the process
  configuration file and individual site configuration must be written in order
  for the service to successfully start.

- After the child process is started, any change to any dependent template will
  cause the reload signal to be sent to the child process. If no reload signal
  is provided, Consul Template will kill the process and spawn a new instance.
  The reload signal can be specified and customized via the CLI or configuration
  file.

- When Consul Template is stopped gracefully, it will send the configurable kill
  signal to the child process. The default value is SIGTERM, but it can be
  customized via the CLI or configuration file.

- Consul Template will forward all signals it receives to the child process
  **except** its defined `reload_signal` and `kill_signal`. If you disable these
  signals, Consul Template will forward them to the child process.

- It is not possible to have more than one exec command (although each template
  can still have its own reload command).

- Individual template reload commands still fire independently of the exec
  command.

### De-Duplication Mode

Consul Template works by parsing templates to determine what data is needed and
then watching Consul for any changes to that data. This allows Consul Template
to efficiently re-render templates when a change occurs. However, if there are
many instances of Consul Template rendering a common template there is a linear
duplication of work as each instance is querying the same data.

To make this pattern more efficient Consul Template supports work de-duplication
across instances. This can be enabled with the `-dedup` flag or via the
`deduplicate` configuration block. Once enabled, Consul Template uses leader
election on a per-template basis to have only a single node perform the queries.
Results are shared among other instances rendering the same template by passing
compressed data through the Consul K/V store.

Please note that no Vault data will be stored in the compressed template.
Because ACLs around Vault are typically more closely controlled than those ACLs
around Consul's KV, Consul Template will still request the secret from Vault on
each iteration.

When running in de-duplication mode, it is important that local template
functions resolve correctly. For example, you may have a local template function
that relies on the `env` helper like this:

```hcl
{{ key (env "KEY") }}
```

It is crucial that the environment variable `KEY` in this example is consistent
across all machines engaged in de-duplicating this template. If the values are
different, Consul Template will be unable to resolve the template, and you will
not get a successful render.

### Termination on Error

By default Consul Template is highly fault-tolerant. If Consul is unreachable or
a template changes, Consul Template will happily continue running. The only
exception to this rule is if the optional `command` exits non-zero. In this
case, Consul Template will also exit non-zero. The reason for this decision is
so the user can easily configure something like Upstart or God to manage Consul
Template as a service.

If you want Consul Template to continue watching for changes, even if the
optional command argument fails, you can append `|| true` to your command. Note
that `||` is a "shell-ism", not a built-in function. You will also need to run
your command under a shell:

```shell
$ consul-template \
  -template "in.ctmpl:out.file:/bin/bash -c 'service nginx restart || true'"
```

In this example, even if the Nginx restart command returns non-zero, the overall
function will still return an OK exit code; Consul Template will continue to run
as a service. Additionally, if you have complex logic for restarting your
service, you can intelligently choose when you want Consul Template to exit and
when you want it to continue to watch for changes. For these types of complex
scripts, we recommend using a custom sh or bash script instead of putting the
logic directly in the `consul-template` command or configuration file.

### Command Environment

The current processes environment is used when executing commands with the following additional environment variables:

- `CONSUL_HTTP_ADDR`
- `CONSUL_HTTP_TOKEN`
- `CONSUL_HTTP_AUTH`
- `CONSUL_HTTP_SSL`
- `CONSUL_HTTP_SSL_VERIFY`

These environment variables are exported with their current values when the
command executes. Other Consul tooling reads these environment variables,
providing smooth integration with other Consul tools (like `consul maint` or
`consul lock`). Additionally, exposing these environment variables gives power
users the ability to further customize their command script.

### Multi-phase Execution

Consul Template does an n-pass evaluation of templates, accumulating
dependencies on each pass. This is required due to nested dependencies, such as:

```liquid
{{ range services }}
{{ range service .Name }}
  {{ .Address }}
{{ end }}{{ end }}
```

During the first pass, Consul Template does not know any of the services in
Consul, so it has to perform a query. When those results are returned, the
inner-loop is then evaluated with that result, potentially creating more queries
and watches.

Because of this implementation, template functions need a default value that is
an acceptable parameter to a `range` function (or similar), but does not
actually execute the inner loop (which would cause a panic). This is important
to mention because complex templates **must** account for the "empty" case. For
example, the following **will not work**:

```liquid
{{ with index (service "foo") 0 }}
# ...
{{ end }}
```

This will raise an error like:

```text
<index $services 0>: error calling index: index out of range: 0
```

That is because, during the _first_ evaluation of the template, the `service`
key is returning an empty slice. You can account for this in your template like
so:

```liquid
{{ with service "foo" }}
{{ with index . 0 }}
{{ .Node }}{{ end }}{{ end }}
```

This will still add the dependency to the list of watches, but will not
evaluate the inner-if, avoiding the out-of-index error.

## Running and Process Lifecycle

While there are multiple ways to run Consul Template, the most common pattern is
to run Consul Template as a system service. When Consul Template first starts,
it reads any configuration files and templates from disk and loads them into
memory. From that point forward, changes to the files on disk do not propagate
to running process without a reload.

The reason for this behavior is simple and aligns with other tools like haproxy.
A user may want to perform pre-flight validation checks on the configuration or
templates before loading them into the process. Additionally, a user may want to
update configuration and templates simultaneously. Having Consul Template
automatically watch and reload those files on changes is both operationally
dangerous and against some of the paradigms of modern infrastructure. Instead,
Consul Template listens for the `SIGHUP` syscall to trigger a configuration
reload. If you update configuration or templates, simply send `HUP` to the
running Consul Template process and Consul Template will reload all the
configurations and templates from disk.

## Debugging

Consul Template can print verbose debugging output. To set the log level for
Consul Template, use the `-log-level` flag:

```shell
$ consul-template -log-level info ...
```

```text
<timestamp> [INFO] (cli) received redis from Watcher
<timestamp> [INFO] (cli) invoking Runner
# ...
```

You can also specify the level as debug:

```shell
$ consul-template -log-level debug ...
```

```text
<timestamp> [DEBUG] (cli) creating Runner
<timestamp> [DEBUG] (cli) creating Consul API client
<timestamp> [DEBUG] (cli) creating Watcher
<timestamp> [DEBUG] (cli) looping for data
<timestamp> [DEBUG] (watcher) starting watch
<timestamp> [DEBUG] (watcher) all pollers have started, waiting for finish
<timestamp> [DEBUG] (redis) starting poll
<timestamp> [DEBUG] (service redis) querying Consul with &{...}
<timestamp> [DEBUG] (service redis) Consul returned 2 services
<timestamp> [DEBUG] (redis) writing data to channel
<timestamp> [DEBUG] (redis) starting poll
<timestamp> [INFO] (cli) received redis from Watcher
<timestamp> [INFO] (cli) invoking Runner
<timestamp> [DEBUG] (service redis) querying Consul with &{...}
# ...
```


## FAQ

**Q: How is this different than confd?**<br>
A: The answer is simple: Service Discovery as a first class citizen. You are also encouraged to read [this Pull Request](https://github.com/kelseyhightower/confd/pull/102) on the project for more background information. We think confd is a great project, but Consul Template fills a missing gap. Additionally, Consul Template has first class integration with [Vault](https://vaultproject.io), making it easy to incorporate secret material like database credentials or API tokens into configuration files.

**Q: How is this different than Puppet/Chef/Ansible/Salt?**<br>
A: Configuration management tools are designed to be used in unison with Consul Template. Instead of rendering a stale configuration file, use your configuration management software to render a dynamic template that will be populated by [Consul][].


## Contributing

To build and install Consul Template locally, you will need to install the
Docker engine:

- [Docker for Mac](https://docs.docker.com/engine/installation/mac/)
- [Docker for Windows](https://docs.docker.com/engine/installation/windows/)
- [Docker for Linux](https://docs.docker.com/engine/installation/linux/ubuntulinux/)

Clone the repository:

```shell
$ git clone https://github.com/hashicorp/consul-template.git
```

To compile the `consul-template` binary for your local machine:

```shell
$ make dev
```

This will compile the `consul-template` binary into `bin/consul-template` as
well as your `$GOPATH` and run the test suite.

If you want to compile a specific binary, set `XC_OS` and `XC_ARCH` or run the
following to generate all binaries:

```shell
$ make build
```

If you want to run the tests, first [install consul locally](https://www.consul.io/docs/install/index.html), then:

```shell
$ make test
```

Or to run a specific test in the suite:

```shell
go test ./... -run SomeTestFunction_name
```

[consul]: https://www.consul.io "Consul by HashiCorp"
[examples]: (https://github.com/hashicorp/consul-template/tree/master/examples) "Consul Template Examples"
[hcl]: https://github.com/hashicorp/hcl "HashiCorp Configuration Language (hcl)"
[releases]: https://releases.hashicorp.com/consul-template "Consul Template Releases"
[text-template]: https://golang.org/pkg/text/template/ "Go's text/template package"
[vault]: https://www.vaultproject.io "Vault by HashiCorp"
