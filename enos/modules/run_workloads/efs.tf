# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This file configures an AWS EFS file system for use by CSI workloads.
#
# TODO(tgross): ideally we'll move this into the
# e2e/terraform/provision-inframodule but there's not currently a good way to
# expose outputs from the other module across steps. So we'll probably need to
# inject a tag into the e2e/terraform/provision-infra module from Enos, with a
# reasonable default for nightly, but that'll require some refactoring.

resource "random_pet" "volume_tag" {
}

data "aws_vpc" "default" {
  default = true
}

data "aws_subnet" "test_az" {
  vpc_id            = data.aws_vpc.default.id
  availability_zone = var.availability_zone
  default_for_az    = true
}

# test volume we'll register for the CSI workload
resource "aws_efs_file_system" "test_volume" {
  tags = {
    VolumeTag = random_pet.volume_tag.id
  }
}


resource "aws_security_group" "nfs" {
  name                   = "${random_pet.volume_tag.id}-nfs"
  vpc_id                 = data.aws_vpc.default.id
  revoke_rules_on_delete = true

  ingress {
    from_port   = 2049
    to_port     = 2049
    protocol    = "tcp"
    cidr_blocks = [data.aws_subnet.test_az.cidr_block]
  }
}


# register a mount point for the test subnet so that the EFS plugin can access
# EFS via the DNS name
resource "aws_efs_mount_target" "test_volume" {
  file_system_id  = aws_efs_file_system.test_volume.id
  subnet_id       = data.aws_subnet.test_az.id
  security_groups = [aws_security_group.nfs.id]
}
