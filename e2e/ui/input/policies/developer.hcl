# power-user in dev namespace
namespace "dev" {
  policy = "write"

  secure_variables {
    path "*" {
      capabilities = ["list", "read", "write", "destroy"]
    }

    # operator-owned vars in this namespace
    path "system/*" {
      capabilities = ["list", "read"]
    }
  }
}

# read-only prod access
namespace "prod" {
  capabilities = [
    "list-jobs",
    "read-job",
    "read-logs"
  ]

  # list-only access outside of
  secure_variables {
    path "*" {
      capabilities = ["list"]
    }
  }

}

# narrow read-only access to other namespaces
namespace "*" {
  policy = "read"

  secure_variables {
    path "*" {
      capabilities = ["list"]
    }
  }
}

node {
  policy = "read"
}

agent {
  policy = "read"
}

operator {
  policy = "read"
}

host_volume "*" {
  policy = "write"
}
