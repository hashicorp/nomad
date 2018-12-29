import { currentURL } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import Task from 'nomad-ui/tests/pages/allocations/task/detail';
import moment from 'moment';

let allocation;
let task;

moduleForAcceptance('Acceptance | task detail', {
  beforeEach() {
    server.create('agent');
    server.create('node');
    server.create('job', { createAllocations: false });
    allocation = server.create('allocation', 'withTaskWithPorts');
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    Task.visit({ id: allocation.id, name: task.name });
  },
});

test('/allocation/:id/:task_name should name the task and list high-level task information', function(assert) {
  assert.ok(Task.title.includes(task.name), 'Task name');
  assert.ok(Task.state.includes(task.state), 'Task state');

  assert.ok(
    Task.startedAt.includes(moment(task.startedAt).format('MM/DD/YY HH:mm:ss')),
    'Task started at'
  );
});

test('breadcrumbs match jobs / job / task group / allocation / task', function(assert) {
  const { jobId, taskGroup } = allocation;
  const job = server.db.jobs.find(jobId);

  const shortId = allocation.id.split('-')[0];

  assert.equal(Task.breadcrumbFor('jobs.index').text, 'Jobs', 'Jobs is the first breadcrumb');
  assert.equal(Task.breadcrumbFor('jobs.job.index').text, job.name, 'Job is the second breadcrumb');
  assert.equal(
    Task.breadcrumbFor('jobs.job.task-group').text,
    taskGroup,
    'Task Group is the third breadcrumb'
  );
  assert.equal(
    Task.breadcrumbFor('allocations.allocation').text,
    shortId,
    'Allocation short id is the fourth breadcrumb'
  );
  assert.equal(
    Task.breadcrumbFor('allocations.allocation.task').text,
    task.name,
    'Task name is the fifth breadcrumb'
  );

  Task.breadcrumbFor('jobs.index').visit();
  andThen(() => {
    assert.equal(currentURL(), '/jobs', 'Jobs breadcrumb links correctly');
  });
  andThen(() => {
    Task.visit({ id: allocation.id, name: task.name });
  });
  andThen(() => {
    Task.breadcrumbFor('jobs.job.index').visit();
  });
  andThen(() => {
    assert.equal(currentURL(), `/jobs/${job.id}`, 'Job breadcrumb links correctly');
  });
  andThen(() => {
    Task.visit({ id: allocation.id, name: task.name });
  });
  andThen(() => {
    Task.breadcrumbFor('jobs.job.task-group').visit();
  });
  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${job.id}/${taskGroup}`,
      'Task Group breadcrumb links correctly'
    );
  });
  andThen(() => {
    Task.visit({ id: allocation.id, name: task.name });
  });
  andThen(() => {
    Task.breadcrumbFor('allocations.allocation').visit();
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

  assert.equal(Task.addresses.length, addresses.length, 'All addresses are listed');
});

test('each address row shows the label and value of the address', function(assert) {
  const taskResources = allocation.taskResourcesIds
    .map(id => server.db.taskResources.find(id))
    .findBy('name', task.name);
  const networkAddress = taskResources.resources.Networks[0].IP;
  const reservedPorts = taskResources.resources.Networks[0].ReservedPorts;
  const dynamicPorts = taskResources.resources.Networks[0].DynamicPorts;
  const address = reservedPorts.concat(dynamicPorts).sortBy('Label')[0];

  const addressRow = Task.addresses.objectAt(0);
  assert.equal(
    addressRow.isDynamic,
    reservedPorts.includes(address) ? 'No' : 'Yes',
    'Dynamic port is denoted as such'
  );
  assert.equal(addressRow.name, address.Label, 'Label');
  assert.equal(addressRow.address, `${networkAddress}:${address.Value}`, 'Value');
});

test('the events table lists all recent events', function(assert) {
  const events = server.db.taskEvents.where({ taskStateId: task.id });

  assert.equal(Task.events.length, events.length, `Lists ${events.length} events`);
});

test('each recent event should list the time, type, and description of the event', function(assert) {
  const event = server.db.taskEvents.where({ taskStateId: task.id })[0];
  const recentEvent = Task.events.objectAt(Task.events.length - 1);

  assert.equal(
    recentEvent.time,
    moment(event.time / 1000000).format('MM/DD/YY HH:mm:ss'),
    'Event timestamp'
  );
  assert.equal(recentEvent.type, event.type, 'Event type');
  assert.equal(recentEvent.message, event.displayMessage, 'Event message');
});

test('when the allocation is not found, the application errors', function(assert) {
  Task.visit({ id: 'not-a-real-allocation', name: task.name });

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
    assert.ok(Task.error.isPresent, 'Error message is shown');
    assert.equal(Task.error.title, 'Not Found', 'Error message is for 404');
  });
});

test('when the allocation is found but the task is not, the application errors', function(assert) {
  Task.visit({ id: allocation.id, name: 'not-a-real-task-name' });

  andThen(() => {
    assert.ok(
      server.pretender.handledRequests
        .filterBy('status', 200)
        .mapBy('url')
        .includes(`/v1/allocation/${allocation.id}`),
      'A request to the allocation is made successfully'
    );
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}/not-a-real-task-name`,
      'The URL persists'
    );
    assert.ok(Task.error.isPresent, 'Error message is shown');
    assert.equal(Task.error.title, 'Not Found', 'Error message is for 404');
  });
});

moduleForAcceptance('Acceptance | task detail (no addresses)', {
  beforeEach() {
    server.create('agent');
    server.create('node');
    server.create('job');
    allocation = server.create('allocation', 'withoutTaskWithPorts');
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    Task.visit({ id: allocation.id, name: task.name });
  },
});

test('when the task has no addresses, the addresses table is not shown', function(assert) {
  assert.notOk(Task.hasAddresses, 'No addresses table');
});
