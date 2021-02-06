import { attribute, clickable, create, hasClass, text, visitable } from 'ember-cli-page-object';

import TopoViz from 'nomad-ui/tests/pages/components/topo-viz';
import notification from 'nomad-ui/tests/pages/components/notification';

export default create({
  visit: visitable('/topology'),

  infoPanelTitle: text('[data-test-info-panel-title]'),
  filteredNodesWarning: notification('[data-test-filtered-nodes-warning]'),

  viz: TopoViz('[data-test-topo-viz]'),

  clusterInfoPanel: {
    scope: '[data-test-info-panel]',
    nodeCount: text('[data-test-node-count]'),
    allocCount: text('[data-test-alloc-count]'),

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
  },
});
