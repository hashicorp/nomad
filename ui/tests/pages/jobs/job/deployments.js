/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  clickable,
  create,
  collection,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

import allocations from 'nomad-ui/tests/pages/components/allocations';
import error from 'nomad-ui/tests/pages/components/error';

export default create({
  visit: visitable('/jobs/:id/deployments'),

  deployments: collection('[data-test-deployment]', {
    text: text(),
    status: text('[data-test-deployment-status]'),
    statusClass: attribute('class', '[data-test-deployment-status]'),
    version: text('[data-test-deployment-version]'),
    submitTime: text('[data-test-deployment-submit-time]'),
    promotionIsRequired: isPresent('[data-test-promotion-required]'),

    toggle: clickable('[data-test-deployment-toggle-details]'),

    hasDetails: isPresent('[data-test-deployment-details]'),

    metrics: collection('[data-test-deployment-metric]', {
      id: attribute('data-test-deployment-metric'),
      text: text(),
    }),

    metricFor(id) {
      return this.metrics.toArray().findBy('id', id);
    },

    notification: text('[data-test-deployment-notification]'),

    hasTaskGroups: isPresent('[data-test-deployment-task-groups]'),
    taskGroups: collection('[data-test-deployment-task-group]', {
      name: text('[data-test-deployment-task-group-name]'),
      promotion: text('[data-test-deployment-task-group-promotion]'),
      autoRevert: text('[data-test-deployment-task-group-auto-revert]'),
      canaries: text('[data-test-deployment-task-group-canaries]'),
      allocs: text('[data-test-deployment-task-group-allocs]'),
      healthy: text('[data-test-deployment-task-group-healthy]'),
      unhealthy: text('[data-test-deployment-task-group-unhealthy]'),
      progress: text('[data-test-deployment-task-group-progress-deadline]'),
    }),

    ...allocations('[data-test-deployment-allocation]'),
    hasAllocations: isPresent('[data-test-deployment-allocations]'),
  }),

  error: error(),
});
