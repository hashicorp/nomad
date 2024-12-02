quality "nomad_agent_info" {
    description = "A GET call to /v1/agent/members returns the correct number of
    running servers and they are all alive"
}

quality "nomad_agent_info_self" {
    description = "A GET call to /v1/agent/self against every server returns the
    same last_log_index for all of them."
}

quality "nomad_nodes_status" {
    description = "A GET call to /v1/nodes returns the correct number of clients
    and they are all `eligible` and `ready`".
}

quality "nomad_job_status" {
    description = "A GET call to /v1/jobs returns the correct number of jobs and
    they are all `running`".
}

quality "nomad_register_job" {
    description = "A POST call to /v1/jobs results in a new job running and 
    allocations being started accordingly."
}

quality "nomad_reschedule_alloc" {
   description = "A POST / PUT call to /v1/allocation/:alloc_id/stop results in 
   the stopped allocation being rescheduled"
}


quality "nomad_restore_snapshot" {
    description = "A node can be restored from a snapshot built on a previous
    version"
}

quality "nomad_allocs_status" {
   description = "A GET call to /v1/allocs returns the correct number of 
   allocations and they are all `running`".
}
