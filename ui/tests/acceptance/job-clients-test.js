/* eslint-disable */
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Clients from 'nomad-ui/tests/pages/jobs/job/clients';

let job;
module('Acceptance | job clients', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    server.create('node');

    const clients = server.createList('node', 12, {
      datacenter: 'dc1',
      status: 'ready',
    });
    // Job with 1 task group.
    job = server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type: 'sysbatch',
      resourceSpec: ['M: 256, C: 500'],
      createAllocations: false,
    });
    clients.forEach(c => {
      server.create('allocation', { jobId: job.id, nodeId: c.id });
    });
    console.log(
      'mirage nodes\n\n\n',
      clients.map(c => c.id)
    );
  });

  test('it passes an accessibility audit', async function(assert) {
    await Clients.visit({ id: job.id });
    await this.pauseTest();
    await a11yAudit(assert);
  });
});
