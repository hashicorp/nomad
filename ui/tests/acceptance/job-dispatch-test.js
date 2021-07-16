import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import setupCodeMirror from 'nomad-ui/tests/helpers/codemirror';
import JobDispatch from 'nomad-ui/tests/pages/jobs/dispatch';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import { currentURL } from '@ember/test-helpers';

let job, namespace, managementToken, clientToken;

module('Acceptance | job dispatch (with namespace)', function(hooks) {
  setupApplicationTest(hooks);
  setupCodeMirror(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    // Required for placing allocations (a result of dispatching jobs)
    server.create('node');
    server.createList('namespace', 2);

    namespace = server.db.namespaces[0].name;
    job = server.create('job', 'parameterized', {
      status: 'running',
      namespaceId: namespace,
    });

    managementToken = server.create('token');
    clientToken = server.create('token');

    window.localStorage.nomadTokenSecret = managementToken.secretId;
  });

  test('it passes an accessibility audit', async function(assert) {
    await JobDispatch.visit({ id: job.id, namespace: namespace.name });
    await a11yAudit(assert);
  });

  test('the dispatch button is displayed with management token', async function(assert) {
    await JobDetail.visit({ id: job.id, namespace: namespace.name });
    assert.notOk(JobDetail.dispatchButton.isDisabled);
  });

  test('the dispatch button is displayed when allowed', async function(assert) {
    window.localStorage.nomadTokenSecret = clientToken.secretId;

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

    await JobDetail.visit({ id: job.id, namespace: namespace.name });
    assert.ok(JobDetail.dispatchButton.isDisabled);
  });

  test('all meta fields are displayed', async function(assert) {
    await JobDispatch.visit({ id: job.id, namespace: namespace.name });
    assert.equal(
      JobDispatch.metaFields.length,
      job.parameterizedJob.MetaOptional.length + job.parameterizedJob.MetaRequired.length
    );
  });

  test('required meta fields are properly indicated', async function(assert) {
    await JobDispatch.visit({ id: job.id, namespace: namespace.name });

    JobDispatch.metaFields.map(f => {
      if (job.parameterizedJob.MetaRequired.includes(f.field.id)) {
        assert.ok(f.label.includes('*'), `${f.label} contains required indicator.`);
      } else {
        assert.notOk(f.label.includes('*'), `${f.label} doesn't contain required indicator.`);
      }
    });
  });

  test('payload text area is hidden when forbidden', async function(assert) {
    job = server.create('job', 'parameterizedWithoutPayload', {
      status: 'running',
      namespaceId: namespace,
    });

    await JobDispatch.visit({ id: job.id, namespace: namespace.name });
    assert.ok(JobDispatch.payload.emptyMessage.isPresent);
    assert.notOk(JobDispatch.payload.editor.isPresent);
  });

  test('dispatch a job', async function(assert) {
    await JobDispatch.visit({ id: job.id, namespace: namespace.name });

    // Fill form.
    JobDispatch.metaFields.map(f => f.field.input('meta value'));
    JobDispatch.payload.editor.fillIn('payload');

    await JobDispatch.dispatchButton.click();

    const dispatchedJob = server.db.jobs.find(j => j.id.startsWith(`${job.id}/`));
    assert.ok(dispatchedJob);
    assert.equal(currentURL(), `/jobs/${dispatchedJob.id}`);
  });
});
