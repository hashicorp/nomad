This package provides Vault configuration files that can be used to quickly
configure a Vault server when testing Nomad and Vault integrations.

To configure a Vault server run the following:

In one shell run the Vault server:

```shell
vault server -dev
```

In another run the following to configure the Vault server and create a token
for the Nomad servers (must be in nomad/dev/vault):

```shell
export VAULT_ADDR='http://127.0.0.1:8200'
vault policy-write nomad-server nomad-server-policy.hcl
vault write /auth/token/roles/nomad-cluster @nomad-cluster-role.json
vault token-create -policy nomad-server -period 72h -orphan
```

You can then run Nomad using the generated token. An example would be:

```
nomad agent -dev -vault-enabled -vault-address=http://127.0.0.1:8200 \
    -vault-create-from-role=nomad-cluster -vault-token=<token>
```
