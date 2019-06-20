import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import setupMirage from 'ember-cli-mirage/test-support/setup-mirage';
import FS from 'nomad-ui/tests/pages/allocations/task/fs';

let allocation;
let task;

module('Acceptance | task fs', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    server.create('agent');
    server.create('node', 'forceIPv4');
    const job = server.create('job', { createAllocations: false });

    allocation = server.create('allocation', { jobId: job.id, clientStatus: 'running' });
    task = server.schema.taskStates.where({ allocationId: allocation.id }).models[0];
  });

  test('visiting /allocations/:allocation_id/:task_name/fs', async function(assert) {
    await FS.visit({ id: allocation.id, name: task.name });
    assert.equal(currentURL(), `/allocations/${allocation.id}/${task.name}/fs`, 'No redirect');
  });

  test('when the task is not running, an empty state is shown', async function(assert) {
    task.update({
      finishedAt: new Date(),
    });

    await FS.visit({ id: allocation.id, name: task.name });
    assert.ok(FS.hasEmptyState, 'Non-running task has no files');
    assert.ok(
      FS.emptyState.headline.includes('Task is not Running'),
      'Empty state explains the condition'
    );
  });
});
