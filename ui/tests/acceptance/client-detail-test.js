import { currentURL } from '@ember/test-helpers';
import { assign } from '@ember/polyfills';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import setupMirage from 'ember-cli-mirage/test-support/setup-mirage';
import { formatBytes } from 'nomad-ui/helpers/format-bytes';
import formatDuration from 'nomad-ui/utils/format-duration';
import moment from 'moment';
import ClientDetail from 'nomad-ui/tests/pages/clients/detail';
import Clients from 'nomad-ui/tests/pages/clients/list';
import Jobs from 'nomad-ui/tests/pages/jobs/list';

let node;

const wasPreemptedFilter = allocation => !!allocation.preemptedByAllocation;

module('Acceptance | client detail', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    server.create('node', 'forceIPv4', { schedulingEligibility: 'eligible' });
    node = server.db.nodes[0];

    // Related models
    server.create('agent');
    server.create('job', { createAllocations: false });
    server.createList('allocation', 3);
    server.create('allocation', 'preempted');

    // Force all allocations into the running state so now allocation rows are missing
    // CPU/Mem runtime metrics
    server.schema.allocations.all().models.forEach(allocation => {
      allocation.update({ clientStatus: 'running' });
    });
  });

  test('/clients/:id should have a breadcrumb trail linking back to clients', async function(assert) {
    await ClientDetail.visit({ id: node.id });

    assert.equal(document.title, `Client ${node.name} - Nomad`);

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
    await ClientDetail.breadcrumbFor('clients.index').visit();
    assert.equal(currentURL(), '/clients', 'First breadcrumb links back to clients');
  });

  test('/clients/:id should list immediate details for the node in the title', async function(assert) {
    await ClientDetail.visit({ id: node.id });

    assert.ok(ClientDetail.title.includes(node.name), 'Title includes name');
    assert.ok(ClientDetail.title.includes(node.id), 'Title includes id');
    assert.equal(
      ClientDetail.statusLight.objectAt(0).id,
      node.status,
      'Title includes status light'
    );
  });

  test('/clients/:id should list additional detail for the node below the title', async function(assert) {
    await ClientDetail.visit({ id: node.id });

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

  test('/clients/:id should include resource utilization graphs', async function(assert) {
    await ClientDetail.visit({ id: node.id });

    assert.equal(ClientDetail.resourceCharts.length, 2, 'Two resource utilization graphs');
    assert.equal(ClientDetail.resourceCharts.objectAt(0).name, 'CPU', 'First chart is CPU');
    assert.equal(ClientDetail.resourceCharts.objectAt(1).name, 'Memory', 'Second chart is Memory');
  });

  test('/clients/:id should list all allocations on the node', async function(assert) {
    const allocationsCount = server.db.allocations.where({ nodeId: node.id }).length;

    await ClientDetail.visit({ id: node.id });

    assert.equal(
      ClientDetail.allocations.length,
      allocationsCount,
      `Allocations table lists all ${allocationsCount} associated allocations`
    );
  });

  test('each allocation should have high-level details for the allocation', async function(assert) {
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

    await ClientDetail.visit({ id: node.id });

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

  test('each allocation should show job information even if the job is incomplete and already in the store', async function(assert) {
    // First, visit clients to load the allocations for each visible node.
    // Don't load the job belongsTo of the allocation! Leave it unfulfilled.

    await Clients.visit();

    // Then, visit jobs to load all jobs, which should implicitly fulfill
    // the job belongsTo of each allocation pointed at each job.

    await Jobs.visit();

    // Finally, visit a node to assert that the job name and task group name are
    // present. This will require reloading the job, since task groups aren't a
    // part of the jobs list response.

    await ClientDetail.visit({ id: node.id });

    const allocationRow = ClientDetail.allocations.objectAt(0);
    const allocation = server.db.allocations
      .where({ nodeId: node.id })
      .sortBy('modifyIndex')
      .reverse()[0];

    assert.equal(allocationRow.job, server.db.jobs.find(allocation.jobId).name, 'Job name');
    assert.ok(allocationRow.taskGroup.includes(allocation.taskGroup), 'Task group name');
  });

  test('each allocation should link to the allocation detail page', async function(assert) {
    const allocation = server.db.allocations
      .where({ nodeId: node.id })
      .sortBy('modifyIndex')
      .reverse()[0];

    await ClientDetail.visit({ id: node.id });
    await ClientDetail.allocations.objectAt(0).visit();

    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}`,
      'Allocation rows link to allocation detail pages'
    );
  });

  test('each allocation should link to the job the allocation belongs to', async function(assert) {
    await ClientDetail.visit({ id: node.id });

    const allocation = server.db.allocations.where({ nodeId: node.id })[0];
    const job = server.db.jobs.find(allocation.jobId);

    await ClientDetail.allocations.objectAt(0).visitJob();

    assert.equal(
      currentURL(),
      `/jobs/${job.id}`,
      'Allocation rows link to the job detail page for the allocation'
    );
  });

  test('the allocation section should show the count of preempted allocations on the client', async function(assert) {
    const allocations = server.db.allocations.where({ nodeId: node.id });

    await ClientDetail.visit({ id: node.id });

    assert.equal(
      ClientDetail.allocationFilter.allCount,
      allocations.length,
      'All filter/badge shows all allocations count'
    );
    assert.ok(
      ClientDetail.allocationFilter.preemptionsCount.startsWith(
        allocations.filter(wasPreemptedFilter).length
      ),
      'Preemptions filter/badge shows preempted allocations count'
    );
  });

  test('clicking the preemption badge filters the allocations table and sets a query param', async function(assert) {
    const allocations = server.db.allocations.where({ nodeId: node.id });

    await ClientDetail.visit({ id: node.id });
    await ClientDetail.allocationFilter.preemptions();

    assert.equal(
      ClientDetail.allocations.length,
      allocations.filter(wasPreemptedFilter).length,
      'Only preempted allocations are shown'
    );
    assert.equal(
      currentURL(),
      `/clients/${node.id}?preemptions=true`,
      'Filter is persisted in the URL'
    );
  });

  test('clicking the total allocations badge resets the filter and removes the query param', async function(assert) {
    const allocations = server.db.allocations.where({ nodeId: node.id });

    await ClientDetail.visit({ id: node.id });
    await ClientDetail.allocationFilter.preemptions();
    await ClientDetail.allocationFilter.all();

    assert.equal(ClientDetail.allocations.length, allocations.length, 'All allocations are shown');
    assert.equal(currentURL(), `/clients/${node.id}`, 'Filter is persisted in the URL');
  });

  test('navigating directly to the client detail page with the preemption query param set will filter the allocations table', async function(assert) {
    const allocations = server.db.allocations.where({ nodeId: node.id });

    await ClientDetail.visit({ id: node.id, preemptions: true });

    assert.equal(
      ClientDetail.allocations.length,
      allocations.filter(wasPreemptedFilter).length,
      'Only preempted allocations are shown'
    );
  });

  test('/clients/:id should list all attributes for the node', async function(assert) {
    await ClientDetail.visit({ id: node.id });

    assert.ok(ClientDetail.attributesTable, 'Attributes table is on the page');
  });

  test('/clients/:id lists all meta attributes', async function(assert) {
    node = server.create('node', 'forceIPv4', 'withMeta');

    await ClientDetail.visit({ id: node.id });

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

  test('/clients/:id shows an empty message when there is no meta data', async function(assert) {
    await ClientDetail.visit({ id: node.id });

    assert.notOk(ClientDetail.metaTable, 'Meta attributes table is not on the page');
    assert.ok(ClientDetail.emptyMetaMessage, 'Meta attributes is empty');
  });

  test('when the node is not found, an error message is shown, but the URL persists', async function(assert) {
    await ClientDetail.visit({ id: 'not-a-real-node' });

    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/node/not-a-real-node',
      'A request to the nonexistent node is made'
    );
    assert.equal(currentURL(), '/clients/not-a-real-node', 'The URL persists');
    assert.ok(ClientDetail.error.isShown, 'Error message is shown');
    assert.equal(ClientDetail.error.title, 'Not Found', 'Error message is for 404');
  });

  test('/clients/:id shows the recent events list', async function(assert) {
    await ClientDetail.visit({ id: node.id });

    assert.ok(ClientDetail.hasEvents, 'Client events section exists');
  });

  test('each node event shows basic node event information', async function(assert) {
    const event = server.db.nodeEvents
      .where({ nodeId: node.id })
      .sortBy('time')
      .reverse()[0];

    await ClientDetail.visit({ id: node.id });

    const eventRow = ClientDetail.events.objectAt(0);
    assert.equal(
      eventRow.time,
      moment(event.time).format("MMM DD, 'YY HH:mm:ss ZZ"),
      'Event timestamp'
    );
    assert.equal(eventRow.subsystem, event.subsystem, 'Event subsystem');
    assert.equal(eventRow.message, event.message, 'Event message');
  });

  test('/clients/:id shows the driver status of every driver for the node', async function(assert) {
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

    await ClientDetail.visit({ id: node.id });

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

  test('each driver can be opened to see a message and attributes', async function(assert) {
    // Only detected drivers can be expanded
    const nodeDrivers = node.drivers;
    Object.values(nodeDrivers).forEach(driver => {
      driver.Detected = true;
    });
    node.drivers = nodeDrivers;

    const driver = Object.keys(node.drivers)
      .map(driverName => assign({ Name: driverName }, node.drivers[driverName]))
      .sortBy('Name')[0];

    await ClientDetail.visit({ id: node.id });
    const driverHead = ClientDetail.driverHeads.objectAt(0);
    const driverBody = ClientDetail.driverBodies.objectAt(0);

    assert.notOk(driverBody.descriptionIsShown, 'Driver health description is not shown');
    assert.notOk(driverBody.attributesAreShown, 'Driver attributes section is not shown');

    await driverHead.toggle();
    assert.equal(
      driverBody.description,
      driver.HealthDescription,
      'Driver health description is now shown'
    );
    assert.ok(driverBody.attributesAreShown, 'Driver attributes section is now shown');
  });

  test('the status light indicates when the node is ineligible for scheduling', async function(assert) {
    node = server.create('node', {
      schedulingEligibility: 'ineligible',
    });

    await ClientDetail.visit({ id: node.id });

    assert.equal(
      ClientDetail.statusLight.objectAt(0).id,
      'ineligible',
      'Title status light is in the ineligible state'
    );
  });

  test('when the node has a drain strategy with a positive deadline, the drain stategy section prints the duration', async function(assert) {
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

    await ClientDetail.visit({ id: node.id });

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

  test('when the node has a drain stategy with no deadline, the drain stategy section mentions that and omits the force deadline', async function(assert) {
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

    await ClientDetail.visit({ id: node.id });

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

  test('when the node has a drain stategy with a negative deadline, the drain strategy section shows the force badge', async function(assert) {
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

    await ClientDetail.visit({ id: node.id });

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

module('Acceptance | client detail (multi-namespace)', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
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
  });

  test('when the node has allocations on different namespaces, the associated jobs are fetched correctly', async function(assert) {
    window.localStorage.nomadActiveNamespace = 'other-namespace';

    await ClientDetail.visit({ id: node.id });

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
