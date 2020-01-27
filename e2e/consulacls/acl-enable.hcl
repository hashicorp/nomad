# This partial consul configuration file will enable Consul ACLs. The
# consul-acls-manage.sh script uploads this file as "acl.hcl" to Consul Server
# configuration directories, and restarts those agents.
acl = {
  enabled                  = true
  default_policy           = "deny"
  enable_token_persistence = true
}
