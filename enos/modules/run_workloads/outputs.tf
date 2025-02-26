# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

output "jobs_count" {
  description = "The number of jobs thar should be running in the cluster"
  value       = length(var.workloads) + tonumber(coalesce(chomp(enos_local_exec.get_jobs.stdout)))
}

output "new_jobs_count" {
  description = "The number of jobs that were triggered by the module"
  value       = length(var.workloads)
}

output "allocs_count" {
  description = "The number of allocs that should be running in the cluster"
  value       = local.system_job_count * tonumber(coalesce(chomp(enos_local_exec.get_nodes.stdout))) + local.service_batch_allocs + tonumber(coalesce(chomp(enos_local_exec.get_allocs.stdout)))
}

output "nodes" {
  description = "Number of current clients in the cluster"
  value       = chomp(enos_local_exec.get_nodes.stdout)
}

output "new_allocs_count" {
  description = "The number of allocs that will be added to the cluster after all the workloads are run"
  value       = local.system_job_count * tonumber(coalesce(chomp(enos_local_exec.get_nodes.stdout), "0")) + local.service_batch_allocs
}
