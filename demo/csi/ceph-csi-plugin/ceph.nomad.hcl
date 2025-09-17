job "ceph" {

  group "ceph" {

    network {
      mode = "bridge"

      port "ceph_mon" {
        to = 3300 # Monitor (msgr2 protocol - preferred)
        #static = 3300 # will plugin accept a port?
      }
      port "ceph_mon_legacy" {
        to = 6789 # Monitor (msgr1 protocol - legacy)
      }
      port "ceph_radosgw" {
        to = 8000 # RadosGW (HTTP)
      }
      port "ceph_dashboard" {
        to = 8443 # Dashboard (HTTPS)
      }

      # port "ceph_osd" {
      #   # 6800-7300: OSDs, MGRs, MDSs
      #   to = 6800 # TODO: how do we handle a range to 7300!?
      # }
    }

    service {
      name     = "ceph-mon"
      port     = "ceph_mon"
      provider = "nomad"
    }

    service {
      name     = "ceph-radosgw"
      port     = "ceph_radosgw"
      provider = "nomad"
    }

    service {
      name     = "ceph-dashboard"
      port     = "ceph_dashboard"
      provider = "nomad"
    }

    task "ceph" {
      driver = "docker"

      config {
        image = "quay.io/benjamin_holmes/ceph-aio:v19"
        ports = ["ceph_mon", "ceph_dashboard", "ceph_radosgw"]

        # ref https://github.com/hashicorp/nomad/issues/26852
        security_opt = ["label=disable"]
      }

      env {
        MON_IP               = "0.0.0.0" # Will use host's IP
        OSD_COUNT            = "1"
        OSD_SIZE             = "10G"
        CEPH_PUBLIC_NETWORK  = "0.0.0.0/0"
        CEPH_CLUSTER_NETWORK = "0.0.0.0/0"

        # TODO: this is a bogus demo credential; make a secret block
        DASHBOARD_USER = "admin"
        DASHBOARD_PASS = "admin@ceph123"
      }

      resources {
        memory = 1024

        # need to use cores and not cpu unless we use hard limits, or it'll eat
        # all the available cores
        cores = 2
      }

    }
  }
}
