# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

scenario "upgrade" {
  description = <<-EOF
    The upgrade scenario verifies in-place upgrades between previously released versions of Nomad
    against another candidate build. 
    EOF

  matrix {
    arch    = ["amd64"]
    edition = ["ce"]
    os      = ["linux"]
    //service_discovery  = ["consul", "nomad"]
    //arch = ["amd64", "arm64"]
    //edition = ["ce", "ent"]
    //os      = ["linux", "windows"]
    exclude {
      os   = ["windows"]
      arch = ["arm64"]
    }
  }

  providers = [
    provider.aws.default,
  ]

  locals {
    cluster_name  = "mcj-${matrix.os}-${matrix.arch}-${matrix.edition}-${var.product_version}"
    linux_count   = matrix.os == "linux" ? "4" : "0"
    windows_count = matrix.os == "windows" ? "4" : "0"
    arch          = matrix.arch
    clients_count = local.linux_count + local.windows_count
  }

  step "copy_initial_binary" {
    description = <<-EOF
    Determine which Nomad artifact we want to use for the scenario, depending on the
   'arch', 'edition' and 'os' and bring it from the artifactory to a local instance.
    EOF

    module = module.build_artifactory

    variables {
      artifactory_username = var.artifactory_username
      artifactory_token    = var.artifactory_token
      arch                 = local.arch
      edition              = matrix.edition
      product_version      = var.product_version
      os                   = matrix.os
      binary_path          = "${var.nomad_local_binary}/${matrix.os}-${matrix.arch}-${matrix.edition}-${var.product_version}"
    }
  }

  step "provision_cluster" {
    depends_on  = [step.copy_initial_binary]
    description = <<-EOF
    Using the binary from the previous step, provision a Nomad cluster using the e2e
    EOF

    module = module.provision_cluster
    variables {
      name                      = local.cluster_name
      nomad_local_binary        = step.copy_initial_binary.nomad_local_binary
      server_count              = var.server_count
      client_count_linux        = local.linux_count
      client_count_windows_2016 = local.windows_count
      nomad_license             = var.nomad_license
      consul_license            = var.consul_license
      volumes                   = false
      region                    = var.aws_region
      instance_arch             = matrix.arch
    }
  }

  step "run_initial_workloads" {
    depends_on  = [step.provision_cluster]
    description = <<-EOF
    Verify the health of the cluster by running new workloads
    EOF

    module = module.run_workloads
    variables {
      nomad_addr  = step.provision_cluster.nomad_addr
      ca_file     = step.provision_cluster.ca_file
      cert_file   = step.provision_cluster.cert_file
      key_file    = step.provision_cluster.key_file
      nomad_token = step.provision_cluster.nomad_token
    }
    verifies = [
      quality.nomad_register_job,
    ]
  }

  step "initial_test_cluster_health" {
    depends_on  = [step.run_initial_workloads]
    description = <<-EOF
    Verify the health of the cluster by checking the status of all servers, nodes, jobs and allocs and stopping random allocs to check for correct reschedules"
    EOF

    module = module.test_cluster_health
    variables {
      nomad_addr   = step.provision_cluster.nomad_addr
      ca_file      = step.provision_cluster.ca_file
      cert_file    = step.provision_cluster.cert_file
      key_file     = step.provision_cluster.key_file
      nomad_token  = step.provision_cluster.nomad_token
      server_count = var.server_count
      client_count = local.clients_count
      jobs_count   = step.run_initial_workloads.jobs_count
      alloc_count  = step.run_initial_workloads.allocs_count
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

  step "copy_upgrade_binary" {
    depends_on  = [step.provision_cluster]
    description = <<-EOF
    Bring the new upgraded binary from the artifactory
    EOF

    module = module.build_artifactory

    variables {
      artifactory_username = var.artifactory_username
      artifactory_token    = var.artifactory_token
      arch                 = local.arch
      edition              = matrix.edition
      product_version      = var.upgrade_version
      os                   = matrix.os
      binary_path          = "${var.nomad_local_binary}/${matrix.os}-${matrix.arch}-${matrix.edition}-${var.upgrade_version}"
    }
  }

  step "upgrade_servers" {
    depends_on = [step.copy_upgrade_binary, step.provision_cluster]

    description = <<-EOF
    Takes the servers one by one, makes a snapshot, updates the binary with the
    new one previously fetched, restarts the servers from the snapshot.

    Important: The path where the binary will be placed is hardcoded, according 
    to Nomads best practices it will be: 

     * "C:/opt/nomad.exe" for windows 
     * "/usr/local/bin/nomad" for linux
   EOF
    module      = module.upgrade_servers

    verifies = [
      quality.nomad_agent_info,
      quality.nomad_agent_info_self,
      quality.nomad_restore_snapshot
    ]

    variables {
      nomad_addr                 = step.provision_cluster.nomad_addr
      ca_file                    = step.provision_cluster.ca_file
      cert_file                  = step.provision_cluster.cert_file
      key_file                   = step.provision_cluster.key_file
      nomad_token                = step.provision_cluster.nomad_token
      platform                   = matrix.os
      servers                    = step.provision_cluster.servers
      nomad_local_upgrade_binary = step.copy_upgrade_binary.nomad_local_binary
      ssh_key_path               = step.provision_cluster.ssh_key_file
    }
  }

  step "server_upgrade_test_cluster_health" {
    depends_on  = [step.upgrade_servers]
    description = <<-EOF
    Verify the health of the cluster by checking the status of all servers, nodes, 
    jobs and allocs and stopping random allocs to check for correct reschedules
    EOF

    module = module.test_cluster_health
    variables {
      nomad_addr   = step.provision_cluster.nomad_addr
      ca_file      = step.provision_cluster.ca_file
      cert_file    = step.provision_cluster.cert_file
      key_file     = step.provision_cluster.key_file
      nomad_token  = step.provision_cluster.nomad_token
      server_count = var.server_count
      client_count = local.linux_count + local.windows_count
      jobs_count   = step.run_initial_workloads.jobs_count
      alloc_count  = step.run_initial_workloads.allocs_count
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

  /*
  step "run_servers_workloads" {
   // ...
  }



  step "upgrade_client" {
    description = <<-EOF
    Upgrade the cluster's clients by invoking nomad-cc ...
    EOF

    module      = module.run_cc_nomad

    verifies = [
        quality.nomad_nodes_status,
        quality.nomad_job_status
    ]

    variables {
        cc_update_type = "client"  
        nomad_upgraded_binary             = step.copy_initial_binary.nomad_local_binary
        // ...
    }
  }

  step "run_clients_workloads" {
     // ...
  } 

  step "client_upgrade_test_cluster_health" {
    depends_on  = [step.run_initial_workloads]
    description = <<-EOF
    Verify the health of the cluster by checking the status of all servers, nodes, jobs and allocs and stopping random allocs to check for correct reschedules"
    EOF

    module = module.test_cluster_health
    variables {
      nomad_addr   = step.provision_cluster.nomad_addr
      ca_file      = step.provision_cluster.ca_file
      cert_file    = step.provision_cluster.cert_file
      key_file     = step.provision_cluster.key_file
      nomad_token  = step.provision_cluster.nomad_token
      server_count = var.server_count
      client_count = local.linux_count + local.windows_count
      jobs_count   = step.run_initial_workloads.jobs_count
      alloc_count  = step.run_initial_workloads.allocs_count
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
 */
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

  output "nomad_token" {
    value     = step.provision_cluster.nomad_token
    sensitive = true
  }

}
