# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

scenario "upgrade" {
  description = <<-EOF
    The upgrade scenario verifies in-place upgrades between previously released versions of Nomad
    against another candidate build. 
    EOF

  matrix {
    arch = ["amd64", "arm64"]
    //service_discovery  = ["consul", "nomad"]
    edition = ["ce", "ent"]
    os      = ["linux", "windows"]

    exclude {
      os   = ["windows"]
      arch = ["arm64"]
    }
  }

  locals {
    cluster_name  = "upgrade-testing-cluster-${matrix.os}-${matrix.arch}"
    ubuntu_count  = matrix.os == "linux" ? 4 : 0
    windows_count = matrix.os == "windows" ? 4 : 0
  }

  step "copy_initial_binary" {
    description = <<-EOF
    Determine which Nomad artifact we want to use for the scenario, depending on the
   'arch', 'edition' and 'os'
    EOF
    module      = module.build_artifactory

    variables {
      artifactory_username = var.artifactory_username
      artifactory_token    = var.artifactory_token
      arch                 = matrix.arch
      edition              = matrix.edition
      product_version      = var.product_version
      os                   = matrix.os
      local_path           = var.binary_local_path
    }
  }

  step "provision_cluster" {
    description = <<-EOF
    Using the binary from the previous step, provision a Nomad cluster using the e2e
    EOF

    depends_on = [step.copy_initial_binary]

    module = module.provision_cluster
    variables {
      name                            = local.cluster_name
      nomad_local_binary              = step.copy_initial_binary.nomad_local_binary
      client_count_ubuntu_jammy_amd64 = local.ubuntu_count
      client_count_windows_2016_amd64 = local.windows_count
      nomad_license                   = var.nomad_license
      consul_license                  = var.consul_license
      volumes = false
    }
  }
  /* 
  step "run_new_workloads" {
    description = <<-EOF
    Verify the health of the cluster by running new workloads and stopping random allocs
    EOF
    
    module      = module.run_workloads

    verifies = [
        quality.nomad_register_job,
        quality.nomad_stop_alloc
    ]
  }

  step "copy_upgraded_binary" {
    description = <<-EOF
    Copy the binary of the newer version ...
    EOF

    module      = "copy_${matrix.artifact_source}"

    variables {
        version          = global.upgrade_version 
        artifactory_path = globals.artifact_path
        artifact_token   = globals.artifact_toke. 
        os             = step.copy_initial_binary.os
        distro           = step.copy_initial_binary.distro
        // ...
    }
  }

  step "upgrade_servers" {
    description = <<-EOF
    Upgrade the cluster's servers by invoking nomad-cc ...
   EOF

    module      = module.run_cc_nomad

    verifies = [
        quality.nomad_agent_info,
        quality.nomad_agent_info_self,
        nomad_restore_snapshot
    ]

    variables {
        cc_update_type        = "server"  
        nomad_upgraded_binary = step.copy_initial_binary.nomad_local_binary
        // ...
    }
  }

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
  } */
}
