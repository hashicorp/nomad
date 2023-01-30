export default `job "hello-world" {
  # Specifies the datacenters within which this job should be run.
  # Leave as "dc1" for the default datacenter.
  datacenters = ["dc1"]

  meta {
    # User-defined key/value pairs that can be used in your jobs.
    # You can also use this meta block within Group and Task levels.
    foo = "bar"
  }

  # A group defines a series of tasks that should be co-located
  # on the same client (host). All tasks within a group will be
  # placed on the same host.
  group "servers" {

    network {
      port "www" {
        to = 8001
      }
    }

    service {
      provider = "nomad"
      port     = "www"
    }

    # Tasks are individual units of work that are run by Nomad.
    task "web" {
      # This particular task starts a simple web server within a Docker container
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "httpd"
        args    = ["-v", "-f", "-p", "\${NOMAD_PORT_www}", "-h", "/local"]
        ports   = ["www"]
      }

      template {
        data        = <<EOF
                        <h1>Hello, Nomad!</h1>
                        <ul>
                          <li>Task: {{env "NOMAD_TASK_NAME"}}</li>
                          <li>Group: {{env "NOMAD_GROUP_NAME"}}</li>
                          <li>Job: {{env "NOMAD_JOB_NAME"}}</li>
                          <li>Metadata value for foo: {{env "NOMAD_META_foo"}}</li>
                          <li>Currently running on port: {{env "NOMAD_PORT_www"}}</li>
                        </ul>
                      EOF
        destination = "local/index.html"
      }

      # Specify the maximum resources required to run the task
      resources {
        cpu    = 50
        memory = 64
      }
    }
  }
}`;
