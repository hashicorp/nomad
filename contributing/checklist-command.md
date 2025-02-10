# New CLI command

Subcommands should always be preferred over adding more top-level commands.

Code flow for commands is generally:

```
CLI (command/) -> API Client (api/) -> HTTP API (command/agent) -> RPC (nomad/)
```

## Code

* [ ] Consider similar commands in Consul, Vault, and other tools. Is there
  prior art we should match? Arguments, flags, env vars, etc?
* [ ] New file in `command/` or in an existing file if a subcommand
* [ ] For nested commands make sure all intermediary subcommands exist (for
  example, `nomad acl`, `nomad acl policy`, and `nomad acl policy apply` must
  all be valid commands)
* [ ] Test new command in `command/` package
* [ ] Implement autocomplete
* [ ] Implement `-json` (returns raw API response)
* [ ] Implement `-t` (format API response using gotemplate)
* [ ] Implement `-verbose` (expands truncated UUIDs, adds other detail)
* [ ] Update help text
* [ ] Register new command in `command/commands.go`
* [ ] If the command has a `status` subcommand consider adding a search context
  in `nomad/search_endpoint.go` and update `command/status.go`
* [ ] Implement and test new HTTP endpoint in `command/agent/<command>_endpoint.go`
* [ ] Register new URL paths in `command/agent/http.go`
* [ ] Implement and test new RPC endpoint in `nomad/<command>_endpoint.go`
* [ ] Implement and test new Client RPC endpoint in
  `client/<command>_endpoint.go` (For client endpoints like Filesystem only)
* [ ] Implement and test new `api/` package Request and Response structs
* [ ] Implement and test new `api/` package helper methods
* [ ] Implement and test new `nomad/structs/` package Request and Response structs

## Docs

* [ ] Changelog
* [ ] API docs https://developer.hashicorp.com/nomad/api
* [ ] CLI docs https://developer.hashicorp.com/nomad/docs/commands
* [ ] If adding new docs see [website README](../website/README.md#editing-navigation-sidebars)
* [ ] Consider if it needs a guide https://developer.hashicorp.com/nomad/guides/index.html
