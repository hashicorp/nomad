scenario "upgrade" {
    description = <<-EOF
    The upgrade scenario verifies in-place upgrades between previously released versions of Nomad
    against another candidate build. 
    EOF

    matrix {
        arch                 = ["amd64", "arm64"]
        //service_discovery  = ["consul", "nomad"]
        distro               = ["alpine", "fedora", "rhel","ubuntu"]
        editions             = ["ce", "ent"]
        os                   = ["linux", "darwin", "windows"]
  }

  step "copy_binary" {
    description = <<-EOF
    Determine which Nomad artifact we want to use for the scenario, depending on the
   'arch', 'distro' and 'os'
    EOF
    module      = module.build_artifactory

    variables {
        version          = global.initial_version 
        artifactory_path = globals.artifact_path
        artifact_token   = globals.artifact_token 
        os               = matrix.os
        distro           = matrix.os != "linux" ? var.distro : null
        local_path       = var.binary_local_path
    }
  }
/* 
  step "provision_cluster" {
    description = <<-EOF
    Using the binary from the previous step, provision a Nomad cluster using the e2e
    EOF
    module      = "build_${matrix.os}_${matrix.distro}"

    variables {
        name  
        nomad_local_binary                = step.copy_binary.nomad_local_binary
        nomad_license                     = global.nomad_license_path
        server_count                      = var.server_count
        client_count_ubuntu_jammy_amd64 = matrix.distro != "ubuntu" ? var.client_count_ubuntu_jammy_amd64 : 0
        // ...
    }
  }

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
        os             = step.copy_binary.os
        distro           = step.copy_binary.distro
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
        nomad_upgraded_binary = step.copy_binary.nomad_local_binary
        // ...
    }
  }

  step "run_new_workloads" {
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
        nomad_upgraded_binary             = step.copy_binary.nomad_local_binary
        // ...
    }
  }

  step "run_new_workloads" {
     // ...
  } */
}
