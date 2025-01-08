# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

output "job_names" {
  value = [for j in nomad_job.workloads : j.name]
}

output "allocs_count" {
  description = "The sum of all 'value' fields in the map."
  value       = sum([for wl in var.workloads : wl.alloc_count])
}