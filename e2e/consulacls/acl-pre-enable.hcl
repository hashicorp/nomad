# This partial consul configuration file will enable Consul ACLs in the default:allow
# mode, which is nessessary for the ACL bootstrapping process of a pre-existing cluster.
#
# The consul-acls-manage.sh script uploads this file as "acl.hcl" to Consul Server
# configuration directories, and restarts those agents.
#
# Later the consul-acls-manage.sh script will replace this configuration with the
# one found in acl-enable.sh so as to enforce ACLs.
acl = {
  enabled                  = true
  default_policy           = "allow"
  enable_token_persistence = true
}
