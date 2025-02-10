Demo TLS Configuration
======================

**Do _NOT_ use in production. For testing purposes only.**

See [Securing Nomad](https://developer.hashicorp.com/nomad/guides/securing-nomad.html)
for a full guide.

This directory contains sample TLS certificates and configuration to ease
testing of TLS related features. There is a makefile to generate certificates,
and pre-generated are available for use.

## Files

| Generated? | File | Description |
| - | ------------- | ---|
| ◻️ | `GNUmakefile` | Makefile to generate certificates |
| ◻️ | `tls-*.hcl`   | Nomad TLS configurations |
| ◻️ | `cfssl*.json` | cfssl configuration files |
| ◻️ | `csr*.json`   | cfssl certificate generation configurations |
| ☑️ | `ca*.pem`     | Certificate Authority certificate and key |
| ☑️ | `client*.pem` | Nomad client node certificate and key |
| ☑️ | `dev*.pem`    | Nomad certificate and key for dev agents |
| ☑️ | `server*.pem` | Nomad server certificate and key |
| ☑️ | `user*.pem`   | Nomad user (CLI) certificate and key |
| ☑️ | `user.pfx`    | Nomad browser PKCS #12 certificate and key *(blank password)* |

## Usage

### Agent

To run a TLS-enabled Nomad agent include the `tls.hcl` configuration file with
either the `-dev` flag or your own configuration file. If you're not running
the `nomad agent` command from *this* directory you will have to edit the paths
in `tls.hcl`.

```sh
# Run the dev agent with TLS enabled
nomad agent -dev -config=tls-dev.hcl

# Run a *server* agent with your configuration and TLS enabled
nomad agent -config=path/to/custom.hcl -config=tls-server.hcl

# Run a *client* agent with your configuration and TLS enabled
nomad agent -config=path/to/custom.hcl -config=tls-client.hcl
```

### Browser

To access the Nomad Web UI when TLS is enabled you will need to import two
certificate files into your browser:

- `ca.pem` must be imported as a Certificate Authority
- `user.pfx` must be imported as a Client certificate. The password is blank.

When you access the UI via https://localhost:4646/ you will be prompted to
select the user certificate you imported.
