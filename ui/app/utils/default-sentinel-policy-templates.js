/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// TODO: Several of these are placeholders. Fill them out with real policies or remove them.

import countLimitsPolicy from './sentinel_policy_templates/count-limits';
import noFridaysPolicy from './sentinel_policy_templates/no-friday-deploys';
import alwaysFailPolicy from './sentinel_policy_templates/always-fail';
// import enforceServiceChecksPolicy from './sentinel_policy_templates/enforce-service-checks';
import canariesOnlyPolicy from './sentinel_policy_templates/canaries-only';
// import enforceMeshPolicy from './sentinel_policy_templates/enforce-service-mesh';
// import metadataPolicy from './sentinel_policy_templates/metadata-required';
// import consulSDPolicy from './sentinel_policy_templates/consul-service-discovery-only';
// import vaultSecretsPolicy from './sentinel_policy_templates/vault-secrets-only';
// import dyanmicPortsPolicy from './sentinel_policy_templates/dynamic-ports-only';
import resourceLimitsPolicy from './sentinel_policy_templates/resource-limits';
// import constraintEnforcmentPolicy from './sentinel_policy_templates/constraint-enforcement';
import restictImagesPolicy from './sentinel_policy_templates/restrict-images';

export default [
  {
    displayName: 'Count Limits',
    name: 'count-limits',
    description: 'Enforces that no task group has a count over 100',
    policy: countLimitsPolicy,
  },
  {
    displayName: 'No Friday Deploys',
    name: 'no-friday-deploys',
    description: 'Ensures that no deploys happen on a Friday',
    policy: noFridaysPolicy,
  },
  {
    displayName: 'Always Fail',
    name: 'always-fail',
    description: 'A test Sentinel Policy that will always fail',
    policy: alwaysFailPolicy,
  },
  // {
  //   displayName: 'Enforce Service Checks',
  //   name: 'enforce-service-checks',
  //   description: 'Ensures every service has an http health check',
  //   policy: enforceServiceChecksPolicy,
  // },
  {
    displayName: 'Canaries Only',
    name: 'canaries-only',
    description: 'All deployments must have a canary',
    policy: canariesOnlyPolicy,
  },
  // {
  //   displayName: 'Enforce Service Mesh',
  //   name: 'enforce-service-mesh',
  //   description: 'Any Consul service must use the service mesh',
  //   policy: enforceMeshPolicy,
  // },
  // {
  //   displayName: 'Metadata Required',
  //   name: 'metadata-required',
  //   description: 'Requires that jobs define a certain metadata attribute',
  //   policy: metadataPolicy,
  // },
  // {
  //   displayName: 'Consul Service Discovery Only',
  //   name: 'consul-service-discovery-only',
  //   description:
  //     'Rejects jobs that use Nomad for service discovery instead of Consul',
  //   policy: consulSDPolicy,
  // },
  // {
  //   displayName: 'Vault Secrets Only',
  //   name: 'vault-secrets-only',
  //   description:
  //     'Rejects jobs that use Nomad for secret management instead of Vault',
  //   policy: vaultSecretsPolicy,
  // },
  // {
  //   displayName: 'Dynamic Ports Only',
  //   name: 'dynamic-ports-only',
  //   description: 'Fails any job that requests a static host port',
  //   policy: dyanmicPortsPolicy,
  // },
  {
    displayName: 'Resource Limits',
    name: 'resource-limits',
    description: 'Ensures that tasks do not request too much CPU or Memory',
    policy: resourceLimitsPolicy,
  },
  // {
  //   displayName: 'Constraint Enforcement',
  //   name: 'constraint-enforcement',
  //   description: 'Requires a constraint on client metadata',
  //   policy: constraintEnforcmentPolicy,
  // },
  {
    displayName: 'Restrict Images',
    name: 'restrict-images',
    description: 'Allows only certain Docker images and disables "latest" tags',
    policy: restictImagesPolicy,
  },
];
