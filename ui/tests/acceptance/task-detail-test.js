import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Task from 'nomad-ui/tests/pages/allocations/task/detail';
import moment from 'moment';

let allocation;
let task;

module('Acceptance | task detail', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    server.create('agent');
    server.create('node');
    server.create('job', { createAllocations: false });
    allocation = server.create('allocation', 'withTaskWithPorts', { clientStatus: 'running' });
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    await Task.visit({ id: allocation.id, name: task.name });
  });

  test('/allocation/:id/:task_name should name the task and list high-level task information', async function(assert) {
    assert.ok(Task.title.text.includes(task.name), 'Task name');
    assert.ok(Task.state.includes(task.state), 'Task state');

    assert.ok(
      Task.startedAt.includes(moment(task.startedAt).format("MMM DD, 'YY HH:mm:ss ZZ")),
      'Task started at'
    );

    assert.equal(document.title, `Task ${task.name} - Nomad`);
  });

  test('breadcrumbs match jobs / job / task group / allocation / task', async function(assert) {
    const { jobId, taskGroup } = allocation;
    const job = server.db.jobs.find(jobId);

    const shortId = allocation.id.split('-')[0];

    assert.equal(Task.breadcrumbFor('jobs.index').text, 'Jobs', 'Jobs is the first breadcrumb');
    assert.equal(
      Task.breadcrumbFor('jobs.job.index').text,
      job.name,
      'Job is the second breadcrumb'
    );
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

    await Task.breadcrumbFor('jobs.index').visit();
    assert.equal(currentURL(), '/jobs', 'Jobs breadcrumb links correctly');

    await Task.visit({ id: allocation.id, name: task.name });
    await Task.breadcrumbFor('jobs.job.index').visit();
    assert.equal(currentURL(), `/jobs/${job.id}`, 'Job breadcrumb links correctly');

    await Task.visit({ id: allocation.id, name: task.name });
    await Task.breadcrumbFor('jobs.job.task-group').visit();
    assert.equal(
      currentURL(),
      `/jobs/${job.id}/${taskGroup}`,
      'Task Group breadcrumb links correctly'
    );

    await Task.visit({ id: allocation.id, name: task.name });
    await Task.breadcrumbFor('allocations.allocation').visit();
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}`,
      'Allocations breadcrumb links correctly'
    );
  });

  test('/allocation/:id/:task_name should include resource utilization graphs', async function(assert) {
    assert.equal(Task.resourceCharts.length, 2, 'Two resource utilization graphs');
    assert.equal(Task.resourceCharts.objectAt(0).name, 'CPU', 'First chart is CPU');
    assert.equal(Task.resourceCharts.objectAt(1).name, 'Memory', 'Second chart is Memory');
  });

  test('the addresses table lists all reserved and dynamic ports', async function(assert) {
    const taskResources = allocation.taskResourceIds
      .map(id => server.db.taskResources.find(id))
      .find(resources => resources.name === task.name);
    const reservedPorts = taskResources.resources.Networks[0].ReservedPorts;
    const dynamicPorts = taskResources.resources.Networks[0].DynamicPorts;
    const addresses = reservedPorts.concat(dynamicPorts);

    assert.equal(Task.addresses.length, addresses.length, 'All addresses are listed');
  });

  test('each address row shows the label and value of the address', async function(assert) {
    const taskResources = allocation.taskResourceIds
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

  test('the events table lists all recent events', async function(assert) {
    const events = server.db.taskEvents.where({ taskStateId: task.id });

    assert.equal(Task.events.length, events.length, `Lists ${events.length} events`);
  });

  test('when a task has volumes, the volumes table is shown', async function(assert) {
    const taskGroup = server.schema.taskGroups.where({
      jobId: allocation.jobId,
      name: allocation.taskGroup,
    }).models[0];

    const jobTask = taskGroup.tasks.models.find(m => m.name === task.name);

    assert.ok(Task.hasVolumes);
    assert.equal(Task.volumes.length, jobTask.volumeMounts.length);
  });

  test('when a task does not have volumes, the volumes table is not shown', async function(assert) {
    const job = server.create('job', { createAllocations: false, noHostVolumes: true });
    allocation = server.create('allocation', { jobId: job.id, clientStatus: 'running' });
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    await Task.visit({ id: allocation.id, name: task.name });
    assert.notOk(Task.hasVolumes);
  });

  test('each volume in the volumes table shows information about the volume', async function(assert) {
    const taskGroup = server.schema.taskGroups.where({
      jobId: allocation.jobId,
      name: allocation.taskGroup,
    }).models[0];

    const jobTask = taskGroup.tasks.models.find(m => m.name === task.name);
    const volume = jobTask.volumeMounts[0];

    Task.volumes[0].as(volumeRow => {
      assert.equal(volumeRow.name, volume.Volume);
      assert.equal(volumeRow.destination, volume.Destination);
      assert.equal(volumeRow.permissions, volume.ReadOnly ? 'Read' : 'Read/Write');
      assert.equal(volumeRow.clientSource, taskGroup.volumes[volume.Volume].Source);
    });
  });

  test('each recent event should list the time, type, and description of the event', async function(assert) {
    const event = server.db.taskEvents.where({ taskStateId: task.id })[0];
    const recentEvent = Task.events.objectAt(Task.events.length - 1);

    assert.equal(
      recentEvent.time,
      moment(event.time / 1000000).format("MMM DD, 'YY HH:mm:ss ZZ"),
      'Event timestamp'
    );
    assert.equal(recentEvent.type, event.type, 'Event type');
    assert.equal(recentEvent.message, event.displayMessage, 'Event message');
  });

  test('when the allocation is not found, the application errors', async function(assert) {
    await Task.visit({ id: 'not-a-real-allocation', name: task.name });

    assert.equal(
      server.pretender.handledRequests
        .filter(request => !request.url.includes('policy'))
        .findBy('status', 404).url,
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

  test('when the allocation is found but the task is not, the application errors', async function(assert) {
    await Task.visit({ id: allocation.id, name: 'not-a-real-task-name' });

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

  test('task can be restarted', async function(assert) {
    await Task.restart.idle();
    await Task.restart.confirm();

    const request = server.pretender.handledRequests.findBy('method', 'PUT');
    assert.equal(
      request.url,
      `/v1/client/allocation/${allocation.id}/restart`,
      'Restart request is made for the allocation'
    );

    assert.deepEqual(
      JSON.parse(request.requestBody),
      { TaskName: task.name },
      'Restart request is made for the correct task'
    );
  });

  test('when task restart fails, an error message is shown', async function(assert) {
    server.pretender.put('/v1/client/allocation/:id/restart', () => [403, {}, '']);

    await Task.restart.idle();
    await Task.restart.confirm();

    assert.ok(Task.inlineError.isShown, 'Inline error is shown');
    assert.ok(Task.inlineError.title.includes('Could Not Restart Task'), 'Title is descriptive');
    assert.ok(
      /ACL token.+?allocation lifecycle/.test(Task.inlineError.message),
      'Message mentions ACLs and the appropriate permission'
    );

    await Task.inlineError.dismiss();

    assert.notOk(Task.inlineError.isShown, 'Inline error is no longer shown');
  });

  test('exec button is present', async function(assert) {
    assert.ok(Task.execButton.isPresent);
  });
});

module('Acceptance | task detail (no addresses)', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    server.create('agent');
    server.create('node');
    server.create('job');
    allocation = server.create('allocation', 'withoutTaskWithPorts', { clientStatus: 'running' });
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    await Task.visit({ id: allocation.id, name: task.name });
  });

  test('when the task has no addresses, the addresses table is not shown', async function(assert) {
    assert.notOk(Task.hasAddresses, 'No addresses table');
  });
});

module('Acceptance | task detail (different namespace)', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    server.create('agent');
    server.create('node');
    server.create('namespace');
    server.create('namespace', { id: 'other-namespace' });
    server.create('job', { createAllocations: false, namespaceId: 'other-namespace' });
    allocation = server.create('allocation', 'withTaskWithPorts', { clientStatus: 'running' });
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    await Task.visit({ id: allocation.id, name: task.name });
  });

  test('breadcrumbs match jobs / job / task group / allocation / task', async function(assert) {
    const { jobId, taskGroup } = allocation;
    const job = server.db.jobs.find(jobId);

    await Task.breadcrumbFor('jobs.index').visit();
    assert.equal(
      currentURL(),
      '/jobs?namespace=other-namespace',
      'Jobs breadcrumb links correctly'
    );

    await Task.visit({ id: allocation.id, name: task.name });
    await Task.breadcrumbFor('jobs.job.index').visit();
    assert.equal(
      currentURL(),
      `/jobs/${job.id}?namespace=other-namespace`,
      'Job breadcrumb links correctly'
    );

    await Task.visit({ id: allocation.id, name: task.name });
    await Task.breadcrumbFor('jobs.job.task-group').visit();
    assert.equal(
      currentURL(),
      `/jobs/${job.id}/${taskGroup}?namespace=other-namespace`,
      'Task Group breadcrumb links correctly'
    );

    await Task.visit({ id: allocation.id, name: task.name });
    await Task.breadcrumbFor('allocations.allocation').visit();
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}`,
      'Allocations breadcrumb links correctly'
    );
  });
});

module('Acceptance | task detail (not running)', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    server.create('agent');
    server.create('node');
    server.create('namespace');
    server.create('namespace', { id: 'other-namespace' });
    server.create('job', { createAllocations: false, namespaceId: 'other-namespace' });
    allocation = server.create('allocation', 'withTaskWithPorts', { clientStatus: 'complete' });
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    await Task.visit({ id: allocation.id, name: task.name });
  });

  test('when the allocation for a task is not running, the resource utilization graphs are replaced by an empty message', async function(assert) {
    assert.equal(Task.resourceCharts.length, 0, 'No resource charts');
    assert.equal(Task.resourceEmptyMessage, "Task isn't running", 'Empty message is appropriate');
  });

  test('exec button is absent', async function(assert) {
    assert.notOk(Task.execButton.isPresent);
  });
});

module('Acceptance | proxy task detail', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    server.create('agent');
    server.create('node');
    server.create('job', { createAllocations: false });
    allocation = server.create('allocation', 'withTaskWithPorts', { clientStatus: 'running' });

    const taskState = allocation.task_states.models[0];
    const task = server.schema.tasks.findBy({ name: taskState.name });
    task.update('kind', 'connect-proxy:task');
    task.save();

    await Task.visit({ id: allocation.id, name: taskState.name });
  });

  test('a proxy tag is shown', async function(assert) {
    assert.ok(Task.title.proxyTag.isPresent);
  });
});
