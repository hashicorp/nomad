# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "subnets" {
  type        = list(string)
  description = "Subnet IDs task will run in."
}

variable "security_groups" {
  type        = list(string)
  description = "Security Group IDs task will run in."
}

job "nomad-ecs-e2e" {
  datacenters = ["dc1"]

  group "ecs-remote-task-e2e" {
    restart {
      attempts = 0
      mode     = "fail"
    }

    reschedule {
      delay = "5s"
    }

    task "http-server" {
      driver       = "ecs"
      kill_timeout = "1m" // increased from default to accomodate ECS.

      config {
        task {
          launch_type     = "FARGATE"
          task_definition = "nomad-rtd-e2e"
          network_configuration {
            aws_vpc_configuration {
              assign_public_ip = "ENABLED"
              security_groups  = var.security_groups
              subnets          = var.subnets
            }
          }
        }
      }
    }
  }
}
