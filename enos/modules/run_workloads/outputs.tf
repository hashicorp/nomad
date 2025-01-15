# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

/* output "jobs_count" {
  value = length(local.job_names)
} */

output "jobs_count" {
  description = "The number of jobs thar should be running in the cluster"
  value       = length(var.workloads)
}

output "allocs_count" {
  description = "The number of allocs that should be running in the cluster"
  value       = sum([for wl in var.workloads : wl.alloc_count])
}
