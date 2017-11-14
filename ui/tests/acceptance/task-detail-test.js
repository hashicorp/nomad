import Ember from 'ember';
import { click, findAll, currentURL, find, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import moment from 'moment';
import ipParts from 'nomad-ui/utils/ip-parts';

const { $ } = Ember;

let allocation;
let task;

moduleForAcceptance('Acceptance | task detail', {
  beforeEach() {
    server.create('agent');
    server.create('node');
    server.create('job', { createAllocations: false });
    allocation = server.create('allocation', 'withTaskWithPorts', {
      useMessagePassthru: true,
    });
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    visit(`/allocations/${allocation.id}/${task.name}`);
  },
});

test('/allocation/:id/:task_name should name the task and list high-level task information', function(
  assert
) {
  assert.ok(find('.title').textContent.includes(task.name), 'Task name');
  assert.ok(find('.title').textContent.includes(task.state), 'Task state');

  const inlineDefinitions = findAll('.inline-definitions .pair');
  assert.ok(
    inlineDefinitions[0].textContent.includes(moment(task.startedAt).format('MM/DD/YY HH:mm:ss')),
    'Task started at'
  );
});

test('breadcrumbs includes allocations and link to the allocation detail page', function(assert) {
  const breadcrumbs = findAll('.breadcrumb');
  assert.equal(
    breadcrumbs[0].textContent.trim(),
    'Allocations',
    'Allocations is the first breadcrumb'
  );
  assert.notEqual(
    breadcrumbs[0].tagName.toLowerCase(),
    'a',
    'Allocations breadcrumb is not a link'
  );
  assert.equal(
    breadcrumbs[1].textContent.trim(),
    allocation.id.split('-')[0],
    'Allocation short id is the second breadcrumb'
  );
  assert.equal(breadcrumbs[2].textContent.trim(), task.name, 'Task name is the third breadcrumb');

  click(breadcrumbs[1]);
  andThen(() => {
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}`,
      'Second breadcrumb links back to the allocation detail'
    );
  });
});

test('the addresses table lists all reserved and dynamic ports', function(assert) {
  const taskResources = allocation.taskResourcesIds
    .map(id => server.db.taskResources.find(id))
    .sortBy('name')[0];
  const reservedPorts = taskResources.resources.Networks[0].ReservedPorts;
  const dynamicPorts = taskResources.resources.Networks[0].DynamicPorts;
  const addresses = reservedPorts.concat(dynamicPorts);

  assert.equal(
    findAll('.addresses-list tbody tr').length,
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

  const addressRow = $(find('.addresses-list tbody tr'));
  assert.equal(
    addressRow
      .find('td:eq(0)')
      .text()
      .trim(),
    reservedPorts.includes(address) ? 'No' : 'Yes',
    'Dynamic port is denoted as such'
  );
  assert.equal(
    addressRow
      .find('td:eq(1)')
      .text()
      .trim(),
    address.Label,
    'Label'
  );
  assert.equal(
    addressRow
      .find('td:eq(2)')
      .text()
      .trim(),
    `${ipParts(node.httpAddr).address}:${address.Value}`,
    'Value'
  );
});

test('the events table lists all recent events', function(assert) {
  const events = server.db.taskEvents.where({ taskStateId: task.id });

  assert.equal(
    findAll('.task-events tbody tr').length,
    events.length,
    `Lists ${events.length} events`
  );
});

test('each recent event should list the time, type, and description of the event', function(
  assert
) {
  const event = server.db.taskEvents.where({ taskStateId: task.id })[0];
  const recentEvent = $('.task-events tbody tr:last');

  assert.equal(
    recentEvent
      .find('td:eq(0)')
      .text()
      .trim(),
    moment(event.time / 1000000).format('MM/DD/YY HH:mm:ss'),
    'Event timestamp'
  );
  assert.equal(
    recentEvent
      .find('td:eq(1)')
      .text()
      .trim(),
    event.type,
    'Event type'
  );
  assert.equal(
    recentEvent
      .find('td:eq(2)')
      .text()
      .trim(),
    event.message,
    'Event message'
  );
});

test('when the allocation is not found, the application errors', function(assert) {
  visit(`/allocations/not-a-real-allocation/${task.name}`);

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/allocation/not-a-real-allocation',
      'A request to the non-existent allocation is made'
    );
    assert.equal(
      currentURL(),
      `/allocations/not-a-real-allocation/${task.name}`,
      'The URL persists'
    );
    assert.ok(find('.error-message'), 'Error message is shown');
    assert.equal(
      find('.error-message .title').textContent,
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
    assert.ok(find('.error-message'), 'Error message is shown');
    assert.equal(
      find('.error-message .title').textContent,
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
  assert.notOk(find('.addresses-list'), 'No addresses table');
});
