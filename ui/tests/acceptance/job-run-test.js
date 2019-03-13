import { currentURL } from '@ember/test-helpers';
import { assign } from '@ember/polyfills';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import setupMirage from 'ember-cli-mirage/test-support/setup-mirage';
import JobRun from 'nomad-ui/tests/pages/jobs/run';

const newJobName = 'new-job';
const newJobTaskGroupName = 'redis';

const jsonJob = overrides => {
  return JSON.stringify(
    assign(
      {},
      {
        Name: newJobName,
        Namespace: 'default',
        Datacenters: ['dc1'],
        Priority: 50,
        TaskGroups: [
          {
            Name: newJobTaskGroupName,
            Tasks: [
              {
                Name: 'redis',
                Driver: 'docker',
              },
            ],
          },
        ],
      },
      overrides
    ),
    null,
    2
  );
};

module('Acceptance | job run', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    // Required for placing allocations (a result of creating jobs)
    server.create('node');
  });

  test('visiting /jobs/run', function(assert) {
    JobRun.visit();

    assert.equal(currentURL(), '/jobs/run');
  });

  test('when submitting a job, the site redirects to the new job overview page', function(assert) {
    const spec = jsonJob();

    JobRun.visit();

    JobRun.editor.editor.fillIn(spec);
    JobRun.editor.plan();
    JobRun.editor.run();
    assert.equal(
      currentURL(),
      `/jobs/${newJobName}`,
      `Redirected to the job overview page for ${newJobName}`
    );
  });

  test('when submitting a job to a different namespace, the redirect to the job overview page takes namespace into account', function(assert) {
    const newNamespace = 'second-namespace';

    server.create('namespace', { id: newNamespace });
    const spec = jsonJob({ Namespace: newNamespace });

    JobRun.visit();

    JobRun.editor.editor.fillIn(spec);
    JobRun.editor.plan();
    JobRun.editor.run();
    assert.equal(
      currentURL(),
      `/jobs/${newJobName}?namespace=${newNamespace}`,
      `Redirected to the job overview page for ${newJobName} and switched the namespace to ${newNamespace}`
    );
  });
});
