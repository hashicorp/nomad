import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Exec from 'nomad-ui/tests/pages/exec';

module('Acceptance | exec', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    server.create('namespace');
    this.namespace = server.create('namespace');

    server.create('agent');
    server.create('node');

    this.job = server.create('job', { createAllocations: false, namespaceId: this.namespace.id });
    server.create('allocation', 'withTaskWithPorts', { clientStatus: 'running' });

    server.create('region', { id: 'global' });
    server.create('region', { id: 'region-2' });
  });

  test('/exec/:job should show the region, namespace, and job name', async function(assert) {
    await Exec.visit({ job: this.job.id, namespace: this.namespace.id, region: 'region-2' });

    assert.equal(document.title, 'Exec - region-2 - Nomad');

    assert.equal(Exec.header.region, this.job.region);
    assert.equal(Exec.header.namespace, this.job.namespace);
    assert.equal(Exec.header.job, this.job.name);
  });

  test('/exec/:job should show the task groups and tasks and allow task groups to be collapsed', async function(assert) {
    await Exec.visit({ job: this.job.id, namespace: this.namespace.id });

    assert.equal(Exec.taskGroups.length, this.job.task_groups.length);

    assert.equal(Exec.taskGroups[0].name, this.job.task_groups.models[0].name);
    assert.equal(Exec.taskGroups[0].tasks.length, this.job.task_groups.models[0].tasks.length);

    await Exec.taskGroups[0].click();
    assert.equal(Exec.taskGroups[0].tasks.length, 0);
    assert.ok(Exec.taskGroups[0].chevron.isRight);

    await Exec.taskGroups[0].click();
    assert.equal(Exec.taskGroups[0].tasks.length, this.job.task_groups.models[0].tasks.length);
    assert.ok(Exec.taskGroups[0].chevron.isDown);
  });
});
