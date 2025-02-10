# There can only be a single job definition per file. This job is named
# "countdash" so it will create a job with the ID and Name "countdash".

# The "job" block is the top-most configuration option in the job
# specification. A job is a declarative specification of tasks that Nomad
# should run. Jobs have a globally unique name, one or many task groups, which
# are themselves collections of one or many tasks.
#
# For more information and examples on the "job" block, refer to
#
#     https://developer.hashicorp.com/nomad/docs/job-specification/job
#
job "countdash" {

  # The "ui" block provides options to modify the presentation of the Job index
  # page in Nomad's Web UI.
  #
  # For more information on the "ui" block, refer to:
  #
  #   https://developer.hashicorp.com/nomad/docs/job-specification/ui
  #
  ui {
    description = "Sample Consul Service Mesh Job"
    link {
      label = "Learn more about Nomad"
      url   = "https://developer.hashicorp.com/nomad"
    }
    link {
      label = "Learn more about Consul"
      url   = "https://developer.hashicorp.com/consul"
    }
  }

  # The "group" block defines tasks that should be co-located on the same Nomad
  # client. Nomad places any task within a group onto the same client.
  #
  # For more information and examples on the "group" block, refer to:
  #
  #     https://developer.hashicorp.com/nomad/docs/job-specification/group
  #
  group "api" {

    # The "network" block for a group creates a network namespace shared
    # by all tasks within the group.
    network {
      # "mode" is the CNI plugin used to configure the network namespace.
      # see the documentation for CNI plugins at:
      #
      #     https://github.com/containernetworking/plugins
      #
      mode = "bridge"

      # The service we define for this group is accessible only via
      # Consul Connect, so we do not define ports in its network.
      # port "http" {
      #   to = "8080"
      # }

      # The "dns" block allows operators to override the DNS configuration
      # inherited by the host client.
      # dns {
      #   servers = ["1.1.1.1"]
      # }
    }
    # The "service" block enables Consul Connect.
    service {
      name = "count-api"

      # The port in the service block is the port the service listens on.
      # The Envoy proxy will automatically route traffic to that port
      # inside the network namespace. If the application binds to localhost
      # on this port, the task needs no additional network configuration.
      port = "9001"

      # The "check" block specifies a health check associated with the service.
      # This can be specified multiple times to define multiple checks for the
      # service. Note that checks run inside the task indicated by the "task"
      # field.
      #
      # check {
      #   name     = "alive"
      #   type     = "tcp"
      #   task     = "api"
      #   interval = "10s"
      #   timeout  = "2s"
      # }

      connect {
        # The "sidecar_service" block configures the Envoy sidecar admission
        # controller. For each task group with a sidecar_service, Nomad  will
        # inject an Envoy task into the task group. A group network will be
        # required and a dynamic port will be registered for remote services
        # to connect to Envoy with the name `connect-proxy-<service>`.
        #
        # By default, Envoy will be run via its official upstream Docker image.
        sidecar_service {}
      }
    }

    # The "task" block creates an individual unit of work, such as a Docker
    # container, web application, or batch processing.
    #
    # For more information and examples on the "task" block, refer to:
    #
    #     https://developer.hashicorp.com/nomad/docs/job-specification/task
    #
    task "web" {
      # The "driver" parameter specifies the task driver that should be used to
      # run the task.
      driver = "docker"

      # The "config" block specifies the driver configuration, which is passed
      # directly to the driver to start the task. The details of configurations
      # are specific to each driver, so please see specific driver
      # documentation for more information.
      config {
        image = "hashicorpdev/counter-api:v3"

        # The "auth_soft_fail" configuration instructs Nomad to try public
        # repositories if the task fails to authenticate when pulling images
        # and the Docker driver has an "auth" configuration block.
        auth_soft_fail = true
      }

      # The "resources" block describes the requirements a task needs to
      # execute. Resource requirements include attributes such as memory, cpu,
      # cores, and devices.
      #
      # For a complete list of supported resources and examples on the
      # "resources" block, refer to:
      #
      #     https://developer.hashicorp.com/nomad/docs/job-specification/resources
      #
      resources {
        cpu    = 500 # 500 MHz
        memory = 256 # 256MB
      }
    }

    # The Envoy sidecar admission controller will inject an Envoy task into
    # any task group for each service with a sidecar_service block it contains.
    # A group network will be required and a dynamic port will be registered for
    # remote services to connect to Envoy with the name `connect-proxy-<service>`.
    # By default, Envoy will be run via its official upstream Docker image.
    #
    # There are two ways to modify the default behavior:
    #   * Tasks can define a `sidecar_task` block in the `connect` block
    #     that merges into the default sidecar configuration.
    #   * Add the `kind = "connect-proxy:<service>"` field to another task.
    #     That task will be replace the default Envoy proxy task entirely.
    #
    # task "connect-<service>" {
    #   kind   = "connect-proxy:<service>"
    #   driver = "docker"

    #   config {
    #     image = "${meta.connect.sidecar_image}"
    #     args  = [
    #      "-c", "${NOMAD_TASK_DIR}/bootstrap.json",
    #      "-l", "${meta.connect.log_level}"
    #     ]
    #   }

    #   resources {
    #     cpu    = 100
    #     memory = 300
    #   }

    #   logs {
    #     max_files     = 2
    #     max_file_size = 2
    #   }
    # }
  }

  # This job has a second "group" block to define tasks that might be placed
  # on a separate Nomad client from the group above.
  group "dashboard" {

    network {
      mode = "bridge"

      # The `static = 9002` parameter requests the Nomad scheduler reserve
      # port 9002 on a host network interface. The `to = 9002` parameter
      # forwards that host port to port 9002 inside the network namespace.
      port "http" {
        static = 9002
        to     = 9002
      }
    }

    service {
      name = "count-dashboard"
      port = "9002"

      connect {
        sidecar_service {
          proxy {
            # The upstreams block defines the remote service to access
            # (count-api) and what port to expose that service on inside
            # the network namespace. This allows this task to reach the
            # upstream at localhost:8080.
            upstreams {
              destination_name = "count-api"
              local_bind_port  = 8080
            }
          }
        }

        # The `sidecar_task` block modifies the default configuration
        # of the Envoy proxy task.
        # sidecar_task {
        #   resources {
        #     cpu    = 1000
        #     memory = 512
        #   }
        # }
      }
    }

    task "dashboard" {
      driver = "docker"

      # The application can take advantage of automatically created
      # environment variables to find the address of its upstream
      # service.
      env {
        COUNTING_SERVICE_URL = "http://${NOMAD_UPSTREAM_ADDR_count_api}"
      }

      config {
        image          = "hashicorpdev/counter-dashboard:v3"
        auth_soft_fail = true
      }
    }
  }
}
