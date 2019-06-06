# New CLI command

Subcommands should always be preferred over adding more top-level commands.

Code flow for commands is generally:

```
CLI (command/) -> API Client (api/) -> HTTP API (command/agent) -> RPC (nomad/)
```

## Code

1.  [ ] New file in `command/` or in an existing file if a subcommand
2.  [ ] Test new command in `command/` package
3.  [ ] Implement autocomplete
4.  [ ] Implement `-json` (returns raw API response)
5.  [ ] Implement `-verbose` (expands truncated UUIDs, adds other detail)
7.  [ ] Update help text
7.  [ ] Implement and test new HTTP endpoint in `command/agent/<command>_endpoint.go`
8.  [ ] Implement and test new RPC endpoint in `nomad/<command>_endpoint.go`
9.  [ ] Implement and test new Client RPC endpoint in `client/<command>_endpoint.go`
  * For client endpoints only (e.g. Filesystem)
10. [ ] Implement and test new `api/` package Request and Response structs
11. [ ] Implement and test new `api/` package helper methods
12. [ ] Implement and test new `nomad/structs/` package Request and Response structs

## Docs

1. [ ] Changelog
2. [ ] API docs https://www.nomadproject.io/api/index.html
3. [ ] CLI docs https://www.nomadproject.io/docs/commands/index.html
4. [ ] Consider if it needs a guide https://www.nomadproject.io/guides/index.html
