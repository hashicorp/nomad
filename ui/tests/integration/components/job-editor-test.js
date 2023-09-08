/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { assign } from '@ember/polyfills';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { create } from 'ember-cli-page-object';
import sinon from 'sinon';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import jobEditor from 'nomad-ui/tests/pages/components/job-editor';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import setupCodeMirror from 'nomad-ui/tests/helpers/codemirror';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

const Editor = create(jobEditor());

module('Integration | Component | job-editor', function (hooks) {
  setupRenderingTest(hooks);
  setupCodeMirror(hooks);

  hooks.beforeEach(async function () {
    window.localStorage.clear();

    fragmentSerializerInitializer(this.owner);

    this.store = this.owner.lookup('service:store');
    this.server = startMirage();

    // Required for placing allocations (a result of creating jobs)
    this.server.create('node-pool');
    this.server.create('node');
  });

  hooks.afterEach(async function () {
    this.server.shutdown();
  });

  const newJobName = 'new-job';
  const newJobTaskGroupName = 'redis';
  const jsonJob = (overrides) => {
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

  const hclJob = () => `
  job "${newJobName}" {
    namespace = "default"
    datacenters = ["dc1"]

    task "${newJobTaskGroupName}" {
      driver = "docker"
    }
  }
  `;

  const commonTemplate = hbs`
    <JobEditor
      @job={{job}}
      @context={{context}}
      @onSubmit={{onSubmit}}
      @handleSaveAsTemplate={{handleSaveAsTemplate}}
    />
  `;

  const renderNewJob = async (component, job) => {
    component.setProperties({
      job,
      onSubmit: sinon.spy(),
      handleSaveAsTemplate: sinon.spy(),
      context: 'new',
    });
    await component.render(commonTemplate);
  };

  const planJob = async (spec) => {
    const cm = getCodeMirrorInstance(['data-test-editor']);
    cm.setValue(spec);
    await Editor.plan();
  };

  test('the default state is an editor with an explanation popup', async function (assert) {
    assert.expect(2);

    const job = await this.store.createRecord('job');

    await renderNewJob(this, job);
    assert.ok('[data-test-job-editor]', 'Editor is present');

    await componentA11yAudit(this.element, assert);
  });

  test('submitting a json job skips the parse endpoint', async function (assert) {
    const spec = jsonJob();
    const job = await this.store.createRecord('job');

    await renderNewJob(this, job);

    const cm = getCodeMirrorInstance(['data-test-editor']);
    cm.setValue(spec);
    await Editor.plan();

    const requests = this.server.pretender.handledRequests.mapBy('url');
    assert.notOk(
      requests.includes('/v1/jobs/parse'),
      'JSON job spec is not parsed'
    );
    assert.ok(
      requests.includes(`/v1/job/${newJobName}/plan`),
      'JSON job spec is still planned'
    );
  });

  test('submitting an hcl job requires the parse endpoint', async function (assert) {
    const spec = hclJob();
    const job = await this.store.createRecord('job');

    await renderNewJob(this, job);

    await planJob(spec);
    const requests = this.server.pretender.handledRequests.mapBy('url');
    assert.ok(
      requests.includes('/v1/jobs/parse?namespace=*'),
      'HCL job spec is parsed first'
    );
    assert.ok(
      requests.includes(`/v1/job/${newJobName}/plan`),
      'HCL job spec is planned'
    );
    assert.ok(
      requests.indexOf('/v1/jobs/parse') <
        requests.indexOf(`/v1/job/${newJobName}/plan`),
      'Parse comes before Plan'
    );
  });

  test('when a job is successfully parsed and planned, the plan is shown to the user', async function (assert) {
    assert.expect(4);

    const spec = hclJob();
    const job = await this.store.createRecord('job');

    await renderNewJob(this, job);

    await planJob(spec);
    assert.ok(Editor.planOutput, 'The plan is outputted');
    assert.notOk(
      Editor.editor.isPresent,
      'The editor is replaced with the plan output'
    );
    assert
      .dom('[data-test-plan-help-title]')
      .exists('The plan explanation popup is shown');

    await componentA11yAudit(this.element, assert);
  });

  test('from the plan screen, the cancel button goes back to the editor with the job still in tact', async function (assert) {
    const spec = hclJob();
    const job = await this.store.createRecord('job');

    await renderNewJob(this, job);

    await planJob(spec);
    await Editor.cancel();
    assert.ok(Editor.editor.isPresent, 'The editor is shown again');
    assert.equal(
      Editor.editor.contents,
      spec,
      'The spec that was planned is still in the editor'
    );
  });

  test('when parse fails, the parse error message is shown', async function (assert) {
    assert.expect(5);

    const spec = hclJob();
    const errorMessage = 'Parse Failed!! :o';
    const job = await this.store.createRecord('job');

    this.server.pretender.post('/v1/jobs/parse', () => [400, {}, errorMessage]);

    await renderNewJob(this, job);

    await planJob(spec);
    assert
      .dom('[data-test-error="plan"]')
      .doesNotExist('Plan error is not shown');
    assert
      .dom('[data-test-error="run"]')
      .doesNotExist('Run error is not shown');

    assert.ok(Editor.parseError.isPresent, 'Parse error is shown');
    assert.equal(
      Editor.parseError.message,
      errorMessage,
      'The error message from the server is shown in the error in the UI'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('when plan fails, the plan error message is shown', async function (assert) {
    assert.expect(5);

    const spec = hclJob();
    const errorMessage = 'Plan Failed!! :o';
    const job = await this.store.createRecord('job');

    this.server.pretender.post(`/v1/job/${newJobName}/plan`, () => [
      400,
      {},
      errorMessage,
    ]);

    await renderNewJob(this, job);

    await planJob(spec);
    assert
      .dom('[data-test-error="parse"]')
      .doesNotExist('Parse error is not shown');
    assert
      .dom('[data-test-error="run"]')
      .doesNotExist('Run error is not shown');

    assert.ok(Editor.planError.isPresent, 'Plan error is shown');
    assert.equal(
      Editor.planError.message,
      errorMessage,
      'The error message from the server is shown in the error in the UI'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('when run fails, the run error message is shown', async function (assert) {
    assert.expect(5);

    const spec = hclJob();
    const errorMessage = 'Run Failed!! :o';
    const job = await this.store.createRecord('job');

    this.server.pretender.post('/v1/jobs', () => [400, {}, errorMessage]);

    await renderNewJob(this, job);

    await planJob(spec);
    await Editor.run();

    assert
      .dom('[data-test-error="plan"]')
      .doesNotExist('Plan error is not shown');
    assert
      .dom('[data-test-error="parse"]')
      .doesNotExist('Parse error is not shown');

    assert.dom('[data-test-error="run"]').exists('Run error is shown');
    assert.equal(
      Editor.runError.message,
      errorMessage,
      'The error message from the server is shown in the error in the UI'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('when the scheduler dry-run has warnings, the warnings are shown to the user', async function (assert) {
    assert.expect(4);

    const spec = jsonJob({ Unschedulable: true });
    const job = await this.store.createRecord('job');

    await renderNewJob(this, job);

    await planJob(spec);
    assert.ok(
      Editor.dryRunMessage.errored,
      'The scheduler dry-run message is in the warning state'
    );
    assert.notOk(
      Editor.dryRunMessage.succeeded,
      'The success message is not shown in addition to the warning message'
    );
    assert.ok(
      Editor.dryRunMessage.body.includes(newJobTaskGroupName),
      'The scheduler dry-run message includes the warning from send back by the API'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('when the scheduler dry-run has no warnings, a success message is shown to the user', async function (assert) {
    assert.expect(3);

    const spec = hclJob();
    const job = await this.store.createRecord('job');

    await renderNewJob(this, job);

    await planJob(spec);
    assert.ok(
      Editor.dryRunMessage.succeeded,
      'The scheduler dry-run message is in the success state'
    );
    assert.notOk(
      Editor.dryRunMessage.errored,
      'The warning message is not shown in addition to the success message'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('when a job is submitted in the edit context, a POST request is made to the update job endpoint', async function (assert) {
    const spec = hclJob();
    const job = await this.store.createRecord('job');

    this.set('job', job);

    this.set('onToggleEdit', () => {});
    this.set('onSubmit', () => {});
    this.set('handleSaveAsTemplate', () => {});
    this.set('onSelect', () => {});

    await render(hbs`
      <JobEditor
        @context="edit"
        @job={{this.job}}
        @onToggleEdit={{this.onToggleEdit}}
        @onSubmit={{this.onSubmit}}
        @handleSaveAsTemplate={{this.handleSaveAsTemplate}}
        @onSelect={{this.onSelect}}
      />
    `);

    await planJob(spec);
    await Editor.run();
    const requests = this.server.pretender.handledRequests
      .filterBy('method', 'POST')
      .mapBy('url');
    assert.ok(
      requests.includes(`/v1/job/${newJobName}`),
      'A request was made to job update'
    );
    assert.notOk(
      requests.includes('/v1/jobs'),
      'A request was not made to job create'
    );
  });

  test('when a job is submitted in the new context, a POST request is made to the create job endpoint', async function (assert) {
    const spec = hclJob();
    const job = await this.store.createRecord('job');

    await renderNewJob(this, job);

    await planJob(spec);
    await Editor.run();
    const requests = this.server.pretender.handledRequests
      .filterBy('method', 'POST')
      .mapBy('url');
    assert.ok(
      requests.includes('/v1/jobs'),
      'A request was made to job create'
    );
    assert.notOk(
      requests.includes(`/v1/job/${newJobName}`),
      'A request was not made to job update'
    );
  });

  test('when a job is successfully submitted, the onSubmit hook is called', async function (assert) {
    const spec = hclJob();
    const job = await this.store.createRecord('job');

    await renderNewJob(this, job);

    await planJob(spec);
    await Editor.run();
    assert.ok(
      this.onSubmit.calledWith(newJobName, 'default'),
      'The onSubmit hook was called with the correct arguments'
    );
  });

  test('when the job-editor cancelable flag is false, there is no cancel button in the header', async function (assert) {
    const job = await this.store.createRecord('job');

    await renderNewJob(this, job);
    assert.notOk(Editor.cancelEditingIsAvailable, 'No way to cancel editing');
  });

  test('when the job-editor cancelable flag is true, there is a cancel button in the header', async function (assert) {
    assert.expect(2);

    const job = await this.store.createRecord('job');

    this.set('job', job);

    this.set('onToggleEdit', () => {});
    this.set('onSubmit', () => {});
    this.set('handleSaveAsTemplate', () => {});
    this.set('onSelect', () => {});

    await render(hbs`
      <JobEditor
        @cancelable={{true}}
        @context="new"
        @job={{this.job}}
        @onToggleEdit={{this.onToggleEdit}}
        @onSubmit={{this.onSubmit}}
        @handleSaveAsTemplate={{this.handleSaveAsTemplate}}
        @onSelect={{this.onSelect}}
      />
    `);

    assert.ok(Editor.cancelEditingIsAvailable, 'Cancel editing button exists');

    await componentA11yAudit(this.element, assert);
  });

  test('constructor sets definition and variables correctly', async function (assert) {
    // Arrange
    const onSelect = () => {};
    this.set('onSelect', onSelect);
    this.set('definition', 'pablo');
    this.set('variables', {
      flags: { lastName: 'escobar' },
      literal: 'isCriminal=true',
    });

    // Prepare a job object with a set() method
    const job = {
      set(key, value) {
        this[key] = value;
      },
    };
    this.set('job', job);

    // Act
    await render(hbs`<JobEditor
      @specification={{this.definition}}
      @view="job-spec"
      @variables={{this.variables}}
      @job={{this.job}}
      @onSelect={{this.onSelect}} />`);

    // Check if the definition is set on the model
    assert.equal(job._newDefinition, 'pablo', 'Definition is set on the model');

    // Check if the newDefinitionVariables are set on the model
    function jsonToHcl(obj) {
      const hclLines = [];

      for (const key in obj) {
        const value = obj[key];
        const hclValue = typeof value === 'string' ? `"${value}"` : value;
        hclLines.push(`${key}=${hclValue}\n`);
      }

      return hclLines.join('\n');
    }
    const expectedVariables = jsonToHcl(this.variables.flags).concat(
      this.variables.literal
    );
    assert.deepEqual(
      job._newDefinitionVariables,
      expectedVariables,
      'Variables are set on the model'
    );
  });
});
