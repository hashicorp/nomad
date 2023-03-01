// @ts-check
import { module, test } from 'qunit';
import { visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import percySnapshot from '@percy/ember';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';

module('Acceptance | job status panel', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  // let server;
  // let job;

  hooks.beforeEach(async function () {
    server.create('node');
  });

  test('Status panel lets you switch between Current and Historical', async function (assert) {
    assert.expect(2);
    let job = server.create('job', {
      status: 'running',
      datacenters: ['*'],
      type: 'service',
      createAllocations: true,
      withLotsOfAllocs: true,
    });

    await visit(`/jobs/${job.id}`);
    assert.dom('.job-status-panel').exists();
    await a11yAudit(assert);
    await percySnapshot(assert);
  });
});
