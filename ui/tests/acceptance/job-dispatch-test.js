import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import JobDispatch from 'nomad-ui/tests/pages/jobs/dispatch';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';
import { pauseTest } from '@ember/test-helpers';

import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import { setup } from 'qunit-dom';

let job, managementToken, clientToken;

module('Acceptance | job dispatch (with namespace)', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    // Required for placing allocations (a result of dispatching jobs)
    server.create('node');
    server.createList('namespace', 2);

    job = server.create('job', 'parameterized', {
      status: 'running',
      namespaceId: server.db.namespaces[0].name,
    });

    managementToken = server.create('token');
    clientToken = server.create('token');

    window.localStorage.nomadTokenSecret = managementToken.secretId;
  });

  test('it passes an accessibility audit', async function(assert) {
    const namespace = server.db.namespaces.find(job.namespaceId);
    await JobDispatch.visit({ id: job.id, namespace: namespace.name });
    await a11yAudit(assert);
  });

  test('the dispatch button is displayed with management token', async function(assert) {
    const namespace = server.db.namespaces.find(job.namespaceId);
    await JobDetail.visit({ id: job.id, namespace: namespace.name });
    assert.notOk(JobDetail.dispatchButton.isDisabled);
  });

  test('the dispatch button is displayed when allowed', async function(assert) {
    window.localStorage.nomadTokenSecret = clientToken.secretId;

    const namespace = server.db.namespaces.find(job.namespaceId);
    const policy = server.create('policy', {
      id: 'dispatch',
      name: 'dispatch',
      rulesJSON: {
        Namespaces: [
          {
            Name: namespace.name,
            Capabilities: ['list-jobs', 'dispatch-job'],
          },
        ],
      },
    });

    clientToken.policyIds = [policy.id];
    clientToken.save();

    await JobDetail.visit({ id: job.id, namespace: namespace.name });
    assert.notOk(JobDetail.dispatchButton.isDisabled);

    // Reset clientToken policies.
    clientToken.policyIds = [];
    clientToken.save();
  });

  test('the dispatch button is not displayed when not allowed', async function(assert) {
    window.localStorage.nomadTokenSecret = clientToken.secretId;

    const namespace = server.db.namespaces.find(job.namespaceId);
    await JobDetail.visit({ id: job.id, namespace: namespace.name });
    assert.ok(JobDetail.dispatchButton.isDisabled);
  });
});
