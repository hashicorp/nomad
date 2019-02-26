import { assign } from '@ember/polyfills';
import { currentURL } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import { formatBytes } from 'nomad-ui/helpers/format-bytes';
import formatDuration from 'nomad-ui/utils/format-duration';
import moment from 'moment';
import ClientDetail from 'nomad-ui/tests/pages/clients/detail';
import Clients from 'nomad-ui/tests/pages/clients/list';
import Jobs from 'nomad-ui/tests/pages/jobs/list';

let node;

moduleForAcceptance('Acceptance | client detail', {
  beforeEach() {
    server.create('node', 'forceIPv4', { schedulingEligibility: 'eligible' });
    node = server.db.nodes[0];

    // Related models
    server.create('agent');
    server.create('job', { createAllocations: false });
    server.createList('allocation', 3, { nodeId: node.id, clientStatus: 'running' });
  },
});

test('/clients/:id should have a breadcrumb trail linking back to clients', function(assert) {
  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.equal(
      ClientDetail.breadcrumbFor('clients.index').text,
      'Clients',
      'First breadcrumb says clients'
    );
    assert.equal(
      ClientDetail.breadcrumbFor('clients.client').text,
      node.id.split('-')[0],
      'Second breadcrumb says the node short id'
    );
  });

  andThen(() => {
    ClientDetail.breadcrumbFor('clients.index').visit();
  });

  andThen(() => {
    assert.equal(currentURL(), '/clients', 'First breadcrumb links back to clients');
  });
});

test('/clients/:id should list immediate details for the node in the title', function(assert) {
  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.ok(ClientDetail.title.includes(node.name), 'Title includes name');
    assert.ok(ClientDetail.title.includes(node.id), 'Title includes id');
    assert.equal(
      ClientDetail.statusLight.objectAt(0).id,
      node.status,
      'Title includes status light'
    );
  });
});

test('/clients/:id should list additional detail for the node below the title', function(assert) {
  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.ok(
      ClientDetail.statusDefinition.includes(node.status),
      'Status is in additional details'
    );
    assert.ok(
      ClientDetail.statusDecorationClass.includes(`node-${node.status}`),
      'Status is decorated with a status class'
    );
    assert.ok(
      ClientDetail.addressDefinition.includes(node.httpAddr),
      'Address is in additional details'
    );
    assert.ok(
      ClientDetail.drainingDefinition.includes(node.drain + ''),
      'Drain status is in additional details'
    );
    assert.ok(
      ClientDetail.eligibilityDefinition.includes(node.schedulingEligibility),
      'Scheduling eligibility is in additional details'
    );
    assert.ok(
      ClientDetail.datacenterDefinition.includes(node.datacenter),
      'Datacenter is in additional details'
    );
  });
});

test('/clients/:id should include resource utilization graphs', function(assert) {
  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.equal(ClientDetail.resourceCharts.length, 2, 'Two resource utilization graphs');
    assert.equal(ClientDetail.resourceCharts.objectAt(0).name, 'CPU', 'First chart is CPU');
    assert.equal(ClientDetail.resourceCharts.objectAt(1).name, 'Memory', 'Second chart is Memory');
  });
});

test('/clients/:id should list all allocations on the node', function(assert) {
  const allocationsCount = server.db.allocations.where({ nodeId: node.id }).length;

  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.equal(
      ClientDetail.allocations.length,
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

  ClientDetail.visit({ id: node.id });

  andThen(() => {
    const allocationRow = ClientDetail.allocations.objectAt(0);

    assert.equal(allocationRow.shortId, allocation.id.split('-')[0], 'Allocation short ID');
    assert.equal(
      allocationRow.createTime,
      moment(allocation.createTime / 1000000).format('MMM DD HH:mm:ss ZZ'),
      'Allocation create time'
    );
    assert.equal(
      allocationRow.modifyTime,
      moment(allocation.modifyTime / 1000000).fromNow(),
      'Allocation modify time'
    );
    assert.equal(allocationRow.status, allocation.clientStatus, 'Client status');
    assert.equal(allocationRow.job, server.db.jobs.find(allocation.jobId).name, 'Job name');
    assert.ok(allocationRow.taskGroup, 'Task group name');
    assert.ok(allocationRow.jobVersion, 'Job Version');
    assert.equal(
      allocationRow.cpu,
      Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks) / cpuUsed,
      'CPU %'
    );
    assert.equal(
      allocationRow.cpuTooltip,
      `${Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks)} / ${cpuUsed} MHz`,
      'Detailed CPU information is in a tooltip'
    );
    assert.equal(
      allocationRow.mem,
      allocStats.resourceUsage.MemoryStats.RSS / 1024 / 1024 / memoryUsed,
      'Memory used'
    );
    assert.equal(
      allocationRow.memTooltip,
      `${formatBytes([allocStats.resourceUsage.MemoryStats.RSS])} / ${memoryUsed} MiB`,
      'Detailed memory information is in a tooltip'
    );
  });
});

test('each allocation should show job information even if the job is incomplete and already in the store', function(assert) {
  // First, visit clients to load the allocations for each visible node.
  // Don't load the job belongsTo of the allocation! Leave it unfulfilled.

  Clients.visit();

  // Then, visit jobs to load all jobs, which should implicitly fulfill
  // the job belongsTo of each allocation pointed at each job.

  Jobs.visit();

  // Finally, visit a node to assert that the job name and task group name are
  // present. This will require reloading the job, since task groups aren't a
  // part of the jobs list response.

  ClientDetail.visit({ id: node.id });

  andThen(() => {
    const allocationRow = ClientDetail.allocations.objectAt(0);
    const allocation = server.db.allocations
      .where({ nodeId: node.id })
      .sortBy('modifyIndex')
      .reverse()[0];

    assert.equal(allocationRow.job, server.db.jobs.find(allocation.jobId).name, 'Job name');
    assert.ok(allocationRow.taskGroup.includes(allocation.taskGroup), 'Task group name');
  });
});

test('each allocation should link to the allocation detail page', function(assert) {
  const allocation = server.db.allocations
    .where({ nodeId: node.id })
    .sortBy('modifyIndex')
    .reverse()[0];

  ClientDetail.visit({ id: node.id });

  andThen(() => {
    ClientDetail.allocations.objectAt(0).visit();
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
  ClientDetail.visit({ id: node.id });

  const allocation = server.db.allocations.where({ nodeId: node.id })[0];
  const job = server.db.jobs.find(allocation.jobId);

  andThen(() => {
    ClientDetail.allocations.objectAt(0).visitJob();
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
  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.ok(ClientDetail.attributesTable, 'Attributes table is on the page');
  });
});

test('/clients/:id lists all meta attributes', function(assert) {
  node = server.create('node', 'forceIPv4', 'withMeta');

  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.ok(ClientDetail.metaTable, 'Meta attributes table is on the page');
    assert.notOk(ClientDetail.emptyMetaMessage, 'Meta attributes is not empty');

    const firstMetaKey = Object.keys(node.meta)[0];
    const firstMetaAttribute = ClientDetail.metaAttributes.objectAt(0);
    assert.equal(
      firstMetaAttribute.key,
      firstMetaKey,
      'Meta attributes for the node are bound to the attributes table'
    );
    assert.equal(
      firstMetaAttribute.value,
      node.meta[firstMetaKey],
      'Meta attributes for the node are bound to the attributes table'
    );
  });
});

test('/clients/:id shows an empty message when there is no meta data', function(assert) {
  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.notOk(ClientDetail.metaTable, 'Meta attributes table is not on the page');
    assert.ok(ClientDetail.emptyMetaMessage, 'Meta attributes is empty');
  });
});

test('when the node is not found, an error message is shown, but the URL persists', function(assert) {
  ClientDetail.visit({ id: 'not-a-real-node' });

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/node/not-a-real-node',
      'A request to the nonexistent node is made'
    );
    assert.equal(currentURL(), '/clients/not-a-real-node', 'The URL persists');
    assert.ok(ClientDetail.error.isShown, 'Error message is shown');
    assert.equal(ClientDetail.error.title, 'Not Found', 'Error message is for 404');
  });
});

test('/clients/:id shows the recent events list', function(assert) {
  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.ok(ClientDetail.hasEvents, 'Client events section exists');
  });
});

test('each node event shows basic node event information', function(assert) {
  const event = server.db.nodeEvents
    .where({ nodeId: node.id })
    .sortBy('time')
    .reverse()[0];

  ClientDetail.visit({ id: node.id });

  andThen(() => {
    const eventRow = ClientDetail.events.objectAt(0);
    assert.equal(
      eventRow.time,
      moment(event.time).format("MMM DD, 'YY HH:mm:ss ZZ"),
      'Event timestamp'
    );
    assert.equal(eventRow.subsystem, event.subsystem, 'Event subsystem');
    assert.equal(eventRow.message, event.message, 'Event message');
  });
});

test('/clients/:id shows the driver status of every driver for the node', function(assert) {
  // Set the drivers up so health and detection is well tested
  const nodeDrivers = node.drivers;
  const undetectedDriver = 'raw_exec';

  Object.values(nodeDrivers).forEach(driver => {
    driver.Detected = true;
  });

  nodeDrivers[undetectedDriver].Detected = false;
  node.drivers = nodeDrivers;

  const drivers = Object.keys(node.drivers)
    .map(driverName => assign({ Name: driverName }, node.drivers[driverName]))
    .sortBy('Name');

  assert.ok(drivers.length > 0, 'Node has drivers');

  ClientDetail.visit({ id: node.id });

  andThen(() => {
    drivers.forEach((driver, index) => {
      const driverHead = ClientDetail.driverHeads.objectAt(index);

      assert.equal(driverHead.name, driver.Name, `${driver.Name}: Name is correct`);
      assert.equal(
        driverHead.detected,
        driver.Detected ? 'Yes' : 'No',
        `${driver.Name}: Detection is correct`
      );
      assert.equal(
        driverHead.lastUpdated,
        moment(driver.UpdateTime).fromNow(),
        `${driver.Name}: Last updated shows time since now`
      );

      if (driver.Name === undetectedDriver) {
        assert.notOk(
          driverHead.healthIsShown,
          `${driver.Name}: No health for the undetected driver`
        );
      } else {
        assert.equal(
          driverHead.health,
          driver.Healthy ? 'Healthy' : 'Unhealthy',
          `${driver.Name}: Health is correct`
        );
        assert.ok(
          driverHead.healthClass.includes(driver.Healthy ? 'running' : 'failed'),
          `${driver.Name}: Swatch with correct class is shown`
        );
      }
    });
  });
});

test('each driver can be opened to see a message and attributes', function(assert) {
  // Only detected drivers can be expanded
  const nodeDrivers = node.drivers;
  Object.values(nodeDrivers).forEach(driver => {
    driver.Detected = true;
  });
  node.drivers = nodeDrivers;

  const driver = Object.keys(node.drivers)
    .map(driverName => assign({ Name: driverName }, node.drivers[driverName]))
    .sortBy('Name')[0];

  ClientDetail.visit({ id: node.id });
  const driverHead = ClientDetail.driverHeads.objectAt(0);
  const driverBody = ClientDetail.driverBodies.objectAt(0);

  andThen(() => {
    assert.notOk(driverBody.descriptionIsShown, 'Driver health description is not shown');
    assert.notOk(driverBody.attributesAreShown, 'Driver attributes section is not shown');
    driverHead.toggle();
  });

  andThen(() => {
    assert.equal(
      driverBody.description,
      driver.HealthDescription,
      'Driver health description is now shown'
    );
    assert.ok(driverBody.attributesAreShown, 'Driver attributes section is now shown');
  });
});

test('the status light indicates when the node is ineligible for scheduling', function(assert) {
  node = server.create('node', {
    schedulingEligibility: 'ineligible',
  });

  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.equal(
      ClientDetail.statusLight.objectAt(0).id,
      'ineligible',
      'Title status light is in the ineligible state'
    );
  });
});

test('when the node has a drain strategy with a positive deadline, the drain stategy section prints the duration', function(assert) {
  const deadline = 5400000000000; // 1.5 hours in nanoseconds
  const forceDeadline = moment().add(1, 'd');

  node = server.create('node', {
    drain: true,
    schedulingEligibility: 'ineligible',
    drainStrategy: {
      Deadline: deadline,
      ForceDeadline: forceDeadline.toISOString(),
      IgnoreSystemJobs: false,
    },
  });

  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.ok(
      ClientDetail.drain.deadline.includes(formatDuration(deadline)),
      'Deadline is shown in a human formatted way'
    );

    assert.ok(
      ClientDetail.drain.forcedDeadline.includes(forceDeadline.format("MMM DD, 'YY HH:mm:ss ZZ")),
      'Force deadline is shown as an absolute date'
    );

    assert.ok(
      ClientDetail.drain.forcedDeadline.includes(forceDeadline.fromNow()),
      'Force deadline is shown as a relative date'
    );

    assert.ok(
      ClientDetail.drain.ignoreSystemJobs.endsWith('No'),
      'Ignore System Jobs state is shown'
    );
  });
});

test('when the node has a drain stategy with no deadline, the drain stategy section mentions that and omits the force deadline', function(assert) {
  const deadline = 0;

  node = server.create('node', {
    drain: true,
    schedulingEligibility: 'ineligible',
    drainStrategy: {
      Deadline: deadline,
      ForceDeadline: '0001-01-01T00:00:00Z', // null as a date
      IgnoreSystemJobs: true,
    },
  });

  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.ok(
      ClientDetail.drain.deadline.includes('No deadline'),
      'The value for Deadline is "no deadline"'
    );

    assert.notOk(
      ClientDetail.drain.hasForcedDeadline,
      'Forced deadline is not shown since there is no forced deadline'
    );

    assert.ok(
      ClientDetail.drain.ignoreSystemJobs.endsWith('Yes'),
      'Ignore System Jobs state is shown'
    );
  });
});

test('when the node has a drain stategy with a negative deadline, the drain strategy section shows the force badge', function(assert) {
  const deadline = -1;

  node = server.create('node', {
    drain: true,
    schedulingEligibility: 'ineligible',
    drainStrategy: {
      Deadline: deadline,
      ForceDeadline: '0001-01-01T00:00:00Z', // null as a date
      IgnoreSystemJobs: false,
    },
  });

  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.equal(ClientDetail.drain.badgeLabel, 'Forced Drain', 'Forced Drain badge is described');
    assert.ok(ClientDetail.drain.badgeIsDangerous, 'Forced Drain is shown in a red badge');

    assert.notOk(
      ClientDetail.drain.hasForcedDeadline,
      'Forced deadline is not shown since there is no forced deadline'
    );

    assert.ok(
      ClientDetail.drain.ignoreSystemJobs.endsWith('No'),
      'Ignore System Jobs state is shown'
    );
  });
});

moduleForAcceptance('Acceptance | client detail (multi-namespace)', {
  beforeEach() {
    server.create('node', 'forceIPv4', { schedulingEligibility: 'eligible' });
    node = server.db.nodes[0];

    // Related models
    server.create('namespace');
    server.create('namespace', { id: 'other-namespace' });

    server.create('agent');

    // Make a job for each namespace, but have both scheduled on the same node
    server.create('job', { id: 'job-1', namespaceId: 'default', createAllocations: false });
    server.createList('allocation', 3, { nodeId: node.id, clientStatus: 'running' });

    server.create('job', { id: 'job-2', namespaceId: 'other-namespace', createAllocations: false });
    server.createList('allocation', 3, {
      nodeId: node.id,
      jobId: 'job-2',
      clientStatus: 'running',
    });
  },
});

test('when the node has allocations on different namespaces, the associated jobs are fetched correctly', function(assert) {
  window.localStorage.nomadActiveNamespace = 'other-namespace';

  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.equal(
      ClientDetail.allocations.length,
      server.db.allocations.length,
      'All allocations are scheduled on this node'
    );
    assert.ok(
      server.pretender.handledRequests.findBy('url', '/v1/job/job-1'),
      'Job One fetched correctly'
    );
    assert.ok(
      server.pretender.handledRequests.findBy('url', '/v1/job/job-2?namespace=other-namespace'),
      'Job Two fetched correctly'
    );
  });
});
