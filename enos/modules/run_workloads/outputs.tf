# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

output "jobs_count" {
  value = length(local.job_names)
}

output "allocs_count" {
  description = "The sum of all 'value' fields in the map."
  value       = sum([for wl in var.workloads : wl.alloc_count])
}
