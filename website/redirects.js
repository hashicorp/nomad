module.exports = [
  // Define your custom redirects within this file.
  // Vercel's redirect documentation: https://vercel.com/docs/configuration#project/redirects
  // Playground for testing url pattern matching: https://npm.runkit.com/path-to-regexp

  {
    source: '/docs/operations/overview',
    destination: '/docs/operations',
    permanent: true,
  },

  // This redirect supports a URL built into the Nomad UI, and should always be
  // directed someplace valid to support the text "Read about Outage Recovery."
  {
    source: '/guides/outage.html',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/outage-recovery',
    permanent: true,
  },

  // Nomad Learn Redirects
  {
    source: '/intro/getting-started',
    destination: 'https://learn.hashicorp.com/collections/nomad/get-started',
    permanent: true,
  },
  {
    source: '/intro/getting-started/install',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/get-started-install',
    permanent: true,
  },
  {
    source: '/intro/getting-started/running',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/get-started-run',
    permanent: true,
  },
  {
    source: '/intro/getting-started/jobs',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/get-started-jobs',
    permanent: true,
  },
  {
    source: '/intro/getting-started/cluster',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/get-started-cluster',
    permanent: true,
  },
  {
    source: '/intro/getting-started/ui',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/get-started-ui',
    permanent: true,
  },
  {
    source: '/intro/getting-started/next-steps',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/get-started-learn-more',
    permanent: true,
  },
  {
    source: '/intro/vs/kubernetes',
    destination: '/docs/nomad-vs-kubernetes',
    permanent: true,
  },
  {
    source: '/intro/who-uses-nomad',
    destination: '/docs/who-uses/noamd',
    permanent: true,
  },
  // Guides
  {
    source: '/guides/load-balancing',
    destination: 'https://learn.hashicorp.com/collections/nomad/load-balancing',
    permanent: true,
  },
  {
    source: '/guides/load-balancing/fabio',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/load-balancing-fabio',
    permanent: true,
  },
  {
    source: '/guides/load-balancing/nginx',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/load-balancing-nginx',
    permanent: true,
  },
  {
    source: '/guides/load-balancing/haproxy',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/load-balancing-haproxy',
    permanent: true,
  },
  {
    source: '/guides/load-balancing/traefik',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/load-balancing-traefik',
    permanent: true,
  },

  {
    source: '/guides/stateful-workloads',
    destination:
      'https://learn.hashicorp.com/collections/nomad/stateful-workloads',
    permanent: true,
  },
  {
    source: '/guides/stateful-workloads/host-volumes',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/stateful-workloads-host-volumes',
    permanent: true,
  },
  {
    source: '/guides/stateful-workloads/portworx',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/stateful-workloads-portworx',
    permanent: true,
  },

  {
    source: '/guides/web-ui',
    destination: 'https://learn.hashicorp.com/collections/nomad/web-ui',
    permanent: true,
  },
  {
    source: '/guides/web-ui/access.html',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/web-ui-access',
    permanent: true,
  },
  {
    source: '/guides/web-ui/accessing',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/web-ui-access',
    permanent: true,
  },
  {
    source: '/guides/web-ui/submitting-a-job',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/web-ui-workload-info',
    permanent: true,
  },
  {
    source: '/guides/web-ui/operating-a-job',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/web-ui-submit-job',
    permanent: true,
  },
  {
    source: '/guides/web-ui/inspecting-the-cluster',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/web-ui-cluster-info',
    permanent: true,
  },
  {
    source: '/guides/web-ui/securing',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/web-ui-access#access-an-acl-enabled-ui',
    permanent: true,
  },

  {
    source: '/guides/governance-and-policy',
    destination:
      'https://learn.hashicorp.com/collections/nomad/governance-and-policy',
    permanent: true,
  },
  {
    source: '/guides/governance-and-policy/namespaces',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/namespaces',
    permanent: true,
  },
  {
    source: '/guides/governance-and-policy/quotas',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/quotas',
    permanent: true,
  },
  {
    source: '/guides/governance-and-policy/sentinel/sentinel-policy',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/sentinel',
    permanent: true,
  },
  {
    source: '/guides/governance-and-policy/sentinel/job',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/sentinel',
    permanent: true,
  },

  // /s/* redirects for useful links that need a stable URL but we may need to
  // change its destination in the future.
  {
    source: '/s/port-plan-failure',
    destination: 'https://github.com/hashicorp/nomad/issues/9506',
    permanent: false,
  },

  // Spark guide links are all repointed to deprecated nomad-spark repo
  {
    source: '/guides/spark',
    destination: 'https://github.com/hashicorp/nomad-spark',
    permanent: true,
  },
  {
    source: '/guides/spark/:splat*',
    destination: 'https://github.com/hashicorp/nomad-spark',
    permanent: true,
  },
  {
    source: '/guides/analytical-workloads',
    destination: 'https://github.com/hashicorp/nomad-spark',
    permanent: true,
  },
  {
    source: '/guides/analytical-workloads/:splat*',
    destination: 'https://github.com/hashicorp/nomad-spark',
    permanent: true,
  },
  // These are subsumed in the splat over analytical-workloads, but I'm keeping them here in case
  // we want to reclaim the slug "analytical-workloads" at some point and deal with some bad links.
  // {
  //   source: '/guides/analytical-workloads/spark/spark',
  //   destination: 'https://github.com/hashicorp/nomad-spark',
  //   permanent: true,
  // },

  {
    source: '/guides/operating-a-job',
    destination: 'https://learn.hashicorp.com/collections/nomad/manage-jobs',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/accessing-logs',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/jobs-accessing-logs',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/configuring-tasks',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-configuring',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/external',
    destination: 'https://learn.hashicorp.com/collections/nomad/plugins',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/external/lxc',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/plugin-lxc',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/inspecting-state',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-inspec',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/resource-utilization',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-utilization',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/submitting-jobs',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-submit',
    permanent: true,
  },

  {
    source: '/guides/operating-a-job/advanced-scheduling/advanced-scheduling',
    destination:
      'https://learn.hashicorp.com/collections/nomad/advanced-scheduling',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/advanced-scheduling/affinity',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/affinity',
    permanent: true,
  },
  {
    source:
      '/guides/operating-a-job/advanced-scheduling/preemption-service-batch',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/preemption',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/advanced-scheduling/spread',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/spread',
    permanent: true,
  },

  {
    source: '/guides/operating-a-job/failure-handling-strategies',
    destination:
      'https://learn.hashicorp.com/collections/nomad/job-failure-handling',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/failure-handling-strategies/check-restart',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/failures-check-restart',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/failure-handling-strategies/reschedule',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/failures-reschedule',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/failure-handling-strategies/restart',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/failures-restart',
    permanent: true,
  },

  {
    source: '/guides/operating-a-job/update-strategies',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-update-strategies',
    permanent: true,
  },
  {
    source:
      '/guides/operating-a-job/update-strategies/blue-green-and-canary-deployments',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-blue-green-and-canary-deployments',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/update-strategies/handling-signals',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-update-handle-signals',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/update-strategies/rolling-upgrades',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-rolling-update',
    permanent: true,
  },

  {
    source: '/guides/operations',
    destination:
      'https://learn.hashicorp.com/collections/nomad/manage-clusters',
    permanent: true,
  },
  {
    source: '/guides/operations/autopilot',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/autopilot',
    permanent: true,
  },
  {
    source: '/guides/operations/cluster/automatic',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/operations/cluster/bootstrapping',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/operations/cluster/cloud_auto_join',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/operations/cluster/manual',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/operations/federation',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/federation',
    permanent: true,
  },
  {
    source: '/guides/operations/monitoring-and-alerting/monitoring',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/prometheus-metrics',
    permanent: true,
  },
  {
    source: '/guides/operations/monitoring-and-alerting/prometheus-metrics',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/prometheus-metrics',
    permanent: true,
  },
  {
    source: '/guides/operations/monitoring/nomad-metrics',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/prometheus-metrics',
    permanent: true,
  },
  {
    source: '/guides/operations/node-draining',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/node-drain',
    permanent: true,
  },
  {
    source: '/guides/operations/outage',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/outage-recovery',
    permanent: true,
  },

  {
    source: '/guides/security',
    destination: 'https://learn.hashicorp.com/collections/nomad/access-control',
    permanent: true,
  },
  {
    source: '/guides/security/acl',
    destination: 'https://learn.hashicorp.com/collections/nomad/access-control',
    permanent: true,
  },
  {
    source: '/guides/security/encryption',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/security-gossip-encryption',
    permanent: true,
  },
  {
    source: '/guides/security/securing-nomad',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/security-enable-tls',
    permanent: true,
  },
  {
    source: '/guides/security/vault-pki-integration',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/vault-pki-nomad',
    permanent: true,
  },

  // Multi-part UI guides
  {
    source: '/guides/ui',
    destination: 'https://learn.hashicorp.com/collections/nomad/web-ui',
    permanent: true,
  },

  // Website
  // Docs
  {
    source: '/docs/index',
    destination: '/docs',
    permanent: true,
  },
  {
    source: '/api/index',
    destination: '/api-docs',
    permanent: true,
  },
  {
    source: '/api-docs/index',
    destination: '/api-docs',
    permanent: true,
  },
  {
    source: '/docs/agent/config',
    destination: '/docs/configuration',
    permanent: true,
  },
  {
    source: '/docs/jobops',
    destination: 'https://learn.hashicorp.com/collections/nomad/manage-jobs',
    permanent: true,
  },
  {
    source: '/docs/jobops/taskconfig',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-configuring',
    permanent: true,
  },
  {
    source: '/docs/jobops/inspecting',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-inspect',
    permanent: true,
  },
  {
    source: '/docs/jobops/resources',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-utilization',
    permanent: true,
  },
  {
    source: '/docs/jobops/logs',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/jobs-accessing-logs',
    permanent: true,
  },
  {
    source: '/docs/jobops/updating',
    destination: 'https://learn.hashicorp.com/collections/nomad/job-updates',
    permanent: true,
  },
  {
    source: '/docs/jobops/servicediscovery',
    destination: '/docs/integrations/consul-integration',
    permanent: true,
  },
  {
    source: '/docs/jobspec',
    destination: '/docs/job-specification',
    permanent: true,
  },
  {
    source: '/docs/jobspec/interpreted',
    destination: '/docs/runtime/interpolation',
    permanent: true,
  },
  {
    source: '/docs/jobspec/json',
    destination: '/api-docs/json-jobs',
    permanent: true,
  },
  {
    source: '/docs/jobspec/environment',
    destination: '/docs/runtime/environment',
    permanent: true,
  },
  {
    source: '/docs/jobspec/schedulers',
    destination: '/docs/schedulers',
    permanent: true,
  },
  {
    source: '/docs/jobspec/servicediscovery',
    destination: '/docs/job-specification/service',
    permanent: true,
  },
  {
    source: '/docs/jobspec/networking',
    destination: '/docs/job-specification/network',
    permanent: true,
  },
  {
    source: '/docs/job-specification/index',
    destination: '/docs/job-specification',
    permanent: true,
  },
  {
    source: '/docs/cluster/automatic',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/docs/cluster/manual',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/docs/cluster/federation',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/federation',
    permanent: true,
  },
  {
    source: '/docs/cluster/requirements',
    destination: '/docs/install/production/requirements/',
    permanent: true,
  },
  {
    source: '/docs/commands/operator-index',
    destination: '/docs/commands/operator',
    permanent: true,
  },
  {
    source: '/docs/commands/operator-raft-list-peers',
    destination: '/docs/commands/operator/raft-list-peers',
    permanent: true,
  },
  {
    source: '/docs/commands/operator-raft-remove-peer',
    destination: '/docs/commands/operator/raft-remove-peer',
    permanent: true,
  },
  {
    source: '/docs/commands/job-dispatch',
    destination: '/docs/commands/job/dispatch',
    permanent: true,
  },
  {
    source: '/docs/commands/alloc-status',
    destination: '/docs/commands/alloc/status',
    permanent: true,
  },
  {
    source: '/docs/commands/fs',
    destination: '/docs/commands/alloc/fs',
    permanent: true,
  },
  {
    source: '/docs/commands/logs',
    destination: '/docs/commands/alloc/logs',
    permanent: true,
  },
  {
    source: '/docs/commands/init',
    destination: '/docs/commands/job/init',
    permanent: true,
  },
  {
    source: '/docs/commands/inspect',
    destination: '/docs/commands/job/inspect',
    permanent: true,
  },
  {
    source: '/docs/commands/run',
    destination: '/docs/commands/job/run',
    permanent: true,
  },
  {
    source: '/docs/commands/stop',
    destination: '/docs/commands/job/stop',
    permanent: true,
  },
  {
    source: '/docs/commands/plan',
    destination: '/docs/commands/job/plan',
    permanent: true,
  },
  {
    source: '/docs/commands/validate',
    destination: '/docs/commands/job/validate',
    permanent: true,
  },
  {
    source: '/docs/commands/client-config',
    destination: '/docs/commands/node/config',
    permanent: true,
  },
  {
    source: '/docs/commands/node-drain',
    destination: '/docs/commands/node/drain',
    permanent: true,
  },
  {
    source: '/docs/commands/node-status',
    destination: '/docs/commands/node/status',
    permanent: true,
  },
  {
    source: '/docs/commands/keygen',
    destination: '/docs/commands/operator/keygen',
    permanent: true,
  },
  {
    source: '/docs/commands/keyring',
    destination: '/docs/commands/operator/keyring',
    permanent: true,
  },
  {
    source: '/docs/commands/server-force-leave',
    destination: '/docs/commands/server/force-leave',
    permanent: true,
  },
  {
    source: '/docs/commands/server-join',
    destination: '/docs/commands/server/join',
    permanent: true,
  },
  {
    source: '/docs/commands/server-members',
    destination: '/docs/commands/server/members',
    permanent: true,
  },
  {
    source: '/docs/runtime/schedulers',
    destination: '/docs/schedulers',
    permanent: true,
  },
  {
    source: '/docs/internals/scheduling',
    destination: '/docs/internals/scheduling/scheduling',
    permanent: true,
  },

  // Sometimes code names are too good not to mention
  {
    source: '/heartyeet',
    destination: '/docs/job-specification/group#stop_after_client_disconnect',
    permanent: true,
  },

  // Moved /docs/drivers/external/podman -> /docs/drivers/podman
  {
    source: '/docs/drivers/external/podman',
    destination: '/docs/drivers/podman',
    permanent: true,
  },

  // Moved /docs/operating-a-job/ -> /guides/operating-a-job/
  {
    source: '/docs/operating-a-job',
    destination: 'https://learn.hashicorp.com/collections/nomad/manage-jobs',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/accessing-logs',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/jobs-accessing-logs',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/inspecting-state',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-inspect',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/resource-utilization',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-utilization',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/configuring-tasks',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-configuring',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/submitting-jobs',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-submit',
    permanent: true,
  },

  {
    source: '/docs/operating-a-job/failure-handling-strategies',
    destination:
      'https://learn.hashicorp.com/collections/nomad/job-failure-handling',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/failure-handling-strategies/check-restart',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/failures-check-restart',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/failure-handling-strategies/reschedule',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/failures-reschedule',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/failure-handling-strategies/restart',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/failures-restart',
    permanent: true,
  },

  {
    source: '/docs/operating-a-job/update-strategies',
    destination: 'https://learn.hashicorp.com/collections/nomad/job-updates',
    permanent: true,
  },
  {
    source:
      '/docs/operating-a-job/update-strategies/blue-green-and-canary-deployments',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-blue-green-and-canary-deployments',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/update-strategies/handling-signals',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-update-handle-signals',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/update-strategies/rolling-upgrades',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-rolling-update',
    permanent: true,
  },

  // Moved /docs/agent/configuration/ -> /docs/configuration/ 301!
  {
    source: '/docs/agent/configuration',
    destination: '/docs/configuration',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/acl',
    destination: '/docs/configuration/acl',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/autopilot',
    destination: '/docs/configuration/autopilot',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/client',
    destination: '/docs/configuration/client',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/consul',
    destination: '/docs/configuration/consul',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/sentinel',
    destination: '/docs/configuration/sentinel',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/server',
    destination: '/docs/configuration/server',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/server_join',
    destination: '/docs/configuration/server_join',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/telemetry',
    destination: '/docs/configuration/telemetry',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/tls',
    destination: '/docs/configuration/tls',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/vault',
    destination: '/docs/configuration/vault',
    permanent: true,
  },
  // Moved telemetry under operations
  {
    source: '/docs/telemetry',
    destination: '/docs/operations/telemetry',
    permanent: true,
  },
  {
    source: '/docs/telemetry/metrics',
    destination: '/docs/operations/metrics',
    permanent: true,
  },

  // Moved installing agent under operations as ope
  {
    source: '/docs/install/production/nomad-agent',
    destination: '/docs/operations/nomad-agent',
    permanent: true,
  },

  // Moved guide-like docs to /guides
  {
    source: '/docs/agent',
    destination: '/docs/install/production/nomad-agent/',
    permanent: true,
  },
  {
    source: '/docs/agent/cloud_auto_join',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/docs/agent/telemetry',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/prometheus-metrics',
    permanent: true,
  },
  {
    source: '/docs/agent/encryption',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/security-gossip-encryption',
    permanent: true,
  },
  {
    source: '/docs/service-discovery',
    destination: '/docs/integrations/consul-integration',
    permanent: true,
  },

  // Redirect old LXC driver doc to new one in /docs/external
  {
    source: '/docs/drivers/lxc',
    destination: '/docs/drivers/external/lxc',
    permanent: true,
  },
  {
    source: '/docs/drivers/rkt',
    destination: '/docs/drivers/external/rkt',
    permanent: true,
  },

  // API
  {
    source: '/docs/http/agent-force-leave',
    destination: '/api-docs/agent',
    permanent: true,
  },
  {
    source: '/docs/http/agent-join',
    destination: '/api-docs/agent',
    permanent: true,
  },
  {
    source: '/docs/http/agent-members',
    destination: '/api-docs/agent',
    permanent: true,
  },
  {
    source: '/docs/http/agent-self',
    destination: '/api-docs/agent',
    permanent: true,
  },
  {
    source: '/docs/http/agent-servers',
    destination: '/api-docs/agent',
    permanent: true,
  },
  {
    source: '/docs/http/alloc',
    destination: '/api-docs/allocations',
    permanent: true,
  },
  {
    source: '/docs/http/allocs',
    destination: '/api-docs/allocations',
    permanent: true,
  },
  {
    source: '/docs/http/client-allocation-stats',
    destination: '/api-docs/client',
    permanent: true,
  },
  {
    source: '/docs/http/client-fs',
    destination: '/api-docs/client',
    permanent: true,
  },
  {
    source: '/docs/http/client-stats',
    destination: '/api-docs/client',
    permanent: true,
  },
  {
    source: '/docs/http/eval',
    destination: '/api-docs/evaluations',
    permanent: true,
  },
  {
    source: '/docs/http/evals',
    destination: '/api-docs/evaluations',
    permanent: true,
  },
  {
    source: '/docs/http',
    destination: '/api-docs',
    permanent: true,
  },
  {
    source: '/docs/http/job',
    destination: '/api-docs/jobs',
    permanent: true,
  },
  {
    source: '/docs/http/jobs',
    destination: '/api-docs/jobs',
    permanent: true,
  },
  {
    source: '/docs/http/json-jobs',
    destination: '/api-docs/json-jobs',
    permanent: true,
  },
  {
    source: '/docs/http/node',
    destination: '/api-docs/nodes',
    permanent: true,
  },
  {
    source: '/docs/http/nodes',
    destination: '/api-docs/nodes',
    permanent: true,
  },
  {
    source: '/docs/http/operator',
    destination: '/api-docs/operator',
    permanent: true,
  },
  {
    source: '/docs/http/regions',
    destination: '/api-docs/regions',
    permanent: true,
  },
  {
    source: '/docs/http/status',
    destination: '/api-docs/status',
    permanent: true,
  },
  {
    source: '/docs/http/system',
    destination: '/api-docs/system',
    permanent: true,
  },

  // Guides

  // Reorganized Guides by Persona
  {
    source: '/guides/autopilot',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/autopilot',
    permanent: true,
  },
  {
    source: '/guides/cluster/automatic',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/cluster/bootstrapping',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/operations/cluster/bootstrapping',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/cluster/manual',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/cluster/federation',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/federation',
    permanent: true,
  },
  {
    source: '/guides/cluster/requirements',
    destination: '/docs/install/production/requirements',
    permanent: true,
  },
  {
    source: '/guides/nomad-metrics',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/prometheus-metrics',
    permanent: true,
  },
  {
    source: '/guides/node-draining',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/node-drain',
    permanent: true,
  },
  {
    source: '/guides/outage',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/outage-recovery',
    permanent: true,
  },
  {
    source: '/guides/acl',
    destination: 'https://learn.hashicorp.com/collections/nomad/access-control',
    permanent: true,
  },
  {
    source: '/guides/namespaces',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/namespaces',
    permanent: true,
  },
  {
    source: '/guides/quotas',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/quotas',
    permanent: true,
  },
  {
    source: '/guides/securing-nomad',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/security-enable-tls',
    permanent: true,
  },
  {
    source: '/guides/sentinel-policy',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/sentinel',
    permanent: true,
  },
  {
    source: '/guides/sentinel/job',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/sentinel',
    permanent: true,
  },

  {
    source: '/guides/security/namespaces',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/namespaces',
    permanent: true,
  },
  {
    source: '/guides/security/quotas',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/quotas',
    permanent: true,
  },
  {
    source: '/guides/security/sentinel/job',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/quotas',
    permanent: true,
  },
  {
    source: '/guides/security/sentinel-policy',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/sentinel',
    permanent: true,
  },
  {
    source: '/guides/operations/install/index',
    destination: '/docs/install',
    permanent: true,
  },
  {
    source: '/guides/operations/agent/index',
    destination: '/docs/install/production/nomad-agent',
    permanent: true,
  },
  {
    source: '/guides/operations/requirements',
    destination: '/docs/install/production/requirements',
    permanent: true,
  },
  {
    source: '/guides/operations/consul-integration/index',
    destination: '/docs/integrations/consul-integration',
    permanent: true,
  },
  {
    source: '/guides/operations/vault-integration/index',
    destination: '/docs/integrations/vault-integration',
    permanent: true,
  },
  {
    source: '/guides/advanced-scheduling',
    destination:
      'https://learn.hashicorp.com/collections/nomad/advanced-scheduling',
    permanent: true,
  },
  {
    source: '/guides/external',
    destination: 'https://learn.hashicorp.com/collections/nomad/plugins',
    permanent: true,
  },
  {
    source: '/guides/external/lxc',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/plugin-lxc',
    permanent: true,
  },
  {
    source: '/guides/operations/upgrade',
    destination: '/docs/upgrade',
    permanent: true,
  },
  {
    source: '/guides/operations/upgrade/upgrade-specific',
    destination: '/docs/upgrade/upgrade-specific',
    permanent: true,
  },
  {
    source: '/guides/upgrade',
    destination: '/docs/upgrade',
    permanent: true,
  },
  {
    source: '/guides/upgrade/upgrade-specific',
    destination: '/docs/upgrade/upgrade-specific',
    permanent: true,
  },

  {
    source: '/guides/operations/deployment-guide',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/production-deployment-guide-vm-with-consul',
    permanent: true,
  },
  {
    source: '/guides/operations/reference-architecture',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/production-reference-architecture-vm-with-consul',
    permanent: true,
  },
  {
    source: '/docs/install/production/deployment-guide',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/production-deployment-guide-vm-with-consul',
    permanent: true,
  },
  {
    source: '/docs/install/production/reference-architecture',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/production-reference-architecture-vm-with-consul',
    permanent: true,
  },

  // Enterprise

  // Reorganized Enterprise into single pager
  {
    source: '/docs/enterprise/namespaces',
    destination: '/docs/enterprise#namespaces',
    permanent: true,
  },
  {
    source: '/docs/enterprise/quotas',
    destination: '/docs/enterprise#resource-quotas',
    permanent: true,
  },
  {
    source: '/docs/enterprise/preemption',
    destination: '/docs/enterprise#preemption',
    permanent: true,
  },
  {
    source: '/docs/enterprise/sentinel',
    destination: '/docs/enterprise#sentinel-policies',
    permanent: true,
  },
  {
    source: '/docs/enterprise/autopilot',
    destination: '/docs/enterprise#nomad-enterprise-platform',
    permanent: true,
  },

  // Guide Catch-all Redirects
  {
    source: '/guides/:splat*',
    destination: 'https://learn.hashicorp.com/nomad',
    permanent: true,
  },

  // Vault Integration
  {
    source: '/docs/vault-integration',
    destination: '/docs/integrations/vault-integration',
    permanent: true,
  },
  // Old resources -> Community
  {
    source: '/resources',
    destination: '/community',
    permanent: true,
  },
  // `/<path>/index.html` to /<path>
  {
    source: '/:splat*/index.html',
    destination: '/:splat*',
    permanent: true,
  },
  // `.html` to non-`.html`
  {
    source: '/:splat(.*).html',
    destination: '/:splat',
    permanent: true,
  },
]
