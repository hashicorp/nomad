job "client" {
  datacenters = ["dc1"]

  group "client" {
    count = 6

    task "agent" {
      driver = "docker"

      config {
        image = "djenriquez/nomad:v0.6.0"

        # command = "nomad"
        args         = ["agent"]
        network_mode = "host"
        volumes      = ["local/config:/etc/nomad", "/var/run/docker.sock:/var/run/docker.sock", "/tmp:/tmp"]
        privileged   = true
      }

      resources {
        cpu    = 300
        memory = 100

        network {
          mbits = 10
          port  "http"{}
        }
      }

      template {
        data = <<EOF
log_level    = "DEBUG"
data_dir     = "/tmp/nomad-client{{ env "NOMAD_ALLOC_INDEX" }}"
name         = "client-{{ env "NOMAD_ALLOC_INDEX" }}"
enable_debug = true
client {
  enabled = true
  servers = ["127.0.0.1:4647"]
  options {
    "driver.raw_exec.enable" = "1"
  }
}

ports {
  http = {{ env "NOMAD_PORT_http" }}
}
	 EOF

        destination = "local/config/client.hcl"
      }
    }
  }
}
