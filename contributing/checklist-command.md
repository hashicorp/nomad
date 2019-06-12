# New CLI command

Subcommands should always be preferred over adding more top-level commands.

Code flow for commands is generally:

```
CLI (command/) -> API Client (api/) -> HTTP API (command/agent) -> RPC (nomad/)
```

## Code

*  [ ] New file in `command/` or in an existing file if a subcommand
*  [ ] Test new command in `command/` package
*  [ ] Implement autocomplete
*  [ ] Implement `-json` (returns raw API response)
*  [ ] Implement `-verbose` (expands truncated UUIDs, adds other detail)
*  [ ] Update help text
*  [ ] Implement and test new HTTP endpoint in `command/agent/<command>_endpoint.go`
*  [ ] Implement and test new RPC endpoint in `nomad/<command>_endpoint.go`
*  [ ] Implement and test new Client RPC endpoint in
    `client/<command>_endpoint.go` (For client endpoints like Filesystem only)
* [ ] Implement and test new `api/` package Request and Response structs
* [ ] Implement and test new `api/` package helper methods
* [ ] Implement and test new `nomad/structs/` package Request and Response structs

## Docs

* [ ] Changelog
* [ ] API docs https://www.nomadproject.io/api/index.html
* [ ] CLI docs https://www.nomadproject.io/docs/commands/index.html
* [ ] Consider if it needs a guide https://www.nomadproject.io/guides/index.html
