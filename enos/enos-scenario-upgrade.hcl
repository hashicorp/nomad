# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

scenario "upgrade" {
  description = <<-EOF
    The upgrade scenario verifies in-place upgrades between previously released versions of Nomad
    against another candidate build.
    EOF

  matrix {
    arch = ["amd64"]
    //edition = ["ce", "ent"]
    //os      = ["linux", "windows"]
    edition = ["ent"]
    os      = ["linux"]

    exclude {
      os   = ["windows"]
      arch = ["arm64"]
    }
  }

  providers = [
    provider.aws.default,
  ]

  locals {
    cluster_name           = "${var.prefix}-${matrix.os}-${matrix.arch}-${matrix.edition}-${var.product_version}"
    linux_count            = matrix.os == "linux" ? "4" : "0"
    windows_count          = matrix.os == "windows" ? "4" : "0"
    arch                   = matrix.arch
    clients_count          = local.linux_count + local.windows_count
    test_product_version   = matrix.edition == "ent" ? "${var.product_version}+ent" : "${var.product_version}"
    test_upgrade_version   = matrix.edition == "ent" ? "${var.upgrade_version}+ent" : "${var.upgrade_version}"
    server_os              = "linux"
    download_binaries_path = "${var.download_binary_path}/${matrix.arch}-${matrix.edition}-${var.product_version}"
  }

  step "copy_initial_binary" {
    description = <<-EOF
    Determine which Nomad artifact we want to use for the scenario, depending on the
    'arch', 'edition' and 'os' and bring it from the artifactory to the local instance
    running enos.
    EOF

    module = module.fetch_binaries

    variables {
      artifactory_username   = var.artifactory_username
      artifactory_token      = var.artifactory_token
      artifactory_repo       = var.artifactory_repo_start
      arch                   = local.arch
      edition                = matrix.edition
      product_version        = var.product_version
      oss                    = [local.server_os, matrix.os]
      download_binaries_path = local.download_binaries_path
    }
  }

  step "provision_cluster" {
    depends_on = [step.copy_initial_binary]

    description = <<-EOF
    Using the binary from the previous step, provision a Nomad cluster using the e2e
    module.
    EOF

    module = module.provision_cluster
    variables {
      name                      = local.cluster_name
      nomad_local_binary        = step.copy_initial_binary.binary_path[matrix.os]
      nomad_local_binary_server = step.copy_initial_binary.binary_path[local.server_os]
      server_count              = var.server_count
      client_count_linux        = local.linux_count
      client_count_windows_2022 = local.windows_count
      nomad_license             = var.nomad_license
      consul_license            = var.consul_license
      volumes                   = false
      region                    = var.aws_region
      availability_zone         = var.availability_zone
      instance_arch             = matrix.arch
    }
  }

  step "initial_test_cluster_health" {
    depends_on = [step.provision_cluster]

    description = <<-EOF
    Verify the health of the cluster by checking the status of all servers, nodes,
    jobs and allocs and stopping random allocs to check for correct reschedules"
    EOF

    module = module.test_cluster_health
    variables {
      # connecting to the Nomad API
      nomad_addr  = step.provision_cluster.nomad_addr
      ca_file     = step.provision_cluster.ca_file
      cert_file   = step.provision_cluster.cert_file
      key_file    = step.provision_cluster.key_file
      nomad_token = step.provision_cluster.nomad_token

      # configuring assertions
      server_count    = var.server_count
      client_count    = local.clients_count
      jobs_count      = 0
      alloc_count     = 0
      servers         = step.provision_cluster.servers
      clients_version = local.test_product_version
      servers_version = local.test_product_version
    }

    verifies = [
      quality.nomad_agent_info,
      quality.nomad_agent_info_self,
      quality.nomad_nodes_status,
      quality.nomad_job_status,
      quality.nomad_allocs_status,
      quality.nomad_reschedule_alloc,
    ]
  }

  step "get_vault_env" {

    description = <<-EOF
    Get the HCP vault address and token
    EOF

    module = module.get_vault_env
  }

  step "run_initial_workloads" {
    depends_on = [step.initial_test_cluster_health]

    description = <<-EOF
    Verify the health of the cluster by running new workloads
    EOF

    module = module.run_workloads
    variables {
      nomad_addr        = step.provision_cluster.nomad_addr
      ca_file           = step.provision_cluster.ca_file
      cert_file         = step.provision_cluster.cert_file
      key_file          = step.provision_cluster.key_file
      nomad_token       = step.provision_cluster.nomad_token
      availability_zone = var.availability_zone
      consul_addr       = step.provision_cluster.consul_addr
      consul_token      = step.provision_cluster.consul_token
      vault_token       = step.get_vault_env.vault_token
      vault_addr        = step.get_vault_env.vault_addr
      // The provision_cluster module enables a kv v2 secrets engine using the cluster name as path.
      vault_mount_path = step.provision_cluster.cluster_unique_identifier

      workloads = {
        service_raw_exec = { job_spec = "jobs/raw-exec-service.nomad.hcl", alloc_count = 3, type = "service" }
        service_docker   = { job_spec = "jobs/docker-service.nomad.hcl", alloc_count = 3, type = "service" }
        system_docker    = { job_spec = "jobs/docker-system.nomad.hcl", alloc_count = 0, type = "system" }
        batch_docker     = { job_spec = "jobs/docker-batch.nomad.hcl", alloc_count = 3, type = "batch" }
        batch_raw_exec   = { job_spec = "jobs/raw-exec-batch.nomad.hcl", alloc_count = 3, type = "batch" }
        system_raw_exec  = { job_spec = "jobs/raw-exec-system.nomad.hcl", alloc_count = 0, type = "system" }

        # TODO(tgross): temporarily disabled until we can get CSI plugins to
        # come up reliably

        # nfs = {
        #   job_spec    = "jobs/nfs.nomad.hcl"
        #   alloc_count = 1
        #   type        = "service"
        # }

        # csi_plugin_nfs_controllers = {
        #   job_spec    = "jobs/plugin-nfs-controllers.nomad.hcl"
        #   alloc_count = 1
        #   type        = "service"
        # }

        # csi_plugin_nfs_nodes = {
        #   job_spec    = "jobs/plugin-nfs-nodes.nomad.hcl"
        #   alloc_count = 0
        #   type        = "system"
        # }

        # wants_csi = {
        #   job_spec    = "jobs/wants-volume.nomad.hcl"
        #   alloc_count = 1
        #   type        = "service"
        #   pre_script  = "scripts/wait_for_nfs_volume.sh"
        # }

        tproxy = {
          job_spec    = "jobs/tproxy.nomad.hcl"
          alloc_count = 2
          type        = "service"
          pre_script  = "scripts/create-consul-intention.sh"
        }

        writes_variable = {
          job_spec    = "jobs/writes-vars.nomad.hcl"
          alloc_count = 1
          type        = "service"
          pre_script  = "scripts/configure-variables-acls.sh"
        }

        gets_secret = {
          job_spec    = "jobs/vault-secrets.nomad.hcl",
          alloc_count = 3,
          type        = "service",
          pre_script  = "scripts/populates_secret.sh"
        }
      }
    }

    verifies = [
      quality.nomad_register_job,
    ]
  }

  step "workloads_test_cluster_health" {
    depends_on = [step.run_initial_workloads]

    description = <<-EOF
    Verify the health of the cluster by checking the status of all servers, nodes,
    jobs and allocs and stopping random allocs to check for correct reschedules"
    EOF

    module = module.test_cluster_health
    variables {
      # connecting to the Nomad API
      nomad_addr  = step.provision_cluster.nomad_addr
      ca_file     = step.provision_cluster.ca_file
      cert_file   = step.provision_cluster.cert_file
      key_file    = step.provision_cluster.key_file
      nomad_token = step.provision_cluster.nomad_token

      # configuring assertions
      server_count    = var.server_count
      client_count    = local.clients_count
      jobs_count      = step.run_initial_workloads.jobs_count
      alloc_count     = step.run_initial_workloads.allocs_count
      servers         = step.provision_cluster.servers
      clients_version = local.test_product_version
      servers_version = local.test_product_version
    }

    verifies = [
      quality.nomad_agent_info,
      quality.nomad_agent_info_self,
      quality.nomad_nodes_status,
      quality.nomad_job_status,
      quality.nomad_allocs_status,
      quality.nomad_reschedule_alloc,
    ]
  }

  step "fetch_upgrade_binary" {
    depends_on = [step.provision_cluster, step.workloads_test_cluster_health]

    description = <<-EOF
    Determine which Nomad artifact we want to use for the scenario, depending on the
    'arch', 'edition' and 'os' and fetches the URL and SHA to identify the upgraded
    binary.
    EOF

    module = module.fetch_binaries

    variables {
      artifactory_username = var.artifactory_username
      artifactory_token    = var.artifactory_token
      artifactory_repo     = var.artifactory_repo_upgrade
      arch                 = local.arch
      edition              = matrix.edition
      product_version      = var.upgrade_version
      oss                  = [local.server_os, matrix.os]
      download_binaries    = false
    }
  }

  step "upgrade_servers" {
    depends_on = [step.fetch_upgrade_binary]

    description = <<-EOF
    Takes the servers one by one, makes a snapshot, updates the binary with the
    new one previously fetched and restarts the servers.

    Important: The path where the binary will be placed is hardcoded to match
    what the provision-cluster module does. It can be configurable in the future
    but for now it is:

     * "C:/opt/nomad.exe" for windows
     * "/usr/local/bin/nomad" for linux

    To ensure the servers are upgraded one by one, they use the depends_on meta,
    there are ONLY 3 SERVERS being upgraded in the module.
   EOF
    module      = module.upgrade_servers

    verifies = [
      quality.nomad_agent_info,
      quality.nomad_agent_info_self,
      quality.nomad_restore_snapshot
    ]

    variables {
      # connecting to the Nomad API
      nomad_addr  = step.provision_cluster.nomad_addr
      ca_file     = step.provision_cluster.ca_file
      cert_file   = step.provision_cluster.cert_file
      key_file    = step.provision_cluster.key_file
      nomad_token = step.provision_cluster.nomad_token

      # driving the upgrade
      servers              = step.provision_cluster.servers
      ssh_key_path         = step.provision_cluster.ssh_key_file
      artifactory_username = var.artifactory_username
      artifactory_token    = var.artifactory_token
      artifact_url         = step.fetch_upgrade_binary.artifact_url[local.server_os]
      artifact_sha         = step.fetch_upgrade_binary.artifact_sha[local.server_os]
    }
  }

  step "server_upgrade_test_cluster_health" {
    depends_on = [step.upgrade_servers]

    description = <<-EOF
    Verify the health of the cluster by checking the status of all servers, nodes,
    jobs and allocs and stopping random allocs to check for correct reschedules"
    EOF

    module = module.test_cluster_health
    variables {
      # connecting to the Nomad API
      nomad_addr  = step.provision_cluster.nomad_addr
      ca_file     = step.provision_cluster.ca_file
      cert_file   = step.provision_cluster.cert_file
      key_file    = step.provision_cluster.key_file
      nomad_token = step.provision_cluster.nomad_token

      # configuring assertions
      server_count    = var.server_count
      client_count    = local.clients_count
      jobs_count      = step.run_initial_workloads.jobs_count
      alloc_count     = step.run_initial_workloads.allocs_count
      servers         = step.provision_cluster.servers
      clients_version = local.test_product_version
      servers_version = local.test_upgrade_version
    }

    verifies = [
      quality.nomad_agent_info,
      quality.nomad_agent_info_self,
      quality.nomad_nodes_status,
      quality.nomad_job_status,
      quality.nomad_allocs_status,
      quality.nomad_reschedule_alloc,
    ]
  }

  step "upgrade_first_client" {
    depends_on = [step.server_upgrade_test_cluster_health]

    description = <<-EOF
    Takes a client, writes some dynamic metadata to it,
    updates the binary with the new one previously fetched and restarts it.

    Important: The path where the binary will be placed is hardcoded to match
    what the provision-cluster module does. It can be configurable in the future
    but for now it is:

     * "C:/opt/nomad.exe" for windows
     * "/usr/local/bin/nomad" for linux
    EOF

    module = module.upgrade_client

    verifies = [
      quality.nomad_nodes_status,
      quality.nomad_job_status,
      quality.nomad_node_metadata,
      quality.nomad_alloc_reconnect
    ]

    variables {
      # connecting to the Nomad API
      nomad_addr  = step.provision_cluster.nomad_addr
      ca_file     = step.provision_cluster.ca_file
      cert_file   = step.provision_cluster.cert_file
      key_file    = step.provision_cluster.key_file
      nomad_token = step.provision_cluster.nomad_token

      # configuring assertions
      client               = step.provision_cluster.clients[0]
      ssh_key_path         = step.provision_cluster.ssh_key_file
      artifactory_username = var.artifactory_username
      artifactory_token    = var.artifactory_token
      artifact_url         = step.fetch_upgrade_binary.artifact_url[matrix.os]
      artifact_sha         = step.fetch_upgrade_binary.artifact_sha[matrix.os]
    }
  }

  step "upgrade_second_client" {
    depends_on = [step.upgrade_first_client]

    description = <<-EOF
    Takes a client, writes some dynamic metadata to it,
    updates the binary with the new one previously fetched and restarts it.

    Important: The path where the binary will be placed is hardcoded to match
    what the provision-cluster module does. It can be configurable in the future
    but for now it is:

     * "C:/opt/nomad.exe" for windows
     * "/usr/local/bin/nomad" for linux
    EOF

    module = module.upgrade_client

    verifies = [
      quality.nomad_nodes_status,
      quality.nomad_job_status,
      quality.nomad_node_metadata,
      quality.nomad_alloc_reconnect
    ]

    variables {
      # connecting to the Nomad API
      nomad_addr  = step.provision_cluster.nomad_addr
      ca_file     = step.provision_cluster.ca_file
      cert_file   = step.provision_cluster.cert_file
      key_file    = step.provision_cluster.key_file
      nomad_token = step.provision_cluster.nomad_token

      # configuring assertions
      client               = step.provision_cluster.clients[1]
      ssh_key_path         = step.provision_cluster.ssh_key_file
      artifactory_username = var.artifactory_username
      artifactory_token    = var.artifactory_token
      artifact_url         = step.fetch_upgrade_binary.artifact_url[matrix.os]
      artifact_sha         = step.fetch_upgrade_binary.artifact_sha[matrix.os]
    }
  }

  step "drain_client" {
    depends_on = [step.upgrade_second_client]

    description = <<-EOF
    Selects one client to drain, waits for all allocs to be rescheduled and
    brings back the node eligibility
    EOF

    module = module.drain_client
    variables {
      # connecting to the Nomad API
      nomad_addr     = step.provision_cluster.nomad_addr
      ca_file        = step.provision_cluster.ca_file
      cert_file      = step.provision_cluster.cert_file
      key_file       = step.provision_cluster.key_file
      nomad_token    = step.provision_cluster.nomad_token
      nodes_to_drain = 1
    }
  }

  step "upgrade_third_client" {
    depends_on = [step.drain_client]

    description = <<-EOF
    Takes a client, writes some dynamic metadata to it,
    updates the binary with the new one previously fetched and restarts it.

    Important: The path where the binary will be placed is hardcoded to match
    what the provision-cluster module does. It can be configurable in the future
    but for now it is:

     * "C:/opt/nomad.exe" for windows
     * "/usr/local/bin/nomad" for linux
    EOF

    module = module.upgrade_client

    verifies = [
      quality.nomad_nodes_status,
      quality.nomad_job_status,
      quality.nomad_node_metadata,
      quality.nomad_alloc_reconnect
    ]

    variables {
      # connecting to the Nomad API
      nomad_addr  = step.provision_cluster.nomad_addr
      ca_file     = step.provision_cluster.ca_file
      cert_file   = step.provision_cluster.cert_file
      key_file    = step.provision_cluster.key_file
      nomad_token = step.provision_cluster.nomad_token

      # configuring assertions
      client               = step.provision_cluster.clients[2]
      ssh_key_path         = step.provision_cluster.ssh_key_file
      artifactory_username = var.artifactory_username
      artifactory_token    = var.artifactory_token
      artifact_url         = step.fetch_upgrade_binary.artifact_url[matrix.os]
      artifact_sha         = step.fetch_upgrade_binary.artifact_sha[matrix.os]
    }
  }

  step "upgrade_fourth_client" {
    depends_on = [step.upgrade_third_client]

    description = <<-EOF
    Takes a client, writes some dynamic metadata to it,
    updates the binary with the new one previously fetched and restarts it.

    Important: The path where the binary will be placed is hardcoded to match
    what the provision-cluster module does. It can be configurable in the future
    but for now it is:

     * "C:/opt/nomad.exe" for windows
     * "/usr/local/bin/nomad" for linux
    EOF

    module = module.upgrade_client

    verifies = [
      quality.nomad_nodes_status,
      quality.nomad_job_status,
      quality.nomad_node_metadata,
      quality.nomad_alloc_reconnect
    ]

    variables {
      # connecting to the Nomad API
      nomad_addr  = step.provision_cluster.nomad_addr
      ca_file     = step.provision_cluster.ca_file
      cert_file   = step.provision_cluster.cert_file
      key_file    = step.provision_cluster.key_file
      nomad_token = step.provision_cluster.nomad_token

      # configuring assertions
      client               = step.provision_cluster.clients[3]
      ssh_key_path         = step.provision_cluster.ssh_key_file
      artifactory_username = var.artifactory_username
      artifactory_token    = var.artifactory_token
      artifact_url         = step.fetch_upgrade_binary.artifact_url[matrix.os]
      artifact_sha         = step.fetch_upgrade_binary.artifact_sha[matrix.os]
    }
  }

  step "client_upgrade_test_cluster_health" {
    depends_on = [step.upgrade_fourth_client]

    description = <<-EOF
    Verify the health of the cluster by checking the status of all servers, nodes,
    jobs and allocs and stopping random allocs to check for correct reschedules"
    EOF

    module = module.test_cluster_health
    variables {
      # connecting to the Nomad API
      nomad_addr  = step.provision_cluster.nomad_addr
      ca_file     = step.provision_cluster.ca_file
      cert_file   = step.provision_cluster.cert_file
      key_file    = step.provision_cluster.key_file
      nomad_token = step.provision_cluster.nomad_token

      # configuring assertions
      server_count    = var.server_count
      client_count    = local.clients_count
      jobs_count      = step.run_initial_workloads.jobs_count
      alloc_count     = step.run_initial_workloads.allocs_count
      servers         = step.provision_cluster.servers
      clients_version = local.test_upgrade_version
      servers_version = local.test_upgrade_version
    }

    verifies = [
      quality.nomad_agent_info,
      quality.nomad_agent_info_self,
      quality.nomad_nodes_status,
      quality.nomad_job_status,
      quality.nomad_allocs_status,
      quality.nomad_reschedule_alloc,
    ]
  }

  output "servers" {
    value = step.provision_cluster.servers
  }

  output "linux_clients" {
    value = step.provision_cluster.linux_clients
  }

  output "windows_clients" {
    value = step.provision_cluster.windows_clients
  }

  output "message" {
    value = step.provision_cluster.message
  }

  output "nomad_addr" {
    value = step.provision_cluster.nomad_addr
  }

  output "ca_file" {
    value = step.provision_cluster.ca_file
  }

  output "cert_file" {
    value = step.provision_cluster.cert_file
  }

  output "key_file" {
    value = step.provision_cluster.key_file
  }

  output "ssh_key_file" {
    value = step.provision_cluster.ssh_key_file
  }

  output "nomad_token" {
    value     = step.provision_cluster.nomad_token
    sensitive = true
  }

  output "binary_path" {
    value = step.copy_initial_binary.binary_path
  }

  output "allocs" {
    value = step.run_initial_workloads.allocs_count
  }

  output "new_allocs" {
    value = step.run_initial_workloads.new_allocs_count
  }

  output "nodes" {
    value = step.run_initial_workloads.nodes
  }
}
