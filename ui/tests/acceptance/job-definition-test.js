/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { click, currentURL } from '@ember/test-helpers';
import percySnapshot from '@percy/ember';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import setupCodeMirror from 'nomad-ui/tests/helpers/codemirror';
import Definition from 'nomad-ui/tests/pages/jobs/job/definition';
import { JOB_JSON } from 'nomad-ui/tests/utils/generate-raw-json-job';

let job;

module('Acceptance | job definition', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);
  setupCodeMirror(hooks);

  hooks.beforeEach(async function () {
    server.create('node-pool');
    server.create('node');
    server.create('job');
    job = server.db.jobs[0];
    await Definition.visit({ id: job.id });
  });

  test('it passes an accessibility audit', async function (assert) {
    assert.expect(1);

    await a11yAudit(assert, 'scrollable-region-focusable');
  });

  test('visiting /jobs/:job_id/definition', async function (assert) {
    assert.equal(currentURL(), `/jobs/${job.id}/definition`);
    assert.equal(document.title, `Job ${job.name} definition - Nomad`);
  });

  test('the job definition page starts in read-only view', async function (assert) {
    assert.dom('[data-test-job-spec-view]').exists('Read-only Editor appears.');
  });

  test('the job definition page requests the job to display in an unmutated form', async function (assert) {
    const jobURL = `/v1/job/${job.id}`;
    const jobRequests = server.pretender.handledRequests
      .map((req) => req.url.split('?')[0])
      .filter((url) => url === jobURL);
    assert.strictEqual(
      jobRequests.length,
      2,
      'Two requests for the job were made'
    );
  });

  test('the job definition can be edited', async function (assert) {
    assert
      .dom('[data-test-job-editor]')
      .doesNotExist('Editor not shown on load.');

    await Definition.edit();

    assert.ok(
      Definition.editor.isPresent,
      'Editor is shown after clicking edit'
    );
    assert.notOk(Definition.jsonViewer, 'Editor replaces the JSON viewer');
  });

  test('when in editing mode, the action can be canceled, showing the read-only definition again', async function (assert) {
    await Definition.edit();

    await Definition.editor.cancelEditing();
    assert.dom('[data-test-job-spec-view]').exists('Read-only Editor appears.');
    assert.dom('[data-test-job-editor]').doesNotExist('The editor is gone');
  });

  test('when in editing mode, the editor is prepopulated with the job definition', async function (assert) {
    assert.expect(1);

    const requests = server.pretender.handledRequests;
    const jobSubmission = requests.findBy(
      'url',
      `/v1/job/${job.id}/submission?version=1`
    ).responseText;
    const formattedJobDefinition = JSON.parse(jobSubmission).Source;

    await Definition.edit();
    await percySnapshot(assert);

    assert.equal(
      Definition.editor.editor.contents,
      formattedJobDefinition,
      'The editor already has the job definition in it'
    );
  });

  test('when changes are submitted, the site redirects to the job overview page', async function (assert) {
    await Definition.edit();

    const cm = getCodeMirrorInstance(['data-test-editor']);
    cm.setValue(`{}`);

    await click('[data-test-plan]');
    await Definition.editor.run();
    assert.equal(
      currentURL(),
      `/jobs/${job.id}@default`,
      'Now on the job overview page'
    );
  });

  test('when the job for the definition is not found, an error message is shown, but the URL persists', async function (assert) {
    assert.expect(4);
    await Definition.visit({ id: 'not-a-real-job' });
    await percySnapshot(assert);

    assert.equal(
      server.pretender.handledRequests
        .filter((request) => !request.url.includes('policy'))
        .findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(
      currentURL(),
      '/jobs/not-a-real-job/definition',
      'The URL persists'
    );
    assert.ok(Definition.error.isPresent, 'Error message is shown');
    assert.equal(
      Definition.error.title,
      'Not Found',
      'Error message is for 404'
    );
  });
});

module('Acceptance | job definition | full specification', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);
  setupCodeMirror(hooks);

  hooks.beforeEach(async function () {
    server.create('node-pool');
    server.create('node');
    server.create('job');
    job = server.db.jobs[0];
  });

  test('it allows users to select between full specification and JSON definition', async function (assert) {
    assert.expect(3);
    const specification_response = {
      Format: 'hcl2',
      JobID: 'example',
      JobIndex: 223,
      Namespace: 'default',
      Source:
        'variable "datacenter" {\n  description = "The datacenter to run the job in"\n  type        = string\n  default     = "dc1"\n}\n\njob "example" {\n  datacenters = [var.datacenter]\n\n  group "example-group" {\n    task "example-task" {\n      driver = "docker"\n\n      config {\n        image = "redis:3.2"\n      }\n\n      resources {\n        cpu    = 500\n        memory = 256\n      }\n    }\n  }\n}\n',
      VariableFlags: { datacenter: 'dc2' },
      Variables: '',
      Version: 0,
    };
    server.get('/job/:id', () => JOB_JSON);
    server.get('/job/:id/submission', () => specification_response);

    await Definition.visit({ id: job.id });
    await percySnapshot(assert);

    assert
      .dom('[data-test-select="job-spec"]')
      .exists('A select button exists and defaults to full definition');
    let codeMirror = getCodeMirrorInstance('[data-test-editor]');
    assert.equal(
      codeMirror.getValue(),
      specification_response.Source,
      'Shows the full definition as written by the user'
    );

    await click('[data-test-select-full]');
    codeMirror = getCodeMirrorInstance('[data-test-editor]');
    assert.propContains(JSON.parse(codeMirror.getValue()), JOB_JSON);
  });
});
