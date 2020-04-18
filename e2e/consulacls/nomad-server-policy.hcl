// The Nomad Server requires total access to Consul ACLs, because the Server
// will be requesting new SI tokens from Consul.

acl = "write"

service_prefix "" {
  policy = "write"
}

agent_prefix "" {
  policy = "read"
}

node_prefix "" {
  policy = "read"
}
