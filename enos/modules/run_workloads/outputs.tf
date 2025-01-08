# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

/* output "raw-exec-service_id" {
    value = nomad_job.raw-exec-service.name
}

output "docker-service_id" {
    value = nomad_job.docker-service.name
}
 */

output "job_names" {
  value = [for j in nomad_job.workloads : j.name]
}