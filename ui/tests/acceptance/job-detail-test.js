import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { selectChoose } from 'ember-power-select/test-support';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import moduleForJob from 'nomad-ui/tests/helpers/module-for-job';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';
import JobsList from 'nomad-ui/tests/pages/jobs/list';

moduleForJob('Acceptance | job detail (batch)', 'allocations', () =>
  server.create('job', { type: 'batch', shallow: true })
);
moduleForJob('Acceptance | job detail (system)', 'allocations', () =>
  server.create('job', { type: 'system', shallow: true })
);
moduleForJob('Acceptance | job detail (periodic)', 'children', () =>
  server.create('job', 'periodic', { shallow: true })
);

moduleForJob('Acceptance | job detail (parameterized)', 'children', () =>
  server.create('job', 'parameterized', { shallow: true })
);

moduleForJob('Acceptance | job detail (periodic child)', 'allocations', () => {
  const parent = server.create('job', 'periodic', { childrenCount: 1, shallow: true });
  return server.db.jobs.where({ parentId: parent.id })[0];
});

moduleForJob('Acceptance | job detail (parameterized child)', 'allocations', () => {
  const parent = server.create('job', 'parameterized', { childrenCount: 1, shallow: true });
  return server.db.jobs.where({ parentId: parent.id })[0];
});

moduleForJob(
  'Acceptance | job detail (service)',
  'allocations',
  () => server.create('job', { type: 'service' }),
  {
    'the subnav links to deployment': async (job, assert) => {
      await JobDetail.tabFor('deployments').visit();
      assert.equal(currentURL(), `/jobs/${job.id}/deployments`);
    },
    'when the job is not found, an error message is shown, but the URL persists': async (
      job,
      assert
    ) => {
      await JobDetail.visit({ id: 'not-a-real-job' });

      assert.equal(
        server.pretender.handledRequests
          .filter(request => !request.url.includes('policy'))
          .findBy('status', 404).url,
        '/v1/job/not-a-real-job',
        'A request to the nonexistent job is made'
      );
      assert.equal(currentURL(), '/jobs/not-a-real-job', 'The URL persists');
      assert.ok(JobDetail.error.isPresent, 'Error message is shown');
      assert.equal(JobDetail.error.title, 'Not Found', 'Error message is for 404');
    },
  }
);

module('Acceptance | job detail (with namespaces)', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  let job, clientToken;

  hooks.beforeEach(function() {
    server.createList('namespace', 2);
    server.create('node');
    job = server.create('job', {
      type: 'service',
      status: 'running',
      namespaceId: server.db.namespaces[1].name,
    });
    server.createList('job', 3, { namespaceId: server.db.namespaces[0].name });

    server.create('token');
    clientToken = server.create('token');
  });

  test('it passes an accessibility audit', async function(assert) {
    const namespace = server.db.namespaces.find(job.namespaceId);
    await JobDetail.visit({ id: job.id, namespace: namespace.name });
    await a11yAudit(assert);
  });

  test('when there are namespaces, the job detail page states the namespace for the job', async function(assert) {
    const namespace = server.db.namespaces.find(job.namespaceId);
    await JobDetail.visit({ id: job.id, namespace: namespace.name });

    assert.ok(JobDetail.statFor('namespace').text, 'Namespace included in stats');
  });

  test('when switching namespaces, the app redirects to /jobs with the new namespace', async function(assert) {
    const namespace = server.db.namespaces.find(job.namespaceId);
    const otherNamespace = server.db.namespaces.toArray().find(ns => ns !== namespace).name;
    const label = otherNamespace === 'default' ? 'Default Namespace' : otherNamespace;

    await JobDetail.visit({ id: job.id, namespace: namespace.name });

    // TODO: Migrate to Page Objects
    await selectChoose('[data-test-namespace-switcher]', label);
    assert.equal(currentURL().split('?')[0], '/jobs', 'Navigated to /jobs');

    const jobs = server.db.jobs
      .where({ namespace: otherNamespace })
      .sortBy('modifyIndex')
      .reverse();

    assert.equal(JobsList.jobs.length, jobs.length, 'Shows the right number of jobs');
    JobsList.jobs.forEach((jobRow, index) => {
      assert.equal(jobRow.name, jobs[index].name, `Job ${index} is right`);
    });
  });

  test('the exec button state can change between namespaces', async function(assert) {
    const job1 = server.create('job', {
      status: 'running',
      namespaceId: server.db.namespaces[0].id,
    });
    const job2 = server.create('job', {
      status: 'running',
      namespaceId: server.db.namespaces[1].id,
    });

    window.localStorage.nomadTokenSecret = clientToken.secretId;

    const policy = server.create('policy', {
      id: 'something',
      name: 'something',
      rulesJSON: {
        Namespaces: [
          {
            Name: job1.namespaceId,
            Capabilities: ['list-jobs', 'alloc-exec'],
          },
          {
            Name: job2.namespaceId,
            Capabilities: ['list-jobs'],
          },
        ],
      },
    });

    clientToken.policyIds = [policy.id];
    clientToken.save();

    await JobDetail.visit({ id: job1.id });
    assert.notOk(JobDetail.execButton.isDisabled);

    const secondNamespace = server.db.namespaces[1];
    await JobDetail.visit({ id: job2.id, namespace: secondNamespace.name });
    assert.ok(JobDetail.execButton.isDisabled);
  });

  test('the anonymous policy is fetched to check whether to show the exec button', async function(assert) {
    window.localStorage.removeItem('nomadTokenSecret');

    server.create('policy', {
      id: 'anonymous',
      name: 'anonymous',
      rulesJSON: {
        Namespaces: [
          {
            Name: 'default',
            Capabilities: ['list-jobs', 'alloc-exec'],
          },
        ],
      },
    });

    await JobDetail.visit({ id: job.id, namespace: server.db.namespaces[1].name });
    assert.notOk(JobDetail.execButton.isDisabled);
  });
});
