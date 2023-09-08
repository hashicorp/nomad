# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Nomad ECS Remote Task Driver E2E
resource "aws_ecs_cluster" "nomad_rtd_e2e" {
  name = "nomad-rtd-e2e"
}

resource "aws_ecs_task_definition" "nomad_rtd_e2e" {
  family                = "nomad-rtd-e2e"
  container_definitions = file("ecs-task.json")

  # Don't need a network for e2e tests
  network_mode = "awsvpc"

  requires_compatibilities = ["FARGATE"]
  cpu                      = 256
  memory                   = 512
}

data "template_file" "ecs_vars_hcl" {
  template = <<EOT
security_groups = ["${aws_security_group.clients.id}"]
subnets         = ["${data.aws_subnet.default.id}"]
EOT
}

resource "local_file" "ecs_vars_hcl" {
  content         = data.template_file.ecs_vars_hcl.rendered
  filename        = "${path.module}/../remotetasks/input/ecs.vars"
  file_permission = "0664"
}
