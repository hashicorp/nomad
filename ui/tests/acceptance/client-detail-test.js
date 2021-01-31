import { currentURL, waitUntil, settled } from '@ember/test-helpers';
import { assign } from '@ember/polyfills';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import { formatBytes } from 'nomad-ui/helpers/format-bytes';
import moment from 'moment';
import ClientDetail from 'nomad-ui/tests/pages/clients/detail';
import Clients from 'nomad-ui/tests/pages/clients/list';
import Jobs from 'nomad-ui/tests/pages/jobs/list';
import Layout from 'nomad-ui/tests/pages/layout';

let node;
let managementToken;
let clientToken;

const wasPreemptedFilter = allocation => !!allocation.preemptedByAllocation;

module('Acceptance | client detail', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    server.create('node', 'forceIPv4', { schedulingEligibility: 'eligible' });
    node = server.db.nodes[0];

    managementToken = server.create('token');
    clientToken = server.create('token');

    window.localStorage.nomadTokenSecret = managementToken.secretId;

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

  test('it passes an accessibility audit', async function(assert) {
    await ClientDetail.visit({ id: node.id });
    await a11yAudit(assert);
  });

  test('/clients/:id should have a breadcrumb trail linking back to clients', async function(assert) {
    await ClientDetail.visit({ id: node.id });

    assert.equal(document.title, `Client ${node.name} - Nomad`);

    assert.equal(
      Layout.breadcrumbFor('clients.index').text,
      'Clients',
      'First breadcrumb says clients'
    );
    assert.equal(
      Layout.breadcrumbFor('clients.client').text,
      node.id.split('-')[0],
      'Second breadcrumb says the node short id'
    );
    await Layout.breadcrumbFor('clients.index').visit();
    assert.equal(currentURL(), '/clients', 'First breadcrumb links back to clients');
  });

  test('/clients/:id should list immediate details for the node in the title', async function(assert) {
    node = server.create('node', 'forceIPv4', { schedulingEligibility: 'eligible', drain: false });

    await ClientDetail.visit({ id: node.id });

    assert.ok(ClientDetail.title.includes(node.name), 'Title includes name');
    assert.ok(ClientDetail.clientId.includes(node.id), 'Title includes id');
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
    const cpuUsed = tasks.reduce((sum, task) => sum + task.resources.CPU, 0);
    const memoryUsed = tasks.reduce((sum, task) => sum + task.resources.MemoryMB, 0);

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
    assert.equal(allocationRow.volume, 'Yes', 'Volume');
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
      server.pretender.handledRequests
        .filter(request => !request.url.includes('policy'))
        .findBy('status', 404).url,
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
      drain: false,
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
      ClientDetail.drainDetails.deadline.includes(forceDeadline.fromNow(true)),
      'Deadline is shown in a human formatted way'
    );

    assert.equal(
      ClientDetail.drainDetails.deadlineTooltip,
      forceDeadline.format("MMM DD, 'YY HH:mm:ss ZZ"),
      'The tooltip for deadline shows the force deadline as an absolute date'
    );

    assert.ok(
      ClientDetail.drainDetails.drainSystemJobsText.endsWith('Yes'),
      'Drain System Jobs state is shown'
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

    assert.notOk(ClientDetail.drainDetails.durationIsShown, 'Duration is omitted');

    assert.ok(
      ClientDetail.drainDetails.deadline.includes('No deadline'),
      'The value for Deadline is "no deadline"'
    );

    assert.ok(
      ClientDetail.drainDetails.drainSystemJobsText.endsWith('No'),
      'Drain System Jobs state is shown'
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

    assert.ok(
      ClientDetail.drainDetails.forceDrainText.endsWith('Yes'),
      'Forced Drain is described'
    );

    assert.ok(ClientDetail.drainDetails.duration.includes('--'), 'Duration is shown but unset');

    assert.ok(ClientDetail.drainDetails.deadline.includes('--'), 'Deadline is shown but unset');

    assert.ok(
      ClientDetail.drainDetails.drainSystemJobsText.endsWith('Yes'),
      'Drain System Jobs state is shown'
    );
  });

  test('toggling node eligibility disables the toggle and sends the correct POST request', async function(assert) {
    node = server.create('node', {
      drain: false,
      schedulingEligibility: 'eligible',
    });

    server.pretender.post('/v1/node/:id/eligibility', () => [200, {}, ''], true);

    await ClientDetail.visit({ id: node.id });
    assert.ok(ClientDetail.eligibilityToggle.isActive);

    ClientDetail.eligibilityToggle.toggle();
    await waitUntil(() => server.pretender.handledRequests.findBy('method', 'POST'));

    assert.ok(ClientDetail.eligibilityToggle.isDisabled);
    server.pretender.resolve(server.pretender.requestReferences[0].request);

    await settled();

    assert.notOk(ClientDetail.eligibilityToggle.isActive);
    assert.notOk(ClientDetail.eligibilityToggle.isDisabled);

    const request = server.pretender.handledRequests.findBy('method', 'POST');
    assert.equal(request.url, `/v1/node/${node.id}/eligibility`);
    assert.deepEqual(JSON.parse(request.requestBody), {
      NodeID: node.id,
      Eligibility: 'ineligible',
    });

    ClientDetail.eligibilityToggle.toggle();
    await waitUntil(() => server.pretender.handledRequests.filterBy('method', 'POST').length === 2);
    server.pretender.resolve(server.pretender.requestReferences[0].request);

    assert.ok(ClientDetail.eligibilityToggle.isActive);
    const request2 = server.pretender.handledRequests.filterBy('method', 'POST')[1];

    assert.equal(request2.url, `/v1/node/${node.id}/eligibility`);
    assert.deepEqual(JSON.parse(request2.requestBody), {
      NodeID: node.id,
      Eligibility: 'eligible',
    });
  });

  test('starting a drain sends the correct POST request', async function(assert) {
    let request;

    node = server.create('node', {
      drain: false,
      schedulingEligibility: 'eligible',
    });

    await ClientDetail.visit({ id: node.id });
    await ClientDetail.drainPopover.toggle();
    await ClientDetail.drainPopover.submit();

    request = server.pretender.handledRequests.filterBy('method', 'POST').pop();

    assert.equal(request.url, `/v1/node/${node.id}/drain`);
    assert.deepEqual(
      JSON.parse(request.requestBody),
      {
        NodeID: node.id,
        DrainSpec: {
          Deadline: 0,
          IgnoreSystemJobs: false,
        },
      },
      'Drain with default settings'
    );

    await ClientDetail.drainPopover.toggle();
    await ClientDetail.drainPopover.deadlineToggle.toggle();
    await ClientDetail.drainPopover.submit();

    request = server.pretender.handledRequests.filterBy('method', 'POST').pop();

    assert.deepEqual(
      JSON.parse(request.requestBody),
      {
        NodeID: node.id,
        DrainSpec: {
          Deadline: 60 * 60 * 1000 * 1000000,
          IgnoreSystemJobs: false,
        },
      },
      'Drain with deadline toggled'
    );

    await ClientDetail.drainPopover.toggle();
    await ClientDetail.drainPopover.deadlineOptions.open();
    await ClientDetail.drainPopover.deadlineOptions.options[1].choose();
    await ClientDetail.drainPopover.submit();

    request = server.pretender.handledRequests.filterBy('method', 'POST').pop();

    assert.deepEqual(
      JSON.parse(request.requestBody),
      {
        NodeID: node.id,
        DrainSpec: {
          Deadline: 4 * 60 * 60 * 1000 * 1000000,
          IgnoreSystemJobs: false,
        },
      },
      'Drain with non-default preset deadline set'
    );

    await ClientDetail.drainPopover.toggle();
    await ClientDetail.drainPopover.deadlineOptions.open();
    const optionsCount = ClientDetail.drainPopover.deadlineOptions.options.length;
    await ClientDetail.drainPopover.deadlineOptions.options.objectAt(optionsCount - 1).choose();
    await ClientDetail.drainPopover.setCustomDeadline('1h40m20s');
    await ClientDetail.drainPopover.submit();

    request = server.pretender.handledRequests.filterBy('method', 'POST').pop();

    assert.deepEqual(
      JSON.parse(request.requestBody),
      {
        NodeID: node.id,
        DrainSpec: {
          Deadline: ((1 * 60 + 40) * 60 + 20) * 1000 * 1000000,
          IgnoreSystemJobs: false,
        },
      },
      'Drain with custom deadline set'
    );

    await ClientDetail.drainPopover.toggle();
    await ClientDetail.drainPopover.deadlineToggle.toggle();
    await ClientDetail.drainPopover.forceDrainToggle.toggle();
    await ClientDetail.drainPopover.submit();

    request = server.pretender.handledRequests.filterBy('method', 'POST').pop();

    assert.deepEqual(
      JSON.parse(request.requestBody),
      {
        NodeID: node.id,
        DrainSpec: {
          Deadline: -1,
          IgnoreSystemJobs: false,
        },
      },
      'Drain with force set'
    );

    await ClientDetail.drainPopover.toggle();
    await ClientDetail.drainPopover.systemJobsToggle.toggle();
    await ClientDetail.drainPopover.submit();

    request = server.pretender.handledRequests.filterBy('method', 'POST').pop();

    assert.deepEqual(
      JSON.parse(request.requestBody),
      {
        NodeID: node.id,
        DrainSpec: {
          Deadline: -1,
          IgnoreSystemJobs: true,
        },
      },
      'Drain system jobs unset'
    );
  });

  test('the drain popover cancel button closes the popover', async function(assert) {
    node = server.create('node', {
      drain: false,
      schedulingEligibility: 'eligible',
    });

    await ClientDetail.visit({ id: node.id });
    assert.notOk(ClientDetail.drainPopover.isOpen);

    await ClientDetail.drainPopover.toggle();
    assert.ok(ClientDetail.drainPopover.isOpen);

    await ClientDetail.drainPopover.cancel();
    assert.notOk(ClientDetail.drainPopover.isOpen);
    assert.equal(server.pretender.handledRequests.filterBy('method', 'POST'), 0);
  });

  test('toggling eligibility is disabled while a drain is active', async function(assert) {
    node = server.create('node', {
      drain: true,
      schedulingEligibility: 'ineligible',
    });

    await ClientDetail.visit({ id: node.id });
    assert.ok(ClientDetail.eligibilityToggle.isDisabled);
  });

  test('stopping a drain sends the correct POST request', async function(assert) {
    node = server.create('node', {
      drain: true,
      schedulingEligibility: 'ineligible',
    });

    await ClientDetail.visit({ id: node.id });
    assert.ok(ClientDetail.stopDrainIsPresent);

    await ClientDetail.stopDrain.idle();
    await ClientDetail.stopDrain.confirm();

    const request = server.pretender.handledRequests.findBy('method', 'POST');
    assert.equal(request.url, `/v1/node/${node.id}/drain`);
    assert.deepEqual(JSON.parse(request.requestBody), {
      NodeID: node.id,
      DrainSpec: null,
    });
  });

  test('when a drain is active, the "drain" popover is labeled as the "update" popover', async function(assert) {
    node = server.create('node', {
      drain: true,
      schedulingEligibility: 'ineligible',
    });

    await ClientDetail.visit({ id: node.id });
    assert.equal(ClientDetail.drainPopover.label, 'Update Drain');
  });

  test('forcing a drain sends the correct POST request', async function(assert) {
    node = server.create('node', {
      drain: true,
      schedulingEligibility: 'ineligible',
      drainStrategy: {
        Deadline: 0,
        IgnoreSystemJobs: true,
      },
    });

    await ClientDetail.visit({ id: node.id });
    await ClientDetail.drainDetails.force.idle();
    await ClientDetail.drainDetails.force.confirm();

    const request = server.pretender.handledRequests.findBy('method', 'POST');
    assert.equal(request.url, `/v1/node/${node.id}/drain`);
    assert.deepEqual(JSON.parse(request.requestBody), {
      NodeID: node.id,
      DrainSpec: {
        Deadline: -1,
        IgnoreSystemJobs: true,
      },
    });
  });

  test('when stopping a drain fails, an error is shown', async function(assert) {
    node = server.create('node', {
      drain: true,
      schedulingEligibility: 'ineligible',
    });

    server.pretender.post('/v1/node/:id/drain', () => [500, {}, '']);

    await ClientDetail.visit({ id: node.id });
    await ClientDetail.stopDrain.idle();
    await ClientDetail.stopDrain.confirm();

    assert.ok(ClientDetail.stopDrainError.isPresent);
    assert.ok(ClientDetail.stopDrainError.title.includes('Stop Drain Error'));

    await ClientDetail.stopDrainError.dismiss();
    assert.notOk(ClientDetail.stopDrainError.isPresent);
  });

  test('when starting a drain fails, an error message is shown', async function(assert) {
    node = server.create('node', {
      drain: false,
      schedulingEligibility: 'eligible',
    });

    server.pretender.post('/v1/node/:id/drain', () => [500, {}, '']);

    await ClientDetail.visit({ id: node.id });
    await ClientDetail.drainPopover.toggle();
    await ClientDetail.drainPopover.submit();

    assert.ok(ClientDetail.drainError.isPresent);
    assert.ok(ClientDetail.drainError.title.includes('Drain Error'));

    await ClientDetail.drainError.dismiss();
    assert.notOk(ClientDetail.drainError.isPresent);
  });

  test('when updating a drain fails, an error message is shown', async function(assert) {
    node = server.create('node', {
      drain: true,
      schedulingEligibility: 'ineligible',
    });

    server.pretender.post('/v1/node/:id/drain', () => [500, {}, '']);

    await ClientDetail.visit({ id: node.id });
    await ClientDetail.drainPopover.toggle();
    await ClientDetail.drainPopover.submit();

    assert.ok(ClientDetail.drainError.isPresent);
    assert.ok(ClientDetail.drainError.title.includes('Drain Error'));

    await ClientDetail.drainError.dismiss();
    assert.notOk(ClientDetail.drainError.isPresent);
  });

  test('when toggling eligibility fails, an error message is shown', async function(assert) {
    node = server.create('node', {
      drain: false,
      schedulingEligibility: 'eligible',
    });

    server.pretender.post('/v1/node/:id/eligibility', () => [500, {}, '']);

    await ClientDetail.visit({ id: node.id });
    await ClientDetail.eligibilityToggle.toggle();

    assert.ok(ClientDetail.eligibilityError.isPresent);
    assert.ok(ClientDetail.eligibilityError.title.includes('Eligibility Error'));

    await ClientDetail.eligibilityError.dismiss();
    assert.notOk(ClientDetail.eligibilityError.isPresent);
  });

  test('when navigating away from a client that has an error message to another client, the error is not shown', async function(assert) {
    node = server.create('node', {
      drain: false,
      schedulingEligibility: 'eligible',
    });

    const node2 = server.create('node');

    server.pretender.post('/v1/node/:id/eligibility', () => [500, {}, '']);

    await ClientDetail.visit({ id: node.id });
    await ClientDetail.eligibilityToggle.toggle();

    assert.ok(ClientDetail.eligibilityError.isPresent);
    assert.ok(ClientDetail.eligibilityError.title.includes('Eligibility Error'));

    await ClientDetail.visit({ id: node2.id });

    assert.notOk(ClientDetail.eligibilityError.isPresent);
  });

  test('toggling eligibility and node drain are disabled when the active ACL token does not permit node write', async function(assert) {
    window.localStorage.nomadTokenSecret = clientToken.secretId;

    await ClientDetail.visit({ id: node.id });
    assert.ok(ClientDetail.eligibilityToggle.isDisabled);
    assert.ok(ClientDetail.drainPopover.isDisabled);
  });

  test('the host volumes table lists all host volumes in alphabetical order by name', async function(assert) {
    await ClientDetail.visit({ id: node.id });

    const sortedHostVolumes = Object.keys(node.hostVolumes)
      .map(key => node.hostVolumes[key])
      .sortBy('Name');

    assert.ok(ClientDetail.hasHostVolumes);
    assert.equal(ClientDetail.hostVolumes.length, Object.keys(node.hostVolumes).length);

    ClientDetail.hostVolumes.forEach((volume, index) => {
      assert.equal(volume.name, sortedHostVolumes[index].Name);
    });
  });

  test('each host volume row contains information about the host volume', async function(assert) {
    await ClientDetail.visit({ id: node.id });

    const sortedHostVolumes = Object.keys(node.hostVolumes)
      .map(key => node.hostVolumes[key])
      .sortBy('Name');

    ClientDetail.hostVolumes[0].as(volume => {
      const volumeRow = sortedHostVolumes[0];
      assert.equal(volume.name, volumeRow.Name);
      assert.equal(volume.path, volumeRow.Path);
      assert.equal(volume.permissions, volumeRow.ReadOnly ? 'Read' : 'Read/Write');
    });
  });

  test('the host volumes table is not shown if the client has no host volumes', async function(assert) {
    node = server.create('node', 'noHostVolumes');

    await ClientDetail.visit({ id: node.id });

    assert.notOk(ClientDetail.hasHostVolumes);
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
