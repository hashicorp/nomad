# ACL policy for operator
namespace "*" {
  policy = "write"
  capabilities = [
    "alloc-node-exec",
    "csi-register-plugin",
  ]

  variables {
    path "*" {
      capabilities = ["write", "read", "destroy", "list"]
    }
  }

}

agent {
  policy = "write"
}

operator {
  policy = "write"
}

quota {
  policy = "write"
}

node {
  policy = "write"
}

host_volume "*" {
  policy = "write"
}

plugin {
  policy = "read"
}
