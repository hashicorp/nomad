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

  test('/exec/:job should show the task groups and tasks', async function(assert) {
    await Exec.visit({ job: this.job.id });

    assert.equal(document.title, 'Exec - Nomad');

    assert.equal(Exec.taskGroups.length, this.job.task_groups.length);

    assert.equal(Exec.taskGroups[0].name, this.job.task_groups.models[0].name);
    assert.equal(Exec.taskGroups[0].tasks.length, this.job.task_groups.models[0].tasks.length);
  });
});
