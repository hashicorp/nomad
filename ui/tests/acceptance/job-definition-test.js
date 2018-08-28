import { currentURL } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import Definition from 'nomad-ui/tests/pages/jobs/job/definition';

let job;

moduleForAcceptance('Acceptance | job definition', {
  beforeEach() {
    server.create('node');
    server.create('job');
    job = server.db.jobs[0];
    Definition.visit({ id: job.id });
  },
});

test('visiting /jobs/:job_id/definition', function(assert) {
  assert.equal(currentURL(), `/jobs/${job.id}/definition`);
});

test('the job definition page contains a json viewer component', function(assert) {
  assert.ok(Definition.jsonViewer, 'JSON viewer found');
});

test('the job definition page requests the job to display in an unmutated form', function(assert) {
  const jobURL = `/v1/job/${job.id}`;
  const jobRequests = server.pretender.handledRequests
    .map(req => req.url.split('?')[0])
    .filter(url => url === jobURL);
  assert.ok(jobRequests.length === 2, 'Two requests for the job were made');
});

test('the job definition can be edited', function(assert) {
  assert.notOk(Definition.editor.isPresent, 'Editor is not shown on load');

  Definition.edit();

  andThen(() => {
    assert.ok(Definition.editor.isPresent, 'Editor is shown after clicking edit');
    assert.notOk(Definition.jsonViewer, 'Editor replaces the JSON viewer');
  });
});

test('when in editing mode, the action can be canceled, showing the read-only definition again', function(assert) {
  Definition.edit();

  andThen(() => {
    Definition.editor.cancelEditing();
  });

  andThen(() => {
    assert.ok(Definition.jsonViewer, 'The JSON Viewer is back');
    assert.notOk(Definition.editor.isPresent, 'The editor is gone');
  });
});

test('when in editing mode, the editor is prepopulated with the job definition', function(assert) {
  const requests = server.pretender.handledRequests;
  const jobDefinition = requests.findBy('url', `/v1/job/${job.id}`).responseText;
  const formattedJobDefinition = JSON.stringify(JSON.parse(jobDefinition), null, 2);

  Definition.edit();

  andThen(() => {
    assert.equal(
      Definition.editor.editor.contents,
      formattedJobDefinition,
      'The editor already has the job definition in it'
    );
  });
});

test('when changes are submitted, the site redirects to the job overview page', function(assert) {
  Definition.edit();

  andThen(() => {
    Definition.editor.plan();
    Definition.editor.run();
  });

  andThen(() => {
    assert.equal(currentURL(), `/jobs/${job.id}`, 'Now on the job overview page');
  });
});
