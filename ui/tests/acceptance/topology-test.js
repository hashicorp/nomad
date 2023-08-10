/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
import { get } from '@ember/object';
import { currentURL, typeIn, click } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Topology from 'nomad-ui/tests/pages/topology';
import {
  formatBytes,
  formatScheduledBytes,
  formatHertz,
  formatScheduledHertz,
} from 'nomad-ui/utils/units';
import queryString from 'query-string';
import percySnapshot from '@percy/ember';
import faker from 'nomad-ui/mirage/faker';

const sumResources = (list, dimension) =>
  list.reduce((agg, val) => agg + (get(val, dimension) || 0), 0);

module('Acceptance | topology', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    server.createList('node-pool', 5);
    server.create('job', { createAllocations: false });
  });

  test('it passes an accessibility audit', async function (assert) {
    assert.expect(1);

    server.createList('node', 3);
    server.createList('allocation', 5);

    await Topology.visit();
    await a11yAudit(assert);
  });

  test('by default the info panel shows cluster aggregate stats', async function (assert) {
    faker.seed(1);
    server.create('node-pool', { name: 'all' });
    server.createList('node', 3);
    server.createList('allocation', 5);

    await Topology.visit();

    await percySnapshot(assert);

    assert.equal(Topology.infoPanelTitle, 'Cluster Details');
    assert.notOk(Topology.filteredNodesWarning.isPresent);

    assert.equal(
      Topology.clusterInfoPanel.nodeCount,
      `${server.schema.nodes.all().length} Clients`
    );

    const allocs = server.schema.allocations.all().models;
    const scheduledAllocs = allocs.filter((alloc) =>
      ['pending', 'running'].includes(alloc.clientStatus)
    );
    assert.equal(
      Topology.clusterInfoPanel.allocCount,
      `${scheduledAllocs.length} Allocations`
    );

    // Node pool count ignores 'all'.
    const nodePools = server.schema.nodePools
      .all()
      .models.filter((p) => p.name !== 'all');
    assert.equal(
      Topology.clusterInfoPanel.nodePoolCount,
      `${nodePools.length} Node Pools`
    );

    const nodeResources = server.schema.nodes
      .all()
      .models.mapBy('nodeResources');
    const taskResources = scheduledAllocs
      .mapBy('taskResources.models')
      .flat()
      .mapBy('resources');

    const totalMem = sumResources(nodeResources, 'Memory.MemoryMB');
    const totalCPU = sumResources(nodeResources, 'Cpu.CpuShares');
    const reservedMem = sumResources(taskResources, 'Memory.MemoryMB');
    const reservedCPU = sumResources(taskResources, 'Cpu.CpuShares');

    assert.equal(
      Topology.clusterInfoPanel.memoryProgressValue,
      reservedMem / totalMem
    );
    assert.equal(
      Topology.clusterInfoPanel.cpuProgressValue,
      reservedCPU / totalCPU
    );

    assert.equal(
      Topology.clusterInfoPanel.memoryAbsoluteValue,
      `${formatBytes(reservedMem * 1024 * 1024)} / ${formatBytes(
        totalMem * 1024 * 1024
      )} reserved`
    );

    assert.equal(
      Topology.clusterInfoPanel.cpuAbsoluteValue,
      `${formatHertz(reservedCPU, 'MHz')} / ${formatHertz(
        totalCPU,
        'MHz'
      )} reserved`
    );
  });

  test('all allocations for all namespaces and all clients are queried on load', async function (assert) {
    server.createList('node', 3);
    server.createList('allocation', 5);

    await Topology.visit();
    const requests = this.server.pretender.handledRequests;
    assert.ok(requests.findBy('url', '/v1/nodes?resources=true'));

    const allocationsRequest = requests.find((req) =>
      req.url.startsWith('/v1/allocations')
    );
    assert.ok(allocationsRequest);

    const allocationRequestParams = queryString.parse(
      allocationsRequest.url.split('?')[1]
    );
    assert.deepEqual(allocationRequestParams, {
      namespace: '*',
      task_states: 'false',
      resources: 'true',
    });
  });

  test('when an allocation is selected, the info panel shows information on the allocation', async function (assert) {
    const nodes = server.createList('node', 5);
    const job = server.create('job', { createAllocations: false });
    const taskGroup = server.schema.find('taskGroup', job.taskGroupIds[0]).name;
    const allocs = server.createList('allocation', 5, {
      forceRunningClientStatus: true,
      jobId: job.id,
      taskGroup,
    });

    // Get the first alloc of the first node that has an alloc
    const sortedNodes = nodes.sortBy('datacenter');
    let node, alloc;
    for (let n of sortedNodes) {
      alloc = allocs.find((a) => a.nodeId === n.id);
      if (alloc) {
        node = n;
        break;
      }
    }

    const dcIndex = nodes
      .mapBy('datacenter')
      .uniq()
      .sort()
      .indexOf(node.datacenter);
    const nodeIndex = nodes
      .filterBy('datacenter', node.datacenter)
      .indexOf(node);

    const reset = async () => {
      await Topology.visit();
      await Topology.viz.datacenters[dcIndex].nodes[
        nodeIndex
      ].memoryRects[0].select();
    };

    await reset();
    assert.equal(Topology.infoPanelTitle, 'Allocation Details');

    assert.equal(Topology.allocInfoPanel.id, alloc.id.split('-')[0]);

    const uniqueClients = allocs.mapBy('nodeId').uniq();
    assert.equal(
      Topology.allocInfoPanel.siblingAllocs,
      `Sibling Allocations: ${allocs.length}`
    );
    assert.equal(
      Topology.allocInfoPanel.uniquePlacements,
      `Unique Client Placements: ${uniqueClients.length}`
    );

    assert.equal(Topology.allocInfoPanel.job, job.name);
    assert.ok(Topology.allocInfoPanel.taskGroup.endsWith(alloc.taskGroup));
    assert.equal(Topology.allocInfoPanel.client, node.id.split('-')[0]);

    await Topology.allocInfoPanel.visitAlloc();
    assert.equal(currentURL(), `/allocations/${alloc.id}`);

    await reset();

    await Topology.allocInfoPanel.visitJob();
    assert.equal(currentURL(), `/jobs/${job.id}@default`);

    await reset();

    await Topology.allocInfoPanel.visitClient();
    assert.equal(currentURL(), `/clients/${node.id}`);
  });

  test('changing which allocation is selected changes the metric charts', async function (assert) {
    server.create('node');
    const job1 = server.create('job', { createAllocations: false });
    const taskGroup1 = server.schema.find(
      'taskGroup',
      job1.taskGroupIds[0]
    ).name;
    server.create('allocation', {
      forceRunningClientStatus: true,
      jobId: job1.id,
      taskGroup1,
    });

    const job2 = server.create('job', { createAllocations: false });
    const taskGroup2 = server.schema.find(
      'taskGroup',
      job2.taskGroupIds[0]
    ).name;
    server.create('allocation', {
      forceRunningClientStatus: true,
      jobId: job2.id,
      taskGroup2,
    });

    await Topology.visit();
    await Topology.viz.datacenters[0].nodes[0].memoryRects[0].select();
    const firstAllocationTaskNames =
      Topology.allocInfoPanel.charts[0].areas.mapBy('taskName');

    await Topology.viz.datacenters[0].nodes[0].memoryRects[1].select();
    const secondAllocationTaskNames =
      Topology.allocInfoPanel.charts[0].areas.mapBy('taskName');

    assert.notDeepEqual(firstAllocationTaskNames, secondAllocationTaskNames);
  });

  test('when a node is selected, the info panel shows information on the node', async function (assert) {
    // A high node count is required for node selection
    const nodes = server.createList('node', 51);
    const node = nodes.sortBy('datacenter')[0];
    server.createList('allocation', 5, { forceRunningClientStatus: true });

    const allocs = server.schema.allocations.where({ nodeId: node.id }).models;

    await Topology.visit();

    await Topology.viz.datacenters[0].nodes[0].selectNode();
    assert.equal(Topology.infoPanelTitle, 'Client Details');

    assert.equal(Topology.nodeInfoPanel.id, node.id.split('-')[0]);
    assert.equal(Topology.nodeInfoPanel.name, `Name: ${node.name}`);
    assert.equal(Topology.nodeInfoPanel.address, `Address: ${node.httpAddr}`);
    assert.equal(Topology.nodeInfoPanel.status, `Status: ${node.status}`);

    assert.equal(
      Topology.nodeInfoPanel.drainingLabel,
      node.drain ? 'Yes' : 'No'
    );
    assert.equal(
      Topology.nodeInfoPanel.eligibleLabel,
      node.schedulingEligibility === 'eligible' ? 'Yes' : 'No'
    );

    assert.equal(Topology.nodeInfoPanel.drainingIsAccented, node.drain);
    assert.equal(
      Topology.nodeInfoPanel.eligibleIsAccented,
      node.schedulingEligibility !== 'eligible'
    );

    const taskResources = allocs
      .mapBy('taskResources.models')
      .flat()
      .mapBy('resources');
    const reservedMem = sumResources(taskResources, 'Memory.MemoryMB');
    const reservedCPU = sumResources(taskResources, 'Cpu.CpuShares');

    const totalMem = node.nodeResources.Memory.MemoryMB;
    const totalCPU = node.nodeResources.Cpu.CpuShares;

    assert.equal(
      Topology.nodeInfoPanel.memoryProgressValue,
      reservedMem / totalMem
    );
    assert.equal(
      Topology.nodeInfoPanel.cpuProgressValue,
      reservedCPU / totalCPU
    );

    assert.equal(
      Topology.nodeInfoPanel.memoryAbsoluteValue,
      `${formatScheduledBytes(
        reservedMem * 1024 * 1024
      )} / ${formatScheduledBytes(totalMem, 'MiB')} reserved`
    );

    assert.equal(
      Topology.nodeInfoPanel.cpuAbsoluteValue,
      `${formatScheduledHertz(reservedCPU, 'MHz')} / ${formatScheduledHertz(
        totalCPU,
        'MHz'
      )} reserved`
    );

    await Topology.nodeInfoPanel.visitNode();
    assert.equal(currentURL(), `/clients/${node.id}`);
  });

  test('when one or more nodes lack the NodeResources property, a warning message is shown', async function (assert) {
    server.createList('node', 3);
    server.createList('allocation', 5);

    server.schema.nodes.all().models[0].update({ nodeResources: null });

    await Topology.visit();
    assert.ok(Topology.filteredNodesWarning.isPresent);
    assert.ok(Topology.filteredNodesWarning.message.startsWith('1'));
  });

  test('Filtering and Querying reduces the number of nodes shown', async function (assert) {
    server.createList('node', 10);
    server.createList('node', 2, {
      nodeClass: 'foo-bar-baz',
    });

    // Make sure we have at least one node draining and one ineligible.
    server.create('node', {
      schedulingEligibility: 'ineligible',
    });
    server.create('node', 'draining');

    // Create node pool exclusive for these nodes.
    server.create('node-pool', { name: 'test-node-pool' });
    server.createList('node', 3, {
      nodePool: 'test-node-pool',
    });

    server.createList('allocation', 5);

    // Count draining and ineligible nodes.
    const counts = {
      ineligible: 0,
      draining: 0,
    };
    server.db.nodes.forEach((n) => {
      if (n.schedulingEligibility === 'ineligible') {
        counts['ineligible'] += 1;
      }
      if (n.drain) {
        counts['draining'] += 1;
      }
    });

    await Topology.visit();
    assert.dom('[data-test-topo-viz-node]').exists({ count: 17 });

    // Test search.
    await typeIn('input.node-search', server.schema.nodes.first().name);
    assert.dom('[data-test-topo-viz-node]').exists({ count: 1 });
    await typeIn('input.node-search', server.schema.nodes.first().name);
    assert.dom('[data-test-topo-viz-node]').doesNotExist();
    await click('[title="Clear search"]');
    assert.dom('[data-test-topo-viz-node]').exists({ count: 17 });

    // Test node class filter.
    await Topology.facets.class.toggle();
    await Topology.facets.class.options
      .findOneBy('label', 'foo-bar-baz')
      .toggle();
    assert.dom('[data-test-topo-viz-node]').exists({ count: 2 });
    await Topology.facets.class.options
      .findOneBy('label', 'foo-bar-baz')
      .toggle();

    // Test ineligible state filter.
    await Topology.facets.state.toggle();
    await Topology.facets.state.options
      .findOneBy('label', 'Ineligible')
      .toggle();
    assert
      .dom('[data-test-topo-viz-node]')
      .exists({ count: counts['ineligible'] });
    await Topology.facets.state.options
      .findOneBy('label', 'Ineligible')
      .toggle();
    await Topology.facets.state.toggle();

    // Test draining state filter.
    await Topology.facets.state.toggle();
    await Topology.facets.state.options.findOneBy('label', 'Draining').toggle();
    assert
      .dom('[data-test-topo-viz-node]')
      .exists({ count: counts['draining'] });
    await Topology.facets.state.options.findOneBy('label', 'Draining').toggle();
    await Topology.facets.state.toggle();

    // Test node pool filter.
    await Topology.facets.nodePool.toggle();
    await Topology.facets.nodePool.options
      .findOneBy('label', 'test-node-pool')
      .toggle();
    assert.dom('[data-test-topo-viz-node]').exists({ count: 3 });
    await Topology.facets.nodePool.options
      .findOneBy('label', 'test-node-pool')
      .toggle();
  });
});
