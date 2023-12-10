/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  clickable,
  create,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

import allocations from 'nomad-ui/tests/pages/components/allocations';

export default create({
  visit: visitable('/csi/plugins/:id'),

  title: text('[data-test-title]'),

  controllerHealthIsPresent: isPresent('[data-test-plugin-controller-health]'),
  controllerHealth: text('[data-test-plugin-controller-health]'),
  nodeHealth: text('[data-test-plugin-node-health]'),
  provider: text('[data-test-plugin-provider]'),

  controllerAvailabilityIsPresent: isPresent(
    '[data-test-plugin-controller-availability]'
  ),
  nodeAvailabilityIsPresent: isPresent('[data-test-plugin-node-availability]'),

  ...allocations('[data-test-controller-allocation]', 'controllerAllocations'),
  ...allocations('[data-test-node-allocation]', 'nodeAllocations'),

  goToControllerAllocations: clickable(
    '[data-test-go-to-controller-allocations]'
  ),
  goToNodeAllocations: clickable('[data-test-go-to-node-allocations]'),
  goToControllerAllocationsText: text(
    '[data-test-go-to-controller-allocations]'
  ),
  goToNodeAllocationsText: text('[data-test-go-to-node-allocations]'),

  controllerTableIsPresent: isPresent('[data-test-controller-allocations]'),

  controllerTableIsEmpty: isPresent('[data-test-empty-controller-allocations]'),
  controllerEmptyState: {
    headline: text('[data-test-empty-controller-allocations-headline]'),
  },

  nodeTableIsEmpty: isPresent('[data-test-empty-node-allocations]'),
  nodeEmptyState: {
    headline: text('[data-test-empty-node-allocations-headline]'),
  },
});
