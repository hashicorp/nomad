resource "gpu" {
  range {
    lower = 1
    upper = 4
  }
}

resource "ip" {
  ip_range {
    lower = "192.168.1.0"
    upper = "192.168.1.255"
  }
}

resource "github_token" {
  set {
    items = ["1234", "abcd"]
  }
}

resource "network" {
  enum {
    items {
      private = "eth2"
      overlay = "overlay1"
    }
  }
}
