/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  clickable,
  collection,
  create,
  hasClass,
  text,
  visitable,
} from 'ember-cli-page-object';

import { multiFacet } from 'nomad-ui/tests/pages/components/facet';
import TopoViz from 'nomad-ui/tests/pages/components/topo-viz';
import notification from 'nomad-ui/tests/pages/components/notification';

export default create({
  visit: visitable('/topology'),

  infoPanelTitle: text('[data-test-info-panel-title]'),
  filteredNodesWarning: notification('[data-test-filtered-nodes-warning]'),

  viz: TopoViz('[data-test-topo-viz]'),

  facets: {
    nodePool: multiFacet('[data-test-node-pool-facet]'),
    datacenter: multiFacet('[data-test-datacenter-facet]'),
    class: multiFacet('[data-test-class-facet]'),
    state: multiFacet('[data-test-state-facet]'),
    version: multiFacet('[data-test-version-facet]'),
  },

  clusterInfoPanel: {
    scope: '[data-test-info-panel]',
    nodeCount: text('[data-test-node-count]'),
    allocCount: text('[data-test-alloc-count]'),
    nodePoolCount: text('[data-test-node-pool-count]'),

    memoryProgressValue: attribute('value', '[data-test-memory-progress-bar]'),
    memoryAbsoluteValue: text('[data-test-memory-absolute-value]'),
    cpuProgressValue: attribute('value', '[data-test-cpu-progress-bar]'),
    cpuAbsoluteValue: text('[data-test-cpu-absolute-value]'),
  },

  nodeInfoPanel: {
    scope: '[data-test-info-panel]',
    allocations: text('[data-test-allocaions]'),

    visitNode: clickable('[data-test-client-link]'),

    id: text('[data-test-client-link]'),
    name: text('[data-test-name]'),
    address: text('[data-test-address]'),
    status: text('[data-test-status]'),

    drainingLabel: text('[data-test-draining]'),
    drainingIsAccented: hasClass('is-info', '[data-test-draining]'),

    eligibleLabel: text('[data-test-eligible]'),
    eligibleIsAccented: hasClass('is-warning', '[data-test-eligible]'),

    memoryProgressValue: attribute('value', '[data-test-memory-progress-bar]'),
    memoryAbsoluteValue: text('[data-test-memory-absolute-value]'),
    cpuProgressValue: attribute('value', '[data-test-cpu-progress-bar]'),
    cpuAbsoluteValue: text('[data-test-cpu-absolute-value]'),
  },

  allocInfoPanel: {
    scope: '[data-test-info-panel]',
    id: text('[data-test-id]'),
    visitAlloc: clickable('[data-test-id]'),

    siblingAllocs: text('[data-test-sibling-allocs]'),
    uniquePlacements: text('[data-test-unique-placements]'),

    job: text('[data-test-job]'),
    visitJob: clickable('[data-test-job]'),
    taskGroup: text('[data-test-task-group]'),

    client: text('[data-test-client]'),
    visitClient: clickable('[data-test-client]'),

    charts: collection('[data-test-primary-metric]', {
      areas: collection('[data-test-chart-area]', {
        taskName: attribute('data-test-task-name'),
      }),
    }),
  },
});
