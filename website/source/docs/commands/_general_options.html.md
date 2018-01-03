- `-address=<addr>`: The address of the Nomad server. Overrides the `NOMAD_ADDR`
  environment variable if set. Defaults to `http://127.0.0.1:4646`.

- `-region=<region>`: The region of the Nomad server to forward commands to.
  Overrides the `NOMAD_REGION` environment variable if set. Defaults to the
  Agent's local region.

- `-no-color`: Disables colored command output.

- `-ca-cert=<path>`: Path to a PEM encoded CA cert file to use to verify the
  Nomad server SSL certificate. Overrides the `NOMAD_CACERT` environment
  variable if set.

- `-ca-path=<path>`: Path to a directory of PEM encoded CA cert files to verify
  the Nomad server SSL certificate. If both `-ca-cert` and `-ca-path` are
  specified, `-ca-cert` is used. Overrides the `NOMAD_CAPATH` environment
  variable if set.

- `-client-cert=<path>`: Path to a PEM encoded client certificate for TLS
  authentication to the Nomad server. Must also specify `-client-key`. Overrides
  the `NOMAD_CLIENT_CERT` environment variable if set.

- `-client-key=<path>`: Path to an unencrypted PEM encoded private key matching
  the client certificate from `-client-cert`. Overrides the `NOMAD_CLIENT_KEY`
  environment variable if set.

- `-tls-skip-verify`: Do not verify TLS certificate. This is highly not
  recommended. Verification will also be skipped if `NOMAD_SKIP_VERIFY` is set.
  
- `-token`: The SecretID of an ACL token to use to authenticate API requests with.
  Overrides the `NOMAD_TOKEN` environment variable if set.
