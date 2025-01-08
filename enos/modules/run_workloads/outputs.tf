# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

output "job_names" {
  value = [for j in nomad_job.workloads : j.name]
}