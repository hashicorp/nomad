# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

output "jobs" {
  description = "All the jobs that should be running in the cluster"
  value       = setunion(local.service_jobs, local.system_jobs, local.batch_jobs, local.sysbatch_jobs)
}

output "service_jobs" {
  description = "All the service jobs that should be running in the cluster"
  value       = local.service_jobs
}

output "system_jobs" {
  description = "All the system jobs that should be running in the cluster"
  value       = local.system_jobs
}

output "batch_jobs" {
  description = "All the batch jobs that should be running in the cluster"
  value       = local.batch_jobs
}

output "sysbatch_jobs" {
  description = "All the sysbatch jobs that should be running in the cluster"
  value       = local.sysbatch_jobs
}
