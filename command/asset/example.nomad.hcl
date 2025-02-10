# There can only be a single job definition per file. This job is named
# "example" so it will create a job with the ID and Name "example".

# The "job" block is the top-most configuration option in the job
# specification. A job is a declarative specification of tasks that Nomad
# should run. Jobs have a globally unique name, one or many task groups, which
# are themselves collections of one or many tasks.
#
# For more information and examples on the "job" block, refer to:
#
#     https://developer.hashicorp.com/nomad/docs/job-specification/job
#
job "example" {
  # The "region" parameter specifies the region in which to execute the job.
  # If omitted, this inherits the default region name of "global".
  # region = "global"
  #
  # The "datacenters" parameter specifies the list of datacenters which should
  # be considered when placing this task. This accepts wildcards and defaults
  # allowing placement on all datacenters.
  datacenters = ["*"]

  # The "type" parameter controls the type of job, which impacts the scheduler's
  # decision on placement. This configuration is optional and defaults to
  # "service".
  #
  # For a full list of job types and their differences, refer to:
  #
  #     https://developer.hashicorp.com/nomad/docs/schedulers
  #
  type = "service"

  # The "constraint" block defines additional constraints for placing this job,
  # in addition to any resource or driver constraints. You may place this block
  # at the "job", "group", or "task" level, and it supports variable interpolation.
  #
  # For more information and examples on the "constraint" block, refer to:
  #
  #     https://developer.hashicorp.com/nomad/docs/job-specification/constraint
  #
  # constraint {
  #   attribute = "${attr.kernel.name}"
  #   value     = "linux"
  # }

  # The "update" block specifies the update strategy of task groups. The update
  # strategy is used to control things like rolling upgrades, canaries, and
  # blue/green deployments. If omitted, no update strategy is enforced. The
  # "update" block may be placed at the job or task group. When placed at the
  # job, it applies to all groups within the job. When placed at both the job and
  # group level, the blocks are merged with the groups taking precedence.
  #
  # For more information and examples on the "update" block, refer to:
  #
  #     https://developer.hashicorp.com/nomad/docs/job-specification/update
  #
  update {
    # The "max_parallel" parameter specifies the maximum number of updates to
    # perform in parallel. In this case, this specifies to update a single task
    # at a time.
    max_parallel = 1

    # The "min_healthy_time" parameter specifies the minimum time the allocation
    # must be in the healthy state before it is marked as healthy and unblocks
    # further allocations from being updated.
    min_healthy_time = "10s"

    # The "healthy_deadline" parameter specifies the deadline in which the
    # allocation must be marked as healthy after which the allocation is
    # automatically transitioned to unhealthy. Transitioning to unhealthy will
    # fail the deployment and potentially roll back the job if "auto_revert" is
    # set to true.
    healthy_deadline = "3m"

    # The "progress_deadline" parameter specifies the deadline in which an
    # allocation must be marked as healthy. The deadline begins when the first
    # allocation for the deployment is created and is reset whenever an allocation
    # as part of the deployment transitions to a healthy state. If no allocation
    # transitions to the healthy state before the progress deadline, the
    # deployment is marked as failed.
    progress_deadline = "10m"

    # The "auto_revert" parameter specifies if the job should auto-revert to the
    # last stable job on deployment failure. A job is marked as stable if all the
    # allocations as part of its deployment were marked healthy.
    auto_revert = false

    # The "canary" parameter specifies that changes to the job that would result
    # in destructive updates should create the specified number of canaries
    # without stopping any previous allocations. Once the operator determines the
    # canaries are healthy, they can be promoted which unblocks a rolling update
    # of the remaining allocations at a rate of "max_parallel".
    #
    # Further, setting "canary" equal to the count of the task group allows
    # blue/green deployments. When the job is updated, a full set of the new
    # version is deployed and upon promotion the old version is stopped.
    canary = 0
  }

  # The migrate block specifies the group's strategy for migrating off of
  # draining nodes. If omitted, Nomad applies a default migration strategy.
  #
  # For more information on the "migrate" block, refer to:
  #
  #     https://developer.hashicorp.com/nomad/docs/job-specification/migrate
  #
  migrate {
    # Specifies the number of task groups that can be migrated at the same
    # time. This number must be less than the total count for the group as
    # (count - max_parallel) will be left running during migrations.
    max_parallel = 1

    # Specifies the mechanism in which allocations health is determined. The
    # potential values are "checks" or "task_states".
    health_check = "checks"

    # Specifies the minimum time the allocation must be in the healthy state
    # before it is marked as healthy and unblocks further allocations from being
    # migrated. This is specified using a label suffix like "30s" or "15m".
    min_healthy_time = "10s"

    # Specifies the deadline in which the allocation must be marked as healthy
    # after which the allocation is automatically transitioned to unhealthy. This
    # is specified using a label suffix like "2m" or "1h".
    healthy_deadline = "5m"
  }

  # The "ui" block provides options to modify the presentation of the Job index
  # page in Nomad's Web UI.
  #
  # For more information on the "ui" block, refer to:
  #
  #   https://developer.hashicorp.com/nomad/docs/job-specification/ui
  #
  ui {
    description = "Nomad **Example** Job"
    link {
      label = "Learn more about Nomad"
      url   = "https://developer.hashicorp.com/nomad"
    }
  }

  # The "group" block defines tasks that should be co-located on the same Nomad
  # client. Nomad places any task within a group onto the same client.
  #
  # For more information and examples on the "group" block, refer to:
  #
  #     https://developer.hashicorp.com/nomad/docs/job-specification/group
  #
  group "cache" {
    # The "count" parameter specifies the number of this task group. This value
    # must be non-negative and defaults to 1.
    count = 1

    # The "network" block specifies the network configuration for the task
    # group including requesting port bindings.
    #
    # For more information and examples on the "network" block, refer to:
    #
    #     https://developer.hashicorp.com/nomad/docs/job-specification/network
    #
    network {
      port "db" {
        to = 6379
      }
    }

    # The "service" block instructs Nomad to register this task as a service
    # in the service discovery engine, which is currently Nomad or Consul. This
    # will make the service discoverable after Nomad has placed it on a host and
    # port.
    #
    # For more information and examples on the "service" block, refer to:
    #
    #     https://developer.hashicorp.com/nomad/docs/job-specification/service
    #
    service {
      name     = "redis-cache"
      tags     = ["global", "cache"]
      port     = "db"
      provider = "nomad"

      # The "check" block instructs Nomad to create a health check for
      # this service. Uncomment the sample check below to enable it.
      #
      # For more information and examples on the "check" block, refer to:
      #
      #   https://developer.hashicorp.com/nomad/docs/job-specification/check

      # check {
      #   name     = "alive"
      #   type     = "tcp"
      #   interval = "10s"
      #   timeout  = "2s"
      # }

    }

    # The "restart" block configures a group's behavior on task failure. If
    # left unspecified, Nomad uses a default restart policy based on the job type.
    #
    # For more information and examples on the "restart" block, refer to:
    #
    #     https://developer.hashicorp.com/nomad/docs/job-specification/restart
    #
    restart {
      # The number of attempts to run within the specified interval.
      attempts = 2
      interval = "30m"

      # The "delay" parameter specifies the duration to wait before restarting
      # a task after it has failed.
      delay = "15s"

      # The "mode" parameter controls what happens when a task has restarted
      # "attempts" times within the interval. "delay" mode delays the next
      # restart until the next interval. "fail" mode does not restart the task
      # if "attempts" has been hit within the interval.
      mode = "fail"
    }

    # The "ephemeral_disk" block describes the ephemeral disk requirements of
    # the group. All tasks in this group share the same ephemeral disk.
    #
    # For more information and examples on the "ephemeral_disk" block, refer
    # to:
    #
    #     https://developer.hashicorp.com/nomad/docs/job-specification/ephemeral_disk
    #
    ephemeral_disk {
      # When sticky is true and the task group is updated, the scheduler
      # prefers to place the updated allocation on the same node and migrate the
      # data. This is useful for tasks that store data that should persist
      # across updates.
      # sticky = true

      # Setting migrate to true results Nomad copying the allocation directory
      # to the new allocation on update.
      # migrate = true

      # The "size" parameter specifies the size in MB of shared ephemeral disk
      # between tasks in the group.
      size = 300
    }

    # The "affinity" block lets you express placement preferences
    # based on node attributes or metadata.
    #
    # For more information and examples on the "affinity" block, refer to:
    #
    #     https://developer.hashicorp.com/nomad/docs/job-specification/affinity
    #
    # affinity {
    #   attribute = "${node.datacenter}"
    #   value     = "us-west1"
    #   weight    = 100
    # }

    # The "spread" block let you increase the failure tolerance of
    # your applications by specifying a node attribute that allocations
    # should be spread over.
    #
    # For more information and examples on the "spread" block, refer to:
    #
    #     https://developer.hashicorp.com/nomad/docs/job-specification/spread
    #
    # spread {
    #   attribute = "${node.datacenter}"
    #   target "us-east1" {
    #     percent = 60
    #   }
    #   target "us-west1" {
    #     percent = 40
    #   }
    #  }

    # The "task" block creates an individual unit of work, such as a Docker
    # container, virtual machine, or process.
    #
    # For more information and examples on the "task" block, refer to:
    #
    #     https://developer.hashicorp.com/nomad/docs/job-specification/task
    #
    task "redis" {
      # The "driver" parameter specifies the task driver that should be used to
      # run the task.
      driver = "docker"

      # The "config" block specifies the driver configuration, which is passed
      # directly to the driver to start the task. The details of configurations
      # are specific to each driver, so please see specific driver
      # documentation for more information.
      config {
        image = "redis:7"
        ports = ["db"]

        # The "auth_soft_fail" configuration instructs Nomad to try public
        # repositories if the task fails to authenticate when pulling images
        # and the Docker driver has an "auth" configuration block.
        auth_soft_fail = true
      }

      # The "artifact" block instructs Nomad to download an artifact from a
      # remote source prior to starting the task. This provides a convenient
      # mechanism for downloading configuration files or data needed to run the
      # task. Specify the "artifact" block multiple times to download
      # multiple artifacts.
      #
      # For more information and examples on the "artifact" block, refer to:
      #
      #     https://developer.hashicorp.com/nomad/docs/job-specification/artifact
      #
      # artifact {
      #   source = "http://foo.com/artifact.tar.gz"
      #   options {
      #     checksum = "md5:c4aa853ad2215426eb7d70a21922e794"
      #   }
      # }

      # The "logs" block instructs the Nomad client on how many log files and
      # the maximum size of those logs files to retain. Logging is enabled by
      # default, but the "logs" block allows for finer-grained control over
      # the log rotation and storage configuration.
      #
      # For more information and examples on the "logs" block, refer to:
      #
      #     https://developer.hashicorp.com/nomad/docs/job-specification/logs
      #
      # logs {
      #   max_files     = 10
      #   max_file_size = 15
      # }

      # The "identity" block instructs Nomad to expose the task's workload
      # identity token as an environment variable and in the file
      # secrets/nomad_token.
      #
      # For more information and examples on the "identity" block, refer to:
      #
      #     https://developer.hashicorp.com/nomad/docs/job-specification/identity
      #
      identity {
        env  = true
        file = true
      }

      # The "resources" block describes the requirements a task needs to
      # execute. Resource requirements include attributes such as memory, cpu,
      # cores, and devices.
      #
      # For a complete list of attributes and examples on the "resources"
      # block, refer to:
      #
      #     https://developer.hashicorp.com/nomad/docs/job-specification/resources
      #
      resources {
        cpu    = 500 # 500 MHz
        memory = 256 # 256MB
      }

      # The "template" block instructs Nomad to manage a template, such as a
      # configuration file or script. This template can optionally pull data
      # from Nomad, Consul, or Vault to populate runtime configuration data.
      #
      # For more information and examples on the "template" block, refer to:
      #
      #     https://developer.hashicorp.com/nomad/docs/job-specification/template
      #
      # template {
      #   data          = "---\nkey: {{ key \"service/my-key\" }}"
      #   destination   = "local/file.yml"
      #   change_mode   = "signal"
      #   change_signal = "SIGHUP"
      # }

      # The "template" block can also be used to create environment variables
      # for tasks that prefer those to config files. The task will be restarted
      # when data pulled from Consul or Vault changes.
      #
      # template {
      #   data        = "KEY={{ key \"service/my-key\" }}"
      #   destination = "local/file.env"
      #   env         = true
      # }

      # The "vault" block instructs the Nomad client to acquire a token from a
      # HashiCorp Vault server. Nomad must be configured to communicate with
      # Vault. By default, Nomad injects the token into the task via an
      # environment variable and makes the token available to the "template"
      # block. The Nomad client handles the renewal and revocation of the Vault
      # token.
      #
      # For more information and examples on the "vault" block, refer to:
      #
      #     https://developer.hashicorp.com/nomad/docs/job-specification/vault
      #
      # vault {
      #   policies      = ["cdn", "frontend"]
      #   change_mode   = "signal"
      #   change_signal = "SIGHUP"
      # }

      # Controls the timeout between signalling a task it will be killed
      # and killing the task. If not set a default is used.
      # kill_timeout = "20s"
    }
  }
}
