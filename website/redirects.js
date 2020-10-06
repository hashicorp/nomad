module.exports = [
  {
    source: '/api-docs/index(.html)?',
    destination: '/api-docs',
    permanent: true,
  },
  {
    source: '/api/index(.html)?',
    destination: '/api-docs',
    permanent: true,
  },
  {
    source: '/community(.html)?',
    destination: '/resources',
    permanent: true,
  },
  {
    source: '/docs/agent',
    destination: '/docs/install/production/nomad-agent/',
    permanent: true,
  },
  {
    source: '/docs/agent/cloud_auto_join(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/docs/agent/config(.html)?',
    destination: '/docs/configuration',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration',
    destination: '/docs/configuration',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/acl(.html)?',
    destination: '/docs/configuration/acl',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/autopilot(.html)?',
    destination: '/docs/configuration/autopilot',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/client(.html)?',
    destination: '/docs/configuration/client',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/consul(.html)?',
    destination: '/docs/configuration/consul',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/index.html',
    destination: '/docs/configuration',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/sentinel(.html)?',
    destination: '/docs/configuration/sentinel',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/server(.html)?',
    destination: '/docs/configuration/server',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/server_join(.html)?',
    destination: '/docs/configuration/server_join',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/telemetry(.html)?',
    destination: '/docs/configuration/telemetry',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/tls(.html)?',
    destination: '/docs/configuration/tls',
    permanent: true,
  },
  {
    source: '/docs/agent/configuration/vault(.html)?',
    destination: '/docs/configuration/vault',
    permanent: true,
  },
  {
    source: '/docs/agent/encryption(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/security-gossip-encryption',
    permanent: true,
  },
  {
    source: '/docs/agent/index.html',
    destination: '/docs/install/production/nomad-agent/',
    permanent: true,
  },
  {
    source: '/docs/agent/telemetry(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/prometheus-metrics',
    permanent: true,
  },
  {
    source: '/docs/cluster/automatic(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/docs/cluster/federation(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/federation',
    permanent: true,
  },
  {
    source: '/docs/cluster/manual(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/docs/cluster/requirements(.html)?',
    destination: '/docs/install/production/requirements/',
    permanent: true,
  },
  {
    source: '/docs/commands/alloc-status(.html)?',
    destination: '/docs/commands/alloc/status',
    permanent: true,
  },
  {
    source: '/docs/commands/client-config(.html)?',
    destination: '/docs/commands/node/config',
    permanent: true,
  },
  {
    source: '/docs/commands/fs(.html)?',
    destination: '/docs/commands/alloc/fs',
    permanent: true,
  },
  {
    source: '/docs/commands/init(.html)?',
    destination: '/docs/commands/job/init',
    permanent: true,
  },
  {
    source: '/docs/commands/inspect(.html)?',
    destination: '/docs/commands/job/inspect',
    permanent: true,
  },
  {
    source: '/docs/commands/job-dispatch(.html)?',
    destination: '/docs/commands/job/dispatch',
    permanent: true,
  },
  {
    source: '/docs/commands/keygen(.html)?',
    destination: '/docs/commands/operator/keygen',
    permanent: true,
  },
  {
    source: '/docs/commands/keyring(.html)?',
    destination: '/docs/commands/operator/keyring',
    permanent: true,
  },
  {
    source: '/docs/commands/logs(.html)?',
    destination: '/docs/commands/alloc/logs',
    permanent: true,
  },
  {
    source: '/docs/commands/node-drain(.html)?',
    destination: '/docs/commands/node/drain',
    permanent: true,
  },
  {
    source: '/docs/commands/node-status(.html)?',
    destination: '/docs/commands/node/status',
    permanent: true,
  },
  {
    source: '/docs/commands/operator-index(.html)?',
    destination: '/docs/commands/operator',
    permanent: true,
  },
  {
    source: '/docs/commands/operator-raft-list-peers(.html)?',
    destination: '/docs/commands/operator/raft-list-peers',
    permanent: true,
  },
  {
    source: '/docs/commands/operator-raft-remove-peer(.html)?',
    destination: '/docs/commands/operator/raft-remove-peer',
    permanent: true,
  },
  {
    source: '/docs/commands/plan(.html)?',
    destination: '/docs/commands/job/plan',
    permanent: true,
  },
  {
    source: '/docs/commands/run(.html)?',
    destination: '/docs/commands/job/run',
    permanent: true,
  },
  {
    source: '/docs/commands/server-force-leave(.html)?',
    destination: '/docs/commands/server/force-leave',
    permanent: true,
  },
  {
    source: '/docs/commands/server-join(.html)?',
    destination: '/docs/commands/server/join',
    permanent: true,
  },
  {
    source: '/docs/commands/server-members(.html)?',
    destination: '/docs/commands/server/members',
    permanent: true,
  },
  {
    source: '/docs/commands/stop(.html)?',
    destination: '/docs/commands/job/stop',
    permanent: true,
  },
  {
    source: '/docs/commands/validate(.html)?',
    destination: '/docs/commands/job/validate',
    permanent: true,
  },
  {
    source: '/docs/drivers/external/podman',
    destination: '/docs/drivers/podman',
    permanent: true,
  },
  {
    source: '/docs/drivers/lxc(.html)?',
    destination: '/docs/drivers/external/lxc',
    permanent: true,
  },
  {
    source: '/docs/drivers/rkt(.html)?',
    destination: '/docs/drivers/external/rkt',
    permanent: true,
  },
  {
    source: '/docs/enterprise/autopilot/(index.html)?',
    destination: '/docs/enterprise#nomad-enterprise-platform',
    permanent: true,
  },
  {
    source: '/docs/enterprise/namespaces/(index.html)?',
    destination: '/docs/enterprise#namespaces',
    permanent: true,
  },
  {
    source: '/docs/enterprise/preemption/(index.html)?',
    destination: '/docs/enterprise#preemption',
    permanent: true,
  },
  {
    source: '/docs/enterprise/quotas/(index.html)?',
    destination: '/docs/enterprise#resource-quotas',
    permanent: true,
  },
  {
    source: '/docs/enterprise/sentinel/(index.html)?',
    destination: '/docs/enterprise#sentinel-policies',
    permanent: true,
  },
  {
    source: '/docs/http/agent-force-leave(.html)?',
    destination: '/api-docs/agent',
    permanent: true,
  },
  {
    source: '/docs/http/agent-join(.html)?',
    destination: '/api-docs/agent',
    permanent: true,
  },
  {
    source: '/docs/http/agent-members(.html)?',
    destination: '/api-docs/agent',
    permanent: true,
  },
  {
    source: '/docs/http/agent-self(.html)?',
    destination: '/api-docs/agent',
    permanent: true,
  },
  {
    source: '/docs/http/agent-servers(.html)?',
    destination: '/api-docs/agent',
    permanent: true,
  },
  {
    source: '/docs/http/alloc(.html)?',
    destination: '/api-docs/allocations',
    permanent: true,
  },
  {
    source: '/docs/http/allocs(.html)?',
    destination: '/api-docs/allocations',
    permanent: true,
  },
  {
    source: '/docs/http/client-allocation-stats(.html)?',
    destination: '/api-docs/client',
    permanent: true,
  },
  {
    source: '/docs/http/client-fs(.html)?',
    destination: '/api-docs/client',
    permanent: true,
  },
  {
    source: '/docs/http/client-stats(.html)?',
    destination: '/api-docs/client',
    permanent: true,
  },
  {
    source: '/docs/http/eval(.html)?',
    destination: '/api-docs/evaluations',
    permanent: true,
  },
  {
    source: '/docs/http/evals(.html)?',
    destination: '/api-docs/evaluations',
    permanent: true,
  },
  {
    source: '/docs/http/index.html',
    destination: '/api-docs',
    permanent: true,
  },
  {
    source: '/docs/http/job(.html)?',
    destination: '/api-docs/jobs',
    permanent: true,
  },
  {
    source: '/docs/http/jobs(.html)?',
    destination: '/api-docs/jobs',
    permanent: true,
  },
  {
    source: '/docs/http/json-jobs(.html)?',
    destination: '/api-docs/json-jobs',
    permanent: true,
  },
  {
    source: '/docs/http/node(.html)?',
    destination: '/api-docs/nodes',
    permanent: true,
  },
  {
    source: '/docs/http/nodes(.html)?',
    destination: '/api-docs/nodes',
    permanent: true,
  },
  {
    source: '/docs/http/operator(.html)?',
    destination: '/api-docs/operator',
    permanent: true,
  },
  {
    source: '/docs/http/regions(.html)?',
    destination: '/api-docs/regions',
    permanent: true,
  },
  {
    source: '/docs/http/status(.html)?',
    destination: '/api-docs/status',
    permanent: true,
  },
  {
    source: '/docs/http/system(.html)?',
    destination: '/api-docs/system',
    permanent: true,
  },
  {
    source: '/docs/index(.html)?',
    destination: '/docs',
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
  {
    source: '/docs/internals/scheduling(.html)?',
    destination: '/docs/internals/scheduling/scheduling',
    permanent: true,
  },
  {
    source: '/docs/job-specification/index.html',
    destination: '/docs/job-specification',
    permanent: true,
  },
  {
    source: '/docs/jobops/(index.html)?',
    destination: 'https://learn.hashicorp.com/collections/nomad/manage-jobs',
    permanent: true,
  },
  {
    source: '/docs/jobops/inspecting(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-inspect',
    permanent: true,
  },
  {
    source: '/docs/jobops/logs(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/jobs-accessing-logs',
    permanent: true,
  },
  {
    source: '/docs/jobops/resources(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-utilization',
    permanent: true,
  },
  {
    source: '/docs/jobops/servicediscovery(.html)?',
    destination: '/docs/integrations/consul-integration',
    permanent: true,
  },
  {
    source: '/docs/jobops/taskconfig(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-configuring',
    permanent: true,
  },
  {
    source: '/docs/jobops/updating(.html)?',
    destination: 'https://learn.hashicorp.com/collections/nomad/job-updates',
    permanent: true,
  },
  {
    source: '/docs/jobspec',
    destination: '/docs/job-specification',
    permanent: true,
  },
  {
    source: '/docs/jobspec/environment(.html)?',
    destination: '/docs/runtime/environment',
    permanent: true,
  },
  {
    source: '/docs/jobspec/index.html',
    destination: '/docs/job-specification',
    permanent: true,
  },
  {
    source: '/docs/jobspec/interpreted(.html)?',
    destination: '/docs/runtime/interpolation',
    permanent: true,
  },
  {
    source: '/docs/jobspec/json(.html)?',
    destination: '/api-docs/json-jobs',
    permanent: true,
  },
  {
    source: '/docs/jobspec/networking(.html)?',
    destination: '/docs/job-specification/network',
    permanent: true,
  },
  {
    source: '/docs/jobspec/schedulers(.html)?',
    destination: '/docs/schedulers',
    permanent: true,
  },
  {
    source: '/docs/jobspec/servicediscovery(.html)?',
    destination: '/docs/job-specification/service',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job',
    destination: 'https://learn.hashicorp.com/collections/nomad/manage-jobs',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/accessing-logs.html',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/jobs-accessing-logs',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/configuring-tasks.html',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-configuring',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/failure-handling-strategies',
    destination:
      'https://learn.hashicorp.com/collections/nomad/job-failure-handling',
    permanent: true,
  },
  {
    source:
      '/docs/operating-a-job/failure-handling-strategies/check-restart(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/failures-check-restart',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/failure-handling-strategies/index.html',
    destination:
      'https://learn.hashicorp.com/collections/nomad/job-failure-handling',
    permanent: true,
  },
  {
    source:
      '/docs/operating-a-job/failure-handling-strategies/reschedule(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/failures-reschedule',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/failure-handling-strategies/restart(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/failures-restart',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/index.html',
    destination: 'https://learn.hashicorp.com/collections/nomad/manage-jobs',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/inspecting-state.html',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-inspect',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/resource-utilization.html',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-utilization',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/submitting-jobs.html',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-submit',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/update-strategies',
    destination: 'https://learn.hashicorp.com/collections/nomad/job-updates',
    permanent: true,
  },
  {
    source:
      '/docs/operating-a-job/update-strategies/blue-green-and-canary-deployments(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-blue-green-and-canary-deployments',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/update-strategies/handling-signals(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-update-handle-signals',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/update-strategies/index.html',
    destination: 'https://learn.hashicorp.com/collections/nomad/job-updates',
    permanent: true,
  },
  {
    source: '/docs/operating-a-job/update-strategies/rolling-upgrades(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-rolling-update',
    permanent: true,
  },
  {
    source: '/docs/runtime/schedulers(.html)?',
    destination: '/docs/schedulers',
    permanent: true,
  },
  {
    source: '/docs/service-discovery/(index.html)?',
    destination: '/docs/integrations/consul-integration',
    permanent: true,
  },
  {
    source: '/docs/telemetry/overview',
    destination: '/docs/telemetry',
    permanent: true,
  },
  {
    source: '/docs/vault-integration',
    destination: '/docs/integrations/vault-integration',
    permanent: true,
  },
  {
    source: '/guides',
    destination: 'https://learn.hashicorp.com/nomad',
    permanent: true,
  },
  {
    source: '/guides/:splat*',
    destination: 'https://learn.hashicorp.com/nomad',
    permanent: true,
  },
  {
    source: '/guides/acl(.html)?',
    destination: 'https://learn.hashicorp.com/collections/nomad/access-control',
    permanent: true,
  },
  {
    source: '/guides/advanced-scheduling/',
    destination:
      'https://learn.hashicorp.com/collections/nomad/advanced-scheduling',
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
  {
    source: '/guides/autopilot(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/autopilot',
    permanent: true,
  },
  {
    source: '/guides/cluster/automatic(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/cluster/bootstrapping(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/cluster/federation',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/federation',
    permanent: true,
  },
  {
    source: '/guides/cluster/manual(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/cluster/requirements',
    destination: '/docs/install/production/requirements',
    permanent: true,
  },
  {
    source: '/guides/external/(index.html)?',
    destination: 'https://learn.hashicorp.com/collections/nomad/plugins',
    permanent: true,
  },
  {
    source: '/guides/external/lxc(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/plugin-lxc',
    permanent: true,
  },
  {
    source: '/guides/governance-and-policy',
    destination:
      'https://learn.hashicorp.com/collections/nomad/governance-and-policy',
    permanent: true,
  },
  {
    source: '/guides/governance-and-policy/namespaces(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/namespaces',
    permanent: true,
  },
  {
    source: '/guides/governance-and-policy/quotas(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/quotas',
    permanent: true,
  },
  {
    source: '/guides/governance-and-policy/sentinel/job(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/sentinel',
    permanent: true,
  },
  {
    source: '/guides/governance-and-policy/sentinel/sentinel-policy(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/sentinel',
    permanent: true,
  },
  {
    source: '/guides/load-balancing',
    destination: 'https://learn.hashicorp.com/collections/nomad/load-balancing',
    permanent: true,
  },
  {
    source: '/guides/load-balancing/fabio(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/load-balancing-fabio',
    permanent: true,
  },
  {
    source: '/guides/load-balancing/haproxy(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/load-balancing-haproxy',
    permanent: true,
  },
  {
    source: '/guides/load-balancing/nginx(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/load-balancing-nginx',
    permanent: true,
  },
  {
    source: '/guides/load-balancing/traefik(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/load-balancing-traefik',
    permanent: true,
  },
  {
    source: '/guides/namespaces(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/namespaces',
    permanent: true,
  },
  {
    source: '/guides/node-draining(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/node-drain',
    permanent: true,
  },
  {
    source: '/guides/nomad-metrics(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/prometheus-metrics',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job',
    destination: 'https://learn.hashicorp.com/collections/nomad/manage-jobs',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/accessing-logs(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/jobs-accessing-logs',
    permanent: true,
  },
  {
    source:
      '/guides/operating-a-job/advanced-scheduling/advanced-scheduling(.html)?',
    destination:
      'https://learn.hashicorp.com/collections/nomad/advanced-scheduling',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/advanced-scheduling/affinity(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/affinity',
    permanent: true,
  },
  {
    source:
      '/guides/operating-a-job/advanced-scheduling/preemption-service-batch(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/preemption',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/advanced-scheduling/spread(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/spread',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/configuring-tasks(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-configuring',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/external/(index.html)?',
    destination: 'https://learn.hashicorp.com/collections/nomad/plugins',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/external/lxc(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/plugin-lxc',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/failure-handling-strategies',
    destination:
      'https://learn.hashicorp.com/collections/nomad/job-failure-handling',
    permanent: true,
  },
  {
    source:
      '/guides/operating-a-job/failure-handling-strategies/check-restart(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/failures-check-restart',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/failure-handling-strategies/index.html',
    destination:
      'https://learn.hashicorp.com/collections/nomad/job-failure-handling',
    permanent: true,
  },
  {
    source:
      '/guides/operating-a-job/failure-handling-strategies/reschedule(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/failures-reschedule',
    permanent: true,
  },
  {
    source:
      '/guides/operating-a-job/failure-handling-strategies/restart(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/failures-restart',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/index.html',
    destination: 'https://learn.hashicorp.com/collections/nomad/manage-jobs',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/inspecting-state(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-inspec',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/resource-utilization(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-utilization',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/submitting-jobs(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/jobs-submit',
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
      '/guides/operating-a-job/update-strategies/blue-green-and-canary-deployments(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-blue-green-and-canary-deployments',
    permanent: true,
  },
  {
    source:
      '/guides/operating-a-job/update-strategies/handling-signals(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-update-handle-signals',
    permanent: true,
  },
  {
    source: '/guides/operating-a-job/update-strategies/index.html',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/job-update-strategies',
    permanent: true,
  },
  {
    source:
      '/guides/operating-a-job/update-strategies/rolling-upgrades(.html)?',
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
    source: '/guides/operations/agent/index.html',
    destination: '/docs/install/production/nomad-agent',
    permanent: true,
  },
  {
    source: '/guides/operations/autopilot(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/autopilot',
    permanent: true,
  },
  {
    source: '/guides/operations/cluster/automatic(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/operations/cluster/bootstrapping(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/operations/cluster/bootstrapping.html',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/operations/cluster/cloud_auto_join(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/operations/cluster/manual(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/clustering',
    permanent: true,
  },
  {
    source: '/guides/operations/consul-integration/index.html',
    destination: '/docs/integrations/consul-integration',
    permanent: true,
  },
  {
    source: '/guides/operations/deployment-guide(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/production-deployment-guide-vm-with-consul',
    permanent: true,
  },
  {
    source: '/guides/operations/federation(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/federation',
    permanent: true,
  },
  {
    source: '/guides/operations/index.html',
    destination:
      'https://learn.hashicorp.com/collections/nomad/manage-clusters',
    permanent: true,
  },
  {
    source: '/guides/operations/install/index(.html)?',
    destination: '/docs/install',
    permanent: true,
  },
  {
    source: '/guides/operations/monitoring-and-alerting/monitoring(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/prometheus-metrics',
    permanent: true,
  },
  {
    source:
      '/guides/operations/monitoring-and-alerting/prometheus-metrics(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/prometheus-metrics',
    permanent: true,
  },
  {
    source: '/guides/operations/monitoring/nomad-metrics(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/prometheus-metrics',
    permanent: true,
  },
  {
    source: '/guides/operations/node-draining(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/node-drain',
    permanent: true,
  },
  {
    source: '/guides/operations/outage(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/outage-recovery',
    permanent: true,
  },
  {
    source: '/guides/operations/reference-architecture(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/production-reference-architecture-vm-with-consul',
    permanent: true,
  },
  {
    source: '/guides/operations/requirements(.html)?',
    destination: '/docs/install/production/requirements',
    permanent: true,
  },
  {
    source: '/guides/operations/upgrade/(index.html)?',
    destination: '/docs/upgrade',
    permanent: true,
  },
  {
    source: '/guides/operations/upgrade/upgrade-specific(.html)?',
    destination: '/docs/upgrade/upgrade-specific',
    permanent: true,
  },
  {
    source: '/guides/operations/vault-integration/index.html',
    destination: '/docs/integrations/vault-integration',
    permanent: true,
  },
  {
    source: '/guides/outage(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/outage-recovery',
    permanent: true,
  },
  {
    source: '/guides/quotas(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/quotas',
    permanent: true,
  },
  {
    source: '/guides/securing-nomad(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/security-enable-tls',
    permanent: true,
  },
  {
    source: '/guides/security',
    destination: 'https://learn.hashicorp.com/collections/nomad/access-control',
    permanent: true,
  },
  {
    source: '/guides/security/acl(.html)?',
    destination: 'https://learn.hashicorp.com/collections/nomad/access-control',
    permanent: true,
  },
  {
    source: '/guides/security/encryption(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/security-gossip-encryption',
    permanent: true,
  },
  {
    source: '/guides/security/index.html',
    destination: 'https://learn.hashicorp.com/collections/nomad/access-control',
    permanent: true,
  },
  {
    source: '/guides/security/namespaces(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/namespaces',
    permanent: true,
  },
  {
    source: '/guides/security/quotas(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/quotas',
    permanent: true,
  },
  {
    source: '/guides/security/securing-nomad(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/security-enable-tls',
    permanent: true,
  },
  {
    source: '/guides/security/sentinel-policy(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/sentinel',
    permanent: true,
  },
  {
    source: '/guides/security/sentinel/job(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/quotas',
    permanent: true,
  },
  {
    source: '/guides/security/vault-pki-integration(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/vault-pki-nomad',
    permanent: true,
  },
  {
    source: '/guides/sentinel-policy(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/sentinel',
    permanent: true,
  },
  {
    source: '/guides/sentinel/job(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/sentinel',
    permanent: true,
  },
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
    source: '/guides/stateful-workloads',
    destination:
      'https://learn.hashicorp.com/collections/nomad/stateful-workloads',
    permanent: true,
  },
  {
    source: '/guides/stateful-workloads/host-volumes(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/stateful-workloads-host-volumes',
    permanent: true,
  },
  {
    source: '/guides/stateful-workloads/portworx(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/stateful-workloads-portworx',
    permanent: true,
  },
  {
    source: '/guides/ui(.html)?',
    destination: 'https://learn.hashicorp.com/collections/nomad/web-ui',
    permanent: true,
  },
  {
    source: '/guides/upgrade/(index.html)?',
    destination: '/docs/upgrade',
    permanent: true,
  },
  {
    source: '/guides/upgrade/upgrade-specific(.html)?',
    destination: '/docs/upgrade/upgrade-specific',
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
    source: '/guides/web-ui/inspecting-the-cluster(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/web-ui-cluster-info',
    permanent: true,
  },
  {
    source: '/guides/web-ui/operating-a-job(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/web-ui-submit-job',
    permanent: true,
  },
  {
    source: '/guides/web-ui/securing(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/web-ui-access#access-an-acl-enabled-ui',
    permanent: true,
  },
  {
    source: '/guides/web-ui/submitting-a-job(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/web-ui-workload-info',
    permanent: true,
  },
  {
    source: '/heartyeet',
    destination: '/docs/job-specification/group#stop_after_client_disconnect',
    permanent: true,
  },
  {
    source: '/intro/getting-started/cluster(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/get-started-cluster',
    permanent: true,
  },
  {
    source: '/intro/getting-started/install(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/get-started-install',
    permanent: true,
  },
  {
    source: '/intro/getting-started/jobs(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/get-started-jobs',
    permanent: true,
  },
  {
    source: '/intro/getting-started/next-steps(.html)?',
    destination:
      'https://learn.hashicorp.com/tutorials/nomad/get-started-learn-more',
    permanent: true,
  },
  {
    source: '/intro/getting-started/running(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/get-started-run',
    permanent: true,
  },
  {
    source: '/intro/getting-started/ui(.html)?',
    destination: 'https://learn.hashicorp.com/tutorials/nomad/get-started-ui',
    permanent: true,
  },
]
