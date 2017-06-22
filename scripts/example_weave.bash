#!/bin/bash
if [[ "$USER" != "vagrant" ]]; then
    echo "WARNING: This script is intended to be run from Nomad's Vagrant"
    read -rsp $'Press any key to continue anyway...\n' -n1 key
fi

set -e

if [[ ! -a /usr/local/bin/weave ]]; then
    echo "Installing weave..."
    sudo curl -L git.io/weave -o /usr/local/bin/weave
    sudo chmod a+x /usr/local/bin/weave
fi
weave launch || echo "weave running"
eval $(weave env)

if curl -s localhost:8500 > /dev/null; then
    echo "Consul running"
else
    echo "Running Consul dev agent..."
    consul agent -dev > consul.out &
fi

if curl -s localhost:4646 > /dev/null; then
    echo "Nomad running"
else
    echo "Running Nomad dev agent..."
    nomad agent -dev > nomad.out &
fi

sleep 5

echo "Running Redis with Weave in Nomad..."
cat > redis-weave.nomad <<EOF
job "weave-example" {
  datacenters = ["dc1"]
  type = "service"

  group "cache" {
    count = 1

    task "redis" {
      driver = "docker"
      config {
        image = "redis:3.2"
        port_map {
          db = 6379
        }

        # Use Weave overlay network
        network_mode = "weave"
      }

      resources {
        cpu    = 500 # 500 MHz
        memory = 256 # 256MB
        network {
          mbits = 10
          port "db" {}
        }
      }

      # By default services will advertise the weave address
      service {
        name = "redis"
        tags = ["redis", "weave-addr"]
        port = "db"

        # Since checks are done by Consul on the host system, they default to
        # the host IP:Port.
        check {
          name     = "host-alive"
          type     = "tcp"
          interval = "10s"
          timeout  = "2s"
        }

        # Script checks are run from inside the container, so you can use
        # environment vars to get the container ip:port.
        check {
          name     = "container-script"
          type     = "script"
          command  = "/usr/local/bin/redis-cli"
          args     = ["-p", "\${NOMAD_PORT_db}", "QUIT"]
          interval = "10s"
          timeout  = "2s"
        }
      }

      # Setting address_mode = "host" will create a service entry with the
      # host's address.
      service {
        name = "host-redis"
        tags = ["redis", "host-addr"]
        port = "db"
        address_mode = "host"
      }
    }
  }
}
EOF

nomad run redis-weave.nomad
