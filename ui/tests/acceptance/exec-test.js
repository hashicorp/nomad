import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Exec from 'nomad-ui/tests/pages/exec';

module('Acceptance | exec', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    server.create('agent');
    server.create('node');

    this.job = server.create('job', { createAllocations: false });
    server.create('allocation', 'withTaskWithPorts', { clientStatus: 'running' });
  });

  test('/exec/:job should show the region, namespace, and job name', async function(assert) {
    server.create('namespace');
    const namespace = server.create('namespace');

    server.create('region', { id: 'global' });
    server.create('region', { id: 'region-2' });

    this.job = server.create('job', { createAllocations: false, namespaceId: namespace.id });

    await Exec.visit({ job: this.job.id, namespace: namespace.id, region: 'region-2' });

    assert.equal(document.title, 'Exec - region-2 - Nomad');

    assert.equal(Exec.header.region.text, this.job.region);
    assert.equal(Exec.header.namespace.text, this.job.namespace);
    assert.equal(Exec.header.job, this.job.name);
  });

  test('/exec/:job should not show region and namespace when there are none', async function(assert) {
    await Exec.visit({ job: this.job.id });

    assert.ok(Exec.header.region.isHidden);
    assert.ok(Exec.header.namespace.isHidden);
  });

  test('/exec/:job should show the task groups collapsed by default allow the tasks to be shown', async function(assert) {
    await Exec.visit({ job: this.job.id });

    assert.equal(Exec.taskGroups.length, this.job.task_groups.length);

    assert.equal(Exec.taskGroups[0].name, this.job.task_groups.models[0].name);
    assert.equal(Exec.taskGroups[0].tasks.length, 0);
    assert.ok(Exec.taskGroups[0].chevron.isRight);

    await Exec.taskGroups[0].click();
    assert.equal(Exec.taskGroups[0].tasks.length, this.job.task_groups.models[0].tasks.length);
    assert.ok(Exec.taskGroups[0].chevron.isDown);

    await Exec.taskGroups[0].click();
    assert.equal(Exec.taskGroups[0].tasks.length, 0);
  });

  test('/exec/:job should require selecting a task', async function(assert) {
    await Exec.visit({ job: this.job.id });

    assert.equal(
      window.execTerminal.buffer
        .getLine(0)
        .translateToString()
        .trim(),
      'Select a task to start your session.'
    );
  });
});
