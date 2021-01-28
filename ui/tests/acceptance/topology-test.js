import { get } from '@ember/object';
import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Topology from 'nomad-ui/tests/pages/topology';
import { reduceToLargestUnit } from 'nomad-ui/helpers/format-bytes';
import queryString from 'query-string';

const sumResources = (list, dimension) =>
  list.reduce((agg, val) => agg + (get(val, dimension) || 0), 0);

module('Acceptance | topology', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    server.create('job', { createAllocations: false });
  });

  test('it passes an accessibility audit', async function(assert) {
    server.createList('node', 3);
    server.createList('allocation', 5);

    await Topology.visit();
    await a11yAudit(assert);
  });

  test('by default the info panel shows cluster aggregate stats', async function(assert) {
    server.createList('node', 3);
    server.createList('allocation', 5);

    await Topology.visit();
    assert.equal(Topology.infoPanelTitle, 'Cluster Details');
    assert.notOk(Topology.filteredNodesWarning.isPresent);

    assert.equal(
      Topology.clusterInfoPanel.nodeCount,
      `${server.schema.nodes.all().length} Clients`
    );

    const allocs = server.schema.allocations.all().models;
    const scheduledAllocs = allocs.filter(alloc =>
      ['pending', 'running'].includes(alloc.clientStatus)
    );
    assert.equal(Topology.clusterInfoPanel.allocCount, `${scheduledAllocs.length} Allocations`);

    const nodeResources = server.schema.nodes.all().models.mapBy('nodeResources');
    const taskResources = scheduledAllocs
      .mapBy('taskResources.models')
      .flat()
      .mapBy('resources');

    const totalMem = sumResources(nodeResources, 'Memory.MemoryMB');
    const totalCPU = sumResources(nodeResources, 'Cpu.CpuShares');
    const reservedMem = sumResources(taskResources, 'Memory.MemoryMB');
    const reservedCPU = sumResources(taskResources, 'Cpu.CpuShares');

    assert.equal(Topology.clusterInfoPanel.memoryProgressValue, reservedMem / totalMem);
    assert.equal(Topology.clusterInfoPanel.cpuProgressValue, reservedCPU / totalCPU);

    const [rNum, rUnit] = reduceToLargestUnit(reservedMem * 1024 * 1024);
    const [tNum, tUnit] = reduceToLargestUnit(totalMem * 1024 * 1024);

    assert.equal(
      Topology.clusterInfoPanel.memoryAbsoluteValue,
      `${Math.floor(rNum)} ${rUnit} / ${Math.floor(tNum)} ${tUnit} reserved`
    );

    assert.equal(
      Topology.clusterInfoPanel.cpuAbsoluteValue,
      `${reservedCPU} MHz / ${totalCPU} MHz reserved`
    );
  });

  test('all allocations for all namespaces and all clients are queried on load', async function(assert) {
    server.createList('node', 3);
    server.createList('allocation', 5);

    await Topology.visit();
    const requests = this.server.pretender.handledRequests;
    assert.ok(requests.findBy('url', '/v1/nodes?resources=true'));

    const allocationsRequest = requests.find(req => req.url.startsWith('/v1/allocations'));
    assert.ok(allocationsRequest);

    const allocationRequestParams = queryString.parse(allocationsRequest.url.split('?')[1]);
    assert.deepEqual(allocationRequestParams, {
      namespace: '*',
      task_states: 'false',
      resources: 'true',
    });
  });

  test('when an allocation is selected, the info panel shows information on the allocation', async function(assert) {
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
      alloc = allocs.find(a => a.nodeId === n.id);
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
    const nodeIndex = nodes.filterBy('datacenter', node.datacenter).indexOf(node);

    const reset = async () => {
      await Topology.visit();
      await Topology.viz.datacenters[dcIndex].nodes[nodeIndex].memoryRects[0].select();
    };

    await reset();
    assert.equal(Topology.infoPanelTitle, 'Allocation Details');

    assert.equal(Topology.allocInfoPanel.id, alloc.id.split('-')[0]);

    const uniqueClients = allocs.mapBy('nodeId').uniq();
    assert.equal(Topology.allocInfoPanel.siblingAllocs, `Sibling Allocations: ${allocs.length}`);
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
    assert.equal(currentURL(), `/jobs/${job.id}`);

    await reset();

    await Topology.allocInfoPanel.visitClient();
    assert.equal(currentURL(), `/clients/${node.id}`);
  });

  test('when a node is selected, the info panel shows information on the node', async function(assert) {
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

    assert.equal(Topology.nodeInfoPanel.drainingLabel, node.drain ? 'Yes' : 'No');
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

    assert.equal(Topology.nodeInfoPanel.memoryProgressValue, reservedMem / totalMem);
    assert.equal(Topology.nodeInfoPanel.cpuProgressValue, reservedCPU / totalCPU);

    const [rNum, rUnit] = reduceToLargestUnit(reservedMem * 1024 * 1024);
    const [tNum, tUnit] = reduceToLargestUnit(totalMem * 1024 * 1024);

    assert.equal(
      Topology.nodeInfoPanel.memoryAbsoluteValue,
      `${Math.floor(rNum)} ${rUnit} / ${Math.floor(tNum)} ${tUnit} reserved`
    );

    assert.equal(
      Topology.nodeInfoPanel.cpuAbsoluteValue,
      `${reservedCPU} MHz / ${totalCPU} MHz reserved`
    );

    await Topology.nodeInfoPanel.visitNode();
    assert.equal(currentURL(), `/clients/${node.id}`);
  });

  test('when one or more nodes lack the NodeResources property, a warning message is shown', async function(assert) {
    server.createList('node', 3);
    server.createList('allocation', 5);

    server.schema.nodes.all().models[0].update({ nodeResources: null });

    await Topology.visit();
    assert.ok(Topology.filteredNodesWarning.isPresent);
    assert.ok(Topology.filteredNodesWarning.message.startsWith('1'));
  });
});
