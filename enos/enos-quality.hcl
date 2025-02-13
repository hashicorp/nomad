# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

quality "nomad_agent_info" {
  description = "A GET call to /v1/agent/members returns the correct number of running servers and they are all alive"
}

quality "nomad_agent_info_self" {
  description = "A GET call to /v1/agent/self against every server returns the same last_log_index as the leader"
}

quality "nomad_nodes_status" {
  description = "A GET call to /v1/nodes returns the correct number of clients and they are all eligible and ready"
}

quality "nomad_node_eligibility" {
  description = "A GET call to /v1/node/:node-id returns the same node.SchedulingEligibility before and after a server upgrade"
}

quality "nomad_node_metadata" {
  description = "A GET call to /v1/node/:node-id returns the same  node.Meta for each client before and after a client upgrade"
}

quality "nomad_job_status" {
  description = "A GET call to /v1/jobs returns the correct number of jobs and they are all running"
}

quality "nomad_register_job" {
  description = "A POST call to /v1/jobs results in a new job running and allocations being started accordingly"
}

quality "nomad_reschedule_alloc" {
  description = "A POST / PUT call to /v1/allocation/:alloc_id/stop results in the stopped allocation being rescheduled"
}

quality "nomad_restore_snapshot" {
  description = "A node can be restored from a snapshot built on a previous version"
}

quality "nomad_allocs_status" {
  description = "A GET call to /v1/allocs returns the correct number of allocations and they are all running"
}

quality "nomad_alloc_reconect" {
  description = "A GET call to /v1/alloc/:alloc_id will return the same alloc.CreateTime for each allocation before and after a client upgrade"
}

