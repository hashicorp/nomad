import { assign } from '@ember/polyfills';
import { currentURL } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import JobRun from 'nomad-ui/tests/pages/jobs/run';

const newJobName = 'new-job';

const jsonJob = overrides => {
  return JSON.stringify(
    assign(
      {},
      {
        Name: newJobName,
        Namespace: 'default',
        Datacenters: ['dc1'],
        Priority: 50,
        TaskGroups: {
          redis: {
            Tasks: {
              redis: {
                Driver: 'docker',
              },
            },
          },
        },
      },
      overrides
    ),
    null,
    2
  );
};

const hclJob = () => `
job "${newJobName}" {
  namespace = "default"
  datacenters = ["dc1"]

  task "redis" {
    driver = "docker"
  }
}
`;

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

test('the page has an editor and an explanation popup', function(assert) {
  JobRun.visit();

  andThen(() => {
    assert.ok(JobRun.editorHelp.isPresent, 'Editor explanation popup is present');
    assert.ok(JobRun.editor.isPresent, 'Editor is present');
  });
});

test('the explanation popup can be dismissed', function(assert) {
  JobRun.visit();

  andThen(() => {
    JobRun.editorHelp.dismiss();
  });

  andThen(() => {
    assert.notOk(JobRun.editorHelp.isPresent, 'Editor explanation popup is gone');
    assert.equal(
      window.localStorage.nomadMessageJobEditor,
      'false',
      'Dismissal is persisted in localStorage'
    );
  });
});

test('the explanation popup is not shown once the dismissal state is set in localStorage', function(assert) {
  window.localStorage.nomadMessageJobEditor = 'false';

  JobRun.visit();

  andThen(() => {
    assert.notOk(JobRun.editorHelp.isPresent, 'Editor explanation popup is gone');
  });
});

test('submitting a json job skips the parse endpoint', function(assert) {
  const spec = jsonJob();

  JobRun.visit();

  andThen(() => {
    JobRun.editor.fillIn(spec);
    JobRun.plan();
  });

  andThen(() => {
    const requests = server.pretender.handledRequests.mapBy('url');
    assert.notOk(requests.includes('/v1/jobs/parse'), 'JSON job spec is not parsed');
    assert.ok(requests.includes(`/v1/job/${newJobName}/plan`), 'JSON job spec is still planned');
  });
});

test('submitting an hcl job requires the parse endpoint', function(assert) {
  const spec = hclJob();

  JobRun.visit();

  andThen(() => {
    JobRun.editor.fillIn(spec);
    JobRun.plan();
  });

  andThen(() => {
    const requests = server.pretender.handledRequests.mapBy('url');
    assert.ok(requests.includes('/v1/jobs/parse'), 'HCL job spec is parsed first');
    assert.ok(requests.includes(`/v1/job/${newJobName}/plan`), 'HCL job spec is planned');
    assert.ok(
      requests.indexOf('/v1/jobs/parse') < requests.indexOf(`/v1/job/${newJobName}/plan`),
      'Parse comes before Plan'
    );
  });
});

test('when a job is successfully parsed and planned, the plan is shown to the user', function(assert) {
  const spec = hclJob();

  JobRun.visit();

  andThen(() => {
    JobRun.editor.fillIn(spec);
    JobRun.plan();
  });

  andThen(() => {
    assert.ok(JobRun.planOutput, 'The plan is outputted');
    assert.notOk(JobRun.editor.isPresent, 'The editor is replaced with the plan output');
    assert.ok(JobRun.planHelp.isPresent, 'The plan explanation popup is shown');
  });
});

test('from the plan screen, the cancel button goes back to the editor with the job still in tact', function(assert) {
  const spec = hclJob();

  JobRun.visit();

  andThen(() => {
    JobRun.editor.fillIn(spec);
    JobRun.plan();
  });

  andThen(() => {
    JobRun.cancel();
  });

  andThen(() => {
    assert.ok(JobRun.editor.isPresent, 'The editor is shown again');
    assert.notOk(JobRun.planOutpu, 'The plan is gone');
    assert.equal(JobRun.editor.contents, spec, 'The spec that was planned is still in the editor');
  });
});

test('from the plan screen, the submit button submits the job and redirects to the job overview page', function(assert) {
  const spec = hclJob();

  JobRun.visit();

  andThen(() => {
    JobRun.editor.fillIn(spec);
    JobRun.plan();
  });

  andThen(() => {
    JobRun.run();
  });

  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${newJobName}`,
      `Redirected to the job overview page for ${newJobName}`
    );

    const runRequest = server.pretender.handledRequests.find(
      req => req.method === 'POST' && req.url === '/v1/jobs'
    );
    const planRequest = server.pretender.handledRequests.find(
      req => req.method === 'POST' && req.url === '/v1/jobs/parse'
    );

    assert.ok(runRequest, 'A POST request was made to run the new job');
    assert.deepEqual(
      JSON.parse(runRequest.requestBody).Job,
      JSON.parse(planRequest.responseText),
      'The Job payload parameter is equivalent to the result of the parse request'
    );
  });
});

test('when parse fails, the parse error message is shown', function(assert) {
  const spec = hclJob();

  const errorMessage = 'Parse Failed!! :o';
  server.pretender.post('/v1/jobs/parse', () => [400, {}, errorMessage]);

  JobRun.visit();

  andThen(() => {
    JobRun.editor.fillIn(spec);
    JobRun.plan();
  });

  andThen(() => {
    assert.notOk(JobRun.planError.isPresent, 'Plan error is not shown');
    assert.notOk(JobRun.runError.isPresent, 'Run error is not shown');

    assert.ok(JobRun.parseError.isPresent, 'Parse error is shown');
    assert.equal(
      JobRun.parseError.message,
      errorMessage,
      'The error message from the server is shown in the error in the UI'
    );
  });
});

test('when plan fails, the plan error message is shown', function(assert) {
  const spec = hclJob();

  const errorMessage = 'Parse Failed!! :o';
  server.pretender.post(`/v1/job/${newJobName}/plan`, () => [400, {}, errorMessage]);

  JobRun.visit();

  andThen(() => {
    JobRun.editor.fillIn(spec);
    JobRun.plan();
  });

  andThen(() => {
    assert.notOk(JobRun.parseError.isPresent, 'Parse error is not shown');
    assert.notOk(JobRun.runError.isPresent, 'Run error is not shown');

    assert.ok(JobRun.planError.isPresent, 'Plan error is shown');
    assert.equal(
      JobRun.planError.message,
      errorMessage,
      'The error message from the server is shown in the error in the UI'
    );
  });
});

test('when run fails, the run error message is shown', function(assert) {
  const spec = hclJob();

  const errorMessage = 'Parse Failed!! :o';
  server.pretender.post('/v1/jobs', () => [400, {}, errorMessage]);

  JobRun.visit();

  andThen(() => {
    JobRun.editor.fillIn(spec);
    JobRun.plan();
  });

  andThen(() => {
    JobRun.run();
  });

  andThen(() => {
    assert.notOk(JobRun.planError.isPresent, 'Plan error is not shown');
    assert.notOk(JobRun.parseError.isPresent, 'Parse error is not shown');

    assert.ok(JobRun.runError.isPresent, 'Run error is shown');
    assert.equal(
      JobRun.runError.message,
      errorMessage,
      'The error message from the server is shown in the error in the UI'
    );
  });
});

test('when submitting a job to a different namespace, the redirect to the job overview page takes namespace into account', function(assert) {
  const newNamespace = 'second-namespace';

  server.create('namespace', { id: newNamespace });
  const spec = jsonJob({ Namespace: newNamespace });

  JobRun.visit();

  andThen(() => {
    JobRun.editor.fillIn(spec);
    JobRun.plan();
  });

  andThen(() => {
    JobRun.run();
  });
  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${newJobName}?namespace=${newNamespace}`,
      `Redirected to the job overview page for ${newJobName} and switched the namespace to ${newNamespace}`
    );
  });
});
