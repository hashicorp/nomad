export default `job "hello-world" {
  # Specifies the datacenters within which this job should be run.
  # Leave as "dc1" for the default datacenter.
  datacenters = ["dc1"]

  # A group defines a series of tasks that should be co-located
  # on the same client (host). All tasks within a group will be
  # placed on the same host.
  group "cache" {

    # This requests a static port named "db" on 6379 of the host.
    # This will be used to connect to the redis service.
    network {
      port "db" {
        to = 6379
      }
    }

    # Tasks are individual units of work that are run by Nomad.
    task "redis" {
      # This particular task starts a redis server within a Docker container
      driver = "docker"

      config {
        image          = "redis:7"
        ports          = ["db"]
        auth_soft_fail = true
      }

      # Specify the maximum resources required to run the task
      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
`;
