import { click, findAll, currentURL, find, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import moment from 'moment';
import ipParts from 'nomad-ui/utils/ip-parts';

let allocation;
let task;

moduleForAcceptance('Acceptance | task detail', {
  beforeEach() {
    server.create('agent');
    server.create('node');
    server.create('job', { createAllocations: false });
    allocation = server.create('allocation', 'withTaskWithPorts');
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    visit(`/allocations/${allocation.id}/${task.name}`);
  },
});

test('/allocation/:id/:task_name should name the task and list high-level task information', function(assert) {
  assert.ok(find('[data-test-title]').textContent.includes(task.name), 'Task name');
  assert.ok(find('[data-test-state]').textContent.includes(task.state), 'Task state');

  assert.ok(
    find('[data-test-started-at]').textContent.includes(
      moment(task.startedAt).format('MM/DD/YY HH:mm:ss')
    ),
    'Task started at'
  );
});

test('breadcrumbs match jobs / job / task group / allocation / task', function(assert) {
  const { jobId, taskGroup } = allocation;
  const job = server.db.jobs.find(jobId);

  const shortId = allocation.id.split('-')[0];

  assert.equal(
    find('[data-test-breadcrumb="Jobs"]').textContent.trim(),
    'Jobs',
    'Jobs is the first breadcrumb'
  );
  assert.equal(
    find(`[data-test-breadcrumb="${job.name}"]`).textContent.trim(),
    job.name,
    'Job is the second breadcrumb'
  );
  assert.equal(
    find(`[data-test-breadcrumb="${taskGroup}`).textContent.trim(),
    taskGroup,
    'Task Group is the third breadcrumb'
  );
  assert.equal(
    find(`[data-test-breadcrumb="${shortId}"]`).textContent.trim(),
    shortId,
    'Allocation short id is the fourth breadcrumb'
  );
  assert.equal(
    find(`[data-test-breadcrumb="${task.name}"]`).textContent.trim(),
    task.name,
    'Task name is the fifth breadcrumb'
  );

  click('[data-test-breadcrumb="Jobs"]');
  andThen(() => {
    assert.equal(currentURL(), '/jobs', 'Jobs breadcrumb links correctly');
  });
  andThen(() => {
    visit(`/allocations/${allocation.id}/${task.name}`);
  });
  andThen(() => {
    click(`[data-test-breadcrumb="${job.name}"]`);
  });
  andThen(() => {
    assert.equal(currentURL(), `/jobs/${job.id}`, 'Job breadcrumb links correctly');
  });
  andThen(() => {
    visit(`/allocations/${allocation.id}/${task.name}`);
  });
  andThen(() => {
    click(`[data-test-breadcrumb="${taskGroup}"]`);
  });
  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${job.id}/${taskGroup}`,
      'Task Group breadcrumb links correctly'
    );
  });
  andThen(() => {
    visit(`/allocations/${allocation.id}/${task.name}`);
  });
  andThen(() => {
    click(`[data-test-breadcrumb="${shortId}"]`);
  });
  andThen(() => {
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}`,
      'Allocations breadcrumb links correctly'
    );
  });
});

test('the addresses table lists all reserved and dynamic ports', function(assert) {
  const taskResources = allocation.taskResourcesIds
    .map(id => server.db.taskResources.find(id))
    .find(resources => resources.name === task.name);
  const reservedPorts = taskResources.resources.Networks[0].ReservedPorts;
  const dynamicPorts = taskResources.resources.Networks[0].DynamicPorts;
  const addresses = reservedPorts.concat(dynamicPorts);

  assert.equal(
    findAll('[data-test-task-address]').length,
    addresses.length,
    'All addresses are listed'
  );
});

test('each address row shows the label and value of the address', function(assert) {
  const node = server.db.nodes.find(allocation.nodeId);
  const taskResources = allocation.taskResourcesIds
    .map(id => server.db.taskResources.find(id))
    .findBy('name', task.name);
  const reservedPorts = taskResources.resources.Networks[0].ReservedPorts;
  const dynamicPorts = taskResources.resources.Networks[0].DynamicPorts;
  const address = reservedPorts.concat(dynamicPorts).sortBy('Label')[0];

  const addressRow = find('[data-test-task-address]');
  assert.equal(
    addressRow.querySelector('[data-test-task-address-is-dynamic]').textContent.trim(),
    reservedPorts.includes(address) ? 'No' : 'Yes',
    'Dynamic port is denoted as such'
  );
  assert.equal(
    addressRow.querySelector('[data-test-task-address-name]').textContent.trim(),
    address.Label,
    'Label'
  );
  assert.equal(
    addressRow.querySelector('[data-test-task-address-address]').textContent.trim(),
    `${ipParts(node.httpAddr).address}:${address.Value}`,
    'Value'
  );
});

test('the events table lists all recent events', function(assert) {
  const events = server.db.taskEvents.where({ taskStateId: task.id });

  assert.equal(
    findAll('[data-test-task-event]').length,
    events.length,
    `Lists ${events.length} events`
  );
});

test('each recent event should list the time, type, and description of the event', function(assert) {
  const event = server.db.taskEvents.where({ taskStateId: task.id })[0];
  const recentEvent = findAll('[data-test-task-event]').get('lastObject');

  assert.equal(
    recentEvent.querySelector('[data-test-task-event-time]').textContent.trim(),
    moment(event.time / 1000000).format('MM/DD/YY HH:mm:ss'),
    'Event timestamp'
  );
  assert.equal(
    recentEvent.querySelector('[data-test-task-event-type]').textContent.trim(),
    event.type,
    'Event type'
  );
  assert.equal(
    recentEvent.querySelector('[data-test-task-event-message]').textContent.trim(),
    event.displayMessage,
    'Event message'
  );
});

test('when the allocation is not found, the application errors', function(assert) {
  visit(`/allocations/not-a-real-allocation/${task.name}`);

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/allocation/not-a-real-allocation',
      'A request to the nonexistent allocation is made'
    );
    assert.equal(
      currentURL(),
      `/allocations/not-a-real-allocation/${task.name}`,
      'The URL persists'
    );
    assert.ok(find('[data-test-error]'), 'Error message is shown');
    assert.equal(
      find('[data-test-error-title]').textContent,
      'Not Found',
      'Error message is for 404'
    );
  });
});

test('when the allocation is found but the task is not, the application errors', function(assert) {
  visit(`/allocations/${allocation.id}/not-a-real-task-name`);

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 200).url,
      `/v1/allocation/${allocation.id}`,
      'A request to the allocation is made successfully'
    );
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}/not-a-real-task-name`,
      'The URL persists'
    );
    assert.ok(find('[data-test-error]'), 'Error message is shown');
    assert.equal(
      find('[data-test-error-title]').textContent,
      'Not Found',
      'Error message is for 404'
    );
  });
});

moduleForAcceptance('Acceptance | task detail (no addresses)', {
  beforeEach() {
    server.create('agent');
    server.create('node');
    server.create('job');
    allocation = server.create('allocation', 'withoutTaskWithPorts');
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    visit(`/allocations/${allocation.id}/${task.name}`);
  },
});

test('when the task has no addresses, the addresses table is not shown', function(assert) {
  assert.notOk(find('[data-test-task-addresses]'), 'No addresses table');
});
