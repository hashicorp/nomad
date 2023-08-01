# Configure Consul ACLs

This directory contains a set of scripts for re-configuring Consul in the TF
provisioned e2e environment to enable Consul ACLs.

## Usage

The `consul-acls-manage.sh` script can be used to manipulate the Consul cluster
to activate or de-activate Consul ACLs. There are 3 targets into the script, only
2 of which should be used from e2e framework tests. The script should be run from
the e2e directory (i.e. the directory from wich the e2e framework also runs).

### bootstrap

The command `consul-acls-manage.sh bootstrap` should *NOT* be used from e2e
framework tests. It's merely a convenience entry-point for doing development /
debugging on the script itself.

The bootstrap process will upload "reasonable" ACL policy files to Consul Servers,
Consul Clients, Nomad Servers, and Nomad Clients.

The bootstrap process creates a file on local disk which contains the generated
Consul ACL master token. The file is named based on the current TF state file
serial number. `/tmp/e2e-consul-bootstrap-<serial>.token`

### enable

The command `consul-acls-manage.sh enable` will enable Consul ACLs, going through
the bootstrap process only if necessary. Whether the bootstrap process is necessary
depends on the existence of a token file that matches the current TF state serial
number. If no associated token file exists for the current TF state, the bootstrap
process is required. Otherwise, the bootstrap process is skipped.

If the bootstrap process was not required (i.e. it already occurred and a
Consul master token already exists for the current TF state), the script will
activate ACLs in the Consul Server configurations and restart those agents. After
using `enable`, the `disable` command can be used to turn Consul ACLs back off,
without destroying any of the existing ACL configuration.

### disable

The command `consul-acls-manage.sh disable` will disable Consul ACLs. This does
not "cleanup" the policy files for Consul / Nomad agents, it merely deactivates
ACLs in the Consul Server configurations and restarts those agents. After using
`disable`, the `enable` command can be used to turn Consul ACLs back on, using
the same ACL token(s) generated before.
