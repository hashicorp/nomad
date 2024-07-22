# notes:

* nomad -> nomad
  * world -> http agent (incl workload id endpoint(s))
  * tls
  * server join
  * servers -> servers (rpc, serf?)
  * clients -> servers (rpc)
* workloads:
  * svc disco -- where does the addr come from?
  * advertise addr in agent config?
  * bridge:
    * nomad iptables admin chain
    * cni bridge config
      * does nomad capture/use the return?
    * firewall plugin? ipt6tables?
  * docker:
    * no ip6tables entries? -- probably because it's not enabled in daemon.json (or w/e)
    * docker driver advertise_ipv6_address
  * podman, other drivers?

* terraform for ipv6 cluster


fedora:
 * dnf install bind-utils # dig
 * does NOT have ipv6 enabled by default: https://reintech.io/blog/setting-up-ipv6-networking-on-fedora-38

