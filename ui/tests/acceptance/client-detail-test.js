import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

let node;

moduleForAcceptance('Acceptance | client detail', {
  beforeEach() {
    server.create('node');
    node = server.db.nodes[0];

    // Related models
    server.create('agent');
    server.create('job');
    server.createList('allocation', 3, { nodeId: node.id });

    visit(`/nodes/${node.id}`);
  },
});

test('/nodes/:id should have a breadrcumb trail linking back to nodes', function(assert) {
  assert.equal(find('.breadcrumb:eq(0)').text(), 'Nodes', 'First breadcrumb says nodes');
  assert.equal(
    find('.breadcrumb:eq(1)').text(),
    node.id.split('-')[0],
    'Second breadcrumb says the node short id'
  );

  click('.breadcrumb:eq(0)');
  andThen(() => {
    assert.equal(currentURL(), '/nodes', 'First breadcrumb links back to nodes');
  });
});

test('/nodes/:id should list all allocations on the node', function(assert) {
  const allocationsCount = server.db.allocations.where({ nodeId: node.id }).length;
  assert.equal(
    find('.allocations tbody tr').length,
    allocationsCount,
    `Allocations table lists all ${allocationsCount} associated allocations`
  );
});

test('each allocation should have high-level details for the allocation', function(assert) {
  const allocationRow = find('.allocations tbody tr:eq(0)');
  const allocation = server.db.allocations
    .where({ nodeId: node.id })
    .sortBy('modifyIndex')
    .reverse()[0];

  assert.equal(
    allocationRow.find('td:eq(0)').text().trim(),
    allocation.id.split('-')[0],
    'Allocation short ID'
  );
  assert.equal(allocationRow.find('td:eq(1)').text().trim(), allocation.name, 'Allocation name');
  assert.ok(
    allocationRow.find('td:eq(2)').text().includes(server.db.jobs.find(allocation.jobId).name),
    'Job name'
  );
  assert.ok(
    allocationRow.find('td:eq(2) .is-faded').text().includes(allocation.taskGroup),
    'Task group name'
  );
  assert.equal(
    allocationRow.find('td:eq(3)').text().trim(),
    server.db.clientAllocationStats.find(allocation.id).resourceUsage.CpuStats.Percent,
    'CPU %'
  );
});

test('each allocation should link to the allocation detail page', function(assert) {
  const allocation = server.db.allocations
    .where({ nodeId: node.id })
    .sortBy('modifyIndex')
    .reverse()[0];

  click('.allocations tbody tr:eq(0) td:eq(0) a');

  andThen(() => {
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}`,
      'Allocation rows link to allocation detail pages'
    );
  });
});

test('each allocation should link to the job the allocation belongs to', function(assert) {
  const allocation = server.db.allocations.where({ nodeId: node.id })[0];
  const job = server.db.jobs.find(allocation.jobId);
  click('.allocations tbody tr:eq(0) td:eq(2) a');

  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${job.id}`,
      'Allocation rows link to the job detail page for the allocation'
    );
  });
});

test('/nodes/:id should list all attributes for the node', function(assert) {
  assert.ok(find('.attributes-table'), 'Attributes table is on the page');
});
