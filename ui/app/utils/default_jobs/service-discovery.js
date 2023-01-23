export default `job "service-discovery-example" {
  datacenters = ["dc1"]

  group "cache" {
    network {
      port "db" {
        to = 6379
      }
    }
    service {
      // Designates this service as a Nomad native service. Change to "consul" if you're using Consul.
      provider = "nomad"
      name = "redis"
      // Specifies the port to advertise for this service. This must match the port label in the group above.
      port = "db"
      // Adds a health check to the service that can be polled and responded-to elsewhere.
      check {
        name = "up"
        type = "tcp"
        interval = "5s"
        timeout = "1s"
      }
    }

    task "redis" {
      driver = "docker"

      config {
        image          = "redis:7"
        ports          = ["db"]
        auth_soft_fail = true
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
`;
