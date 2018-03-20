import $ from 'jquery';
import { click, find, findAll, currentURL, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import { formatBytes } from 'nomad-ui/helpers/format-bytes';
import moment from 'moment';

let node;

moduleForAcceptance('Acceptance | client detail', {
  beforeEach() {
    server.create('node', 'forceIPv4');
    node = server.db.nodes[0];

    // Related models
    server.create('agent');
    server.create('job', { createAllocations: false });
    server.createList('allocation', 3, { nodeId: node.id });
  },
});

test('/clients/:id should have a breadcrumb trail linking back to clients', function(assert) {
  visit(`/clients/${node.id}`);

  andThen(() => {
    assert.equal(
      find('[data-test-breadcrumb="clients"]').textContent,
      'Clients',
      'First breadcrumb says clients'
    );
    assert.equal(
      find('[data-test-breadcrumb="client"]').textContent,
      node.id.split('-')[0],
      'Second breadcrumb says the node short id'
    );
  });

  andThen(() => {
    click(find('[data-test-breadcrumb="clients"]'));
  });

  andThen(() => {
    assert.equal(currentURL(), '/clients', 'First breadcrumb links back to clients');
  });
});

test('/clients/:id should list immediate details for the node in the title', function(assert) {
  visit(`/clients/${node.id}`);

  andThen(() => {
    assert.ok(find('[data-test-title]').textContent.includes(node.name), 'Title includes name');
    assert.ok(find('[data-test-title]').textContent.includes(node.id), 'Title includes id');
    assert.ok(find(`[data-test-node-status="${node.status}"]`), 'Title includes status light');
  });
});

test('/clients/:id should list additional detail for the node below the title', function(assert) {
  visit(`/clients/${node.id}`);

  andThen(() => {
    assert.equal(
      findAll('.inline-definitions .pair')[0].textContent,
      `Status ${node.status}`,
      'Status is in additional details'
    );
    assert.ok(
      $('[data-test-status-definition] .status-text').hasClass(`node-${node.status}`),
      'Status is decorated with a status class'
    );
    assert.equal(
      find('[data-test-address-definition]').textContent,
      `Address ${node.httpAddr}`,
      'Address is in additional details'
    );
    assert.equal(
      find('[data-test-datacenter-definition]').textContent,
      `Datacenter ${node.datacenter}`,
      'Datacenter is in additional details'
    );
  });
});

test('/clients/:id should list all allocations on the node', function(assert) {
  const allocationsCount = server.db.allocations.where({ nodeId: node.id }).length;

  visit(`/clients/${node.id}`);

  andThen(() => {
    assert.equal(
      findAll('[data-test-allocation]').length,
      allocationsCount,
      `Allocations table lists all ${allocationsCount} associated allocations`
    );
  });
});

test('each allocation should have high-level details for the allocation', function(assert) {
  const allocation = server.db.allocations
    .where({ nodeId: node.id })
    .sortBy('modifyIndex')
    .reverse()[0];

  const allocStats = server.db.clientAllocationStats.find(allocation.id);
  const taskGroup = server.db.taskGroups.findBy({
    name: allocation.taskGroup,
    jobId: allocation.jobId,
  });

  const tasks = taskGroup.taskIds.map(id => server.db.tasks.find(id));
  const cpuUsed = tasks.reduce((sum, task) => sum + task.Resources.CPU, 0);
  const memoryUsed = tasks.reduce((sum, task) => sum + task.Resources.MemoryMB, 0);

  visit(`/clients/${node.id}`);

  andThen(() => {
    const allocationRow = $(find('[data-test-allocation]'));
    assert.equal(
      allocationRow
        .find('[data-test-short-id]')
        .text()
        .trim(),
      allocation.id.split('-')[0],
      'Allocation short ID'
    );
    assert.equal(
      allocationRow
        .find('[data-test-modify-time]')
        .text()
        .trim(),
      moment(allocation.modifyTime / 1000000).format('MM/DD HH:mm:ss'),
      'Allocation modify time'
    );
    assert.equal(
      allocationRow
        .find('[data-test-name]')
        .text()
        .trim(),
      allocation.name,
      'Allocation name'
    );
    assert.equal(
      allocationRow
        .find('[data-test-client-status]')
        .text()
        .trim(),
      allocation.clientStatus,
      'Client status'
    );
    assert.equal(
      allocationRow
        .find('[data-test-job]')
        .text()
        .trim(),
      server.db.jobs.find(allocation.jobId).name,
      'Job name'
    );
    assert.ok(
      allocationRow
        .find('[data-test-task-group]')
        .text()
        .includes(allocation.taskGroup),
      'Task group name'
    );
    assert.ok(
      allocationRow
        .find('[data-test-job-version]')
        .text()
        .includes(allocation.jobVersion),
      'Job Version'
    );
    assert.equal(
      allocationRow
        .find('[data-test-cpu]')
        .text()
        .trim(),
      Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks) / cpuUsed,
      'CPU %'
    );
    assert.equal(
      allocationRow.find('[data-test-cpu] .tooltip').attr('aria-label'),
      `${Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks)} / ${cpuUsed} MHz`,
      'Detailed CPU information is in a tooltip'
    );
    assert.equal(
      allocationRow
        .find('[data-test-mem]')
        .text()
        .trim(),
      allocStats.resourceUsage.MemoryStats.RSS / 1024 / 1024 / memoryUsed,
      'Memory used'
    );
    assert.equal(
      allocationRow.find('[data-test-mem] .tooltip').attr('aria-label'),
      `${formatBytes([allocStats.resourceUsage.MemoryStats.RSS])} / ${memoryUsed} MiB`,
      'Detailed memory information is in a tooltip'
    );
  });
});

test('each allocation should show job information even if the job is incomplete and already in the store', function(assert) {
  // First, visit clients to load the allocations for each visible node.
  // Don't load the job belongsTo of the allocation! Leave it unfulfilled.

  visit('/clients');

  // Then, visit jobs to load all jobs, which should implicitly fulfill
  // the job belongsTo of each allocation pointed at each job.

  visit('/jobs');

  // Finally, visit a node to assert that the job name and task group name are
  // present. This will require reloading the job, since task groups aren't a
  // part of the jobs list response.

  visit(`/clients/${node.id}`);

  andThen(() => {
    const allocationRow = $(find('[data-test-allocation]'));
    const allocation = server.db.allocations
      .where({ nodeId: node.id })
      .sortBy('modifyIndex')
      .reverse()[0];

    assert.ok(
      allocationRow
        .find('[data-test-job]')
        .text()
        .includes(server.db.jobs.find(allocation.jobId).name),
      'Job name'
    );
    assert.ok(
      allocationRow
        .find('[data-test-task-group]')
        .text()
        .includes(allocation.taskGroup),
      'Task group name'
    );
  });
});

test('each allocation should link to the allocation detail page', function(assert) {
  const allocation = server.db.allocations
    .where({ nodeId: node.id })
    .sortBy('modifyIndex')
    .reverse()[0];

  visit(`/clients/${node.id}`);

  andThen(() => {
    click('[data-test-short-id] a');
  });

  andThen(() => {
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}`,
      'Allocation rows link to allocation detail pages'
    );
  });
});

test('each allocation should link to the job the allocation belongs to', function(assert) {
  visit(`/clients/${node.id}`);

  const allocation = server.db.allocations.where({ nodeId: node.id })[0];
  const job = server.db.jobs.find(allocation.jobId);

  andThen(() => {
    click('[data-test-job]');
  });

  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${job.id}`,
      'Allocation rows link to the job detail page for the allocation'
    );
  });
});

test('/clients/:id should list all attributes for the node', function(assert) {
  visit(`/clients/${node.id}`);

  andThen(() => {
    assert.ok(find('[data-test-attributes]'), 'Attributes table is on the page');
  });
});

test('/clients/:id lists all meta attributes', function(assert) {
  node = server.create('node', 'forceIPv4', 'withMeta');

  visit(`/clients/${node.id}`);

  andThen(() => {
    assert.ok(find('[data-test-meta]'), 'Meta attributes table is on the page');
    assert.notOk(find('[data-test-empty-meta-message]'), 'Meta attributes is not empty');

    const firstMetaKey = Object.keys(node.meta)[0];
    assert.equal(
      find('[data-test-meta] [data-test-key]').textContent.trim(),
      firstMetaKey,
      'Meta attributes for the node are bound to the attributes table'
    );
    assert.equal(
      find('[data-test-meta] [data-test-value]').textContent.trim(),
      node.meta[firstMetaKey],
      'Meta attributes for the node are bound to the attributes table'
    );
  });
});

test('/clients/:id shows an empty message when there is no meta data', function(assert) {
  visit(`/clients/${node.id}`);

  andThen(() => {
    assert.notOk(find('[data-test-meta]'), 'Meta attributes table is not on the page');
    assert.ok(find('[data-test-empty-meta-message]'), 'Meta attributes is empty');
  });
});

test('when the node is not found, an error message is shown, but the URL persists', function(assert) {
  visit('/clients/not-a-real-node');

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/node/not-a-real-node',
      'A request to the nonexistent node is made'
    );
    assert.equal(currentURL(), '/clients/not-a-real-node', 'The URL persists');
    assert.ok(find('[data-test-error]'), 'Error message is shown');
    assert.equal(
      find('[data-test-error-title]').textContent,
      'Not Found',
      'Error message is for 404'
    );
  });
});
