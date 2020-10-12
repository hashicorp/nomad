import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import setupCodeMirror from 'nomad-ui/tests/helpers/codemirror';
import Definition from 'nomad-ui/tests/pages/jobs/job/definition';

let job;

module('Acceptance | job definition', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);
  setupCodeMirror(hooks);

  hooks.beforeEach(async function() {
    server.create('node');
    server.create('job');
    job = server.db.jobs[0];
    await Definition.visit({ id: job.id });
  });

  test('it passes an accessibility audit', async function(assert) {
    await a11yAudit(assert, 'scrollable-region-focusable');
  });

  test('visiting /jobs/:job_id/definition', async function(assert) {
    assert.equal(currentURL(), `/jobs/${job.id}/definition`);
    assert.equal(document.title, `Job ${job.name} definition - Nomad`);
  });

  test('the job definition page contains a json viewer component', async function(assert) {
    assert.ok(Definition.jsonViewer, 'JSON viewer found');
  });

  test('the job definition page requests the job to display in an unmutated form', async function(assert) {
    const jobURL = `/v1/job/${job.id}`;
    const jobRequests = server.pretender.handledRequests
      .map(req => req.url.split('?')[0])
      .filter(url => url === jobURL);
    assert.ok(jobRequests.length === 2, 'Two requests for the job were made');
  });

  test('the job definition can be edited', async function(assert) {
    assert.notOk(Definition.editor.isPresent, 'Editor is not shown on load');

    await Definition.edit();

    assert.ok(Definition.editor.isPresent, 'Editor is shown after clicking edit');
    assert.notOk(Definition.jsonViewer, 'Editor replaces the JSON viewer');
  });

  test('when in editing mode, the action can be canceled, showing the read-only definition again', async function(assert) {
    await Definition.edit();

    await Definition.editor.cancelEditing();
    assert.ok(Definition.jsonViewer, 'The JSON Viewer is back');
    assert.notOk(Definition.editor.isPresent, 'The editor is gone');
  });

  test('when in editing mode, the editor is prepopulated with the job definition', async function(assert) {
    const requests = server.pretender.handledRequests;
    const jobDefinition = requests.findBy('url', `/v1/job/${job.id}`).responseText;
    const formattedJobDefinition = JSON.stringify(JSON.parse(jobDefinition), null, 2);

    await Definition.edit();

    assert.equal(
      Definition.editor.editor.contents,
      formattedJobDefinition,
      'The editor already has the job definition in it'
    );
  });

  test('when changes are submitted, the site redirects to the job overview page', async function(assert) {
    await Definition.edit();

    await Definition.editor.plan();
    await Definition.editor.run();
    assert.equal(currentURL(), `/jobs/${job.id}`, 'Now on the job overview page');
  });

  test('when the job for the definition is not found, an error message is shown, but the URL persists', async function(assert) {
    await Definition.visit({ id: 'not-a-real-job' });

    assert.equal(
      server.pretender.handledRequests
        .filter(request => !request.url.includes('policy'))
        .findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(currentURL(), '/jobs/not-a-real-job/definition', 'The URL persists');
    assert.ok(Definition.error.isPresent, 'Error message is shown');
    assert.equal(Definition.error.title, 'Not Found', 'Error message is for 404');
  });
});
