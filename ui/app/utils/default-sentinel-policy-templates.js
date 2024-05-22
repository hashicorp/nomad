/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import countLimitsPolicy from './sentinel_policy_templates/count-limits';
import noFridaysPolicy from './sentinel_policy_templates/no-friday-deploys';
import alwaysFailPolicy from './sentinel_policy_templates/always-fail';
import canariesOnlyPolicy from './sentinel_policy_templates/canaries-only';
import resourceLimitsPolicy from './sentinel_policy_templates/resource-limits';
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
  {
    displayName: 'Canaries Only',
    name: 'canaries-only',
    description: 'All deployments must have a canary',
    policy: canariesOnlyPolicy,
  },
  {
    displayName: 'Resource Limits',
    name: 'resource-limits',
    description: 'Ensures that tasks do not request too much CPU or Memory',
    policy: resourceLimitsPolicy,
  },
  {
    displayName: 'Restrict Images',
    name: 'restrict-images',
    description: 'Allows only certain Docker images and disables "latest" tags',
    policy: restictImagesPolicy,
  },
];
