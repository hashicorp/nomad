The `dev` package provides helper configuration files for use when developing
Nomad itself.

See the individual packages for more detail on how to use the configuration
files. At a high-level the use case for each package is as follows:

* `hooks`: This package provides helpful git hooks for developing Nomad.

* `docker-clients`: This package provides a Nomad job file that can be used to
  spin up Nomad clients in Docker containers. This provides a simple mechanism
  to create a Nomad cluster locally.

* `tls_cluster`: This package provides Nomad client configs and certificates to
  run a TLS enabled cluster.

* `vault`: This package provides basic Vault configuration files for use in
  configuring a Vault server when testing Nomad and Vault integrations.
