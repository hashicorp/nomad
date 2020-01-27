// Nomad Server needs to set allow_unauthenticated=false to enforce the use
// of a Consul Operator Token on job submission for Connect enabled jobs.
//
// The provided consul.token value must be blessed with acl=write ACLs.
consul {
  allow_unauthenticated = false
  token                 = "CONSUL_TOKEN"
}
