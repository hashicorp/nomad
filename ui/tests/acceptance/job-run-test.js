import { assign } from '@ember/polyfills';
import { currentURL } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
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

moduleForAcceptance('Acceptance | job run', {
  beforeEach() {
    // Required for placing allocations (a result of creating jobs)
    server.create('node');
  },
});

test('visiting /jobs/run', function(assert) {
  JobRun.visit();

  andThen(() => {
    assert.equal(currentURL(), '/jobs/run');
  });
});

test('when submitting a job, the site redirects to the new job overview page', function(assert) {
  const spec = jsonJob();

  JobRun.visit();

  andThen(() => {
    JobRun.editor.editor.fillIn(spec);
    JobRun.editor.plan();
  });

  andThen(() => {
    JobRun.editor.run();
  });
  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${newJobName}`,
      `Redirected to the job overview page for ${newJobName}`
    );
  });
});

test('when submitting a job to a different namespace, the redirect to the job overview page takes namespace into account', function(assert) {
  const newNamespace = 'second-namespace';

  server.create('namespace', { id: newNamespace });
  const spec = jsonJob({ Namespace: newNamespace });

  JobRun.visit();

  andThen(() => {
    JobRun.editor.editor.fillIn(spec);
    JobRun.editor.plan();
  });

  andThen(() => {
    JobRun.editor.run();
  });
  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${newJobName}?namespace=${newNamespace}`,
      `Redirected to the job overview page for ${newJobName} and switched the namespace to ${newNamespace}`
    );
  });
});
