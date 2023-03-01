// @ts-check
import { module, test } from 'qunit';
import { visit, currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import percySnapshot from '@percy/ember';


module('Acceptance | job status panel', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  // let server;
  // let job;

  hooks.beforeEach(async function () {
    server.create('node');

    // if (!job.namespace || job.namespace === 'default') {
    //   await JobDetail.visit({ id: job.id });
    // } else {
    //   await JobDetail.visit({ id: `${job.id}@${job.namespace}` });
    // }

    // const hasClientStatus = ['system', 'sysbatch'].includes(job.type);
    // if (context === 'allocations' && hasClientStatus) {
    //   await click("[data-test-accordion-summary-chart='allocation-status']");
    // }
    // const hasJobStatusPanel = ['service'].includes(job.type);
    // if (hasJobStatusPanel) {
    //   await JobDetail.statusModes.historical.click();
    // }
  });


  test('Status panel lets you switch between Current and Historical', async function(assert) {
    let job = server.create('job', {
      status: 'running',
      datacenters: ['*'],
      type: 'service',
      createAllocations: true,
      withLotsOfAllocs: true,
    });

    await visit(`/jobs/${job.id}`);
    assert.dom('.job-status-panel').exists();
    await percySnapshot(assert);
  });
});
