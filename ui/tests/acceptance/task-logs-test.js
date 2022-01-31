/* eslint-disable qunit/require-expect */
import { currentURL } from '@ember/test-helpers';
import { run } from '@ember/runloop';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import TaskLogs from 'nomad-ui/tests/pages/allocations/task/logs';

let allocation;
let task;

module('Acceptance | task logs', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('agent');
    server.create('node', 'forceIPv4');
    const job = server.create('job', { createAllocations: false });

    allocation = server.create('allocation', {
      jobId: job.id,
      clientStatus: 'running',
    });
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    run.later(run, run.cancelTimers, 1000);
    await TaskLogs.visit({ id: allocation.id, name: task.name });
  });

  test('it passes an accessibility audit', async function (assert) {
    await a11yAudit(assert);
  });

  test('/allocation/:id/:task_name/logs should have a log component', async function (assert) {
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}/${task.name}/logs`,
      'No redirect'
    );
    assert.ok(TaskLogs.hasTaskLog, 'Task log component found');
    assert.equal(document.title, `Task ${task.name} logs - Nomad`);
  });

  test('the stdout log immediately starts streaming', async function (assert) {
    const node = server.db.nodes.find(allocation.nodeId);
    const logUrlRegex = new RegExp(
      `${node.httpAddr}/v1/client/fs/logs/${allocation.id}`
    );
    assert.ok(
      server.pretender.handledRequests.filter((req) =>
        logUrlRegex.test(req.url)
      ).length,
      'Log requests were made'
    );
  });
});
