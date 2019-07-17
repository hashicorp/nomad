import { currentURL } from '@ember/test-helpers';
import { assign } from '@ember/polyfills';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import setupMirage from 'ember-cli-mirage/test-support/setup-mirage';
import setupCodeMirror from 'nomad-ui/tests/helpers/codemirror';
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
  setupCodeMirror(hooks);

  hooks.beforeEach(function() {
    // Required for placing allocations (a result of creating jobs)
    server.create('node');
  });

  test('visiting /jobs/run', async function(assert) {
    await JobRun.visit();

    assert.equal(currentURL(), '/jobs/run');
    assert.equal(document.title, 'Run a job - Nomad');
  });

  test('when submitting a job, the site redirects to the new job overview page', async function(assert) {
    const spec = jsonJob();

    await JobRun.visit();

    await JobRun.editor.editor.fillIn(spec);
    await JobRun.editor.plan();
    await JobRun.editor.run();
    assert.equal(
      currentURL(),
      `/jobs/${newJobName}`,
      `Redirected to the job overview page for ${newJobName}`
    );
  });

  test('when submitting a job to a different namespace, the redirect to the job overview page takes namespace into account', async function(assert) {
    const newNamespace = 'second-namespace';

    server.create('namespace', { id: newNamespace });
    const spec = jsonJob({ Namespace: newNamespace });

    await JobRun.visit();

    await JobRun.editor.editor.fillIn(spec);
    await JobRun.editor.plan();
    await JobRun.editor.run();
    assert.equal(
      currentURL(),
      `/jobs/${newJobName}?namespace=${newNamespace}`,
      `Redirected to the job overview page for ${newJobName} and switched the namespace to ${newNamespace}`
    );
  });
});
