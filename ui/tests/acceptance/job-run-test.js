/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import AdapterError from '@ember-data/adapter/error';
import {
  click,
  currentRouteName,
  currentURL,
  fillIn,
  visit,
  settled,
} from '@ember/test-helpers';
import { assign } from '@ember/polyfills';
import { module, test } from 'qunit';
import {
  selectChoose,
  clickTrigger,
} from 'ember-power-select/test-support/helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import setupCodeMirror from 'nomad-ui/tests/helpers/codemirror';
import JobRun from 'nomad-ui/tests/pages/jobs/run';
import percySnapshot from '@percy/ember';

const newJobName = 'new-job';
const newJobTaskGroupName = 'redis';
const newJobNamespace = 'default';

let managementToken, clientToken;

const jsonJob = (overrides) => {
  return JSON.stringify(
    assign(
      {},
      {
        Name: newJobName,
        Namespace: newJobNamespace,
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

module('Acceptance | job run', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);
  setupCodeMirror(hooks);

  hooks.beforeEach(function () {
    // Required for placing allocations (a result of creating jobs)
    server.create('node-pool');
    server.create('node');

    managementToken = server.create('token');
    clientToken = server.create('token');

    window.localStorage.nomadTokenSecret = managementToken.secretId;
  });

  test('it passes an accessibility audit', async function (assert) {
    assert.expect(1);

    await JobRun.visit();
    await a11yAudit(assert);
  });

  test('visiting /jobs/run', async function (assert) {
    await JobRun.visit();

    assert.equal(currentURL(), '/jobs/run');
    assert.equal(document.title, 'Run a job - Nomad');
  });

  test('when submitting a job, the site redirects to the new job overview page', async function (assert) {
    const spec = jsonJob();

    await JobRun.visit();

    await JobRun.editor.editor.fillIn(spec);
    await JobRun.editor.plan();
    await JobRun.editor.run();
    assert.equal(
      currentURL(),
      `/jobs/${newJobName}@${newJobNamespace}`,
      `Redirected to the job overview page for ${newJobName}`
    );
  });

  test('when submitting a job to a different namespace, the redirect to the job overview page takes namespace into account', async function (assert) {
    const newNamespace = 'second-namespace';

    server.create('namespace', { id: newNamespace });
    const spec = jsonJob({ Namespace: newNamespace });

    await JobRun.visit();

    await JobRun.editor.editor.fillIn(spec);
    await JobRun.editor.plan();
    await JobRun.editor.run();
    assert.equal(
      currentURL(),
      `/jobs/${newJobName}@${newNamespace}`,
      `Redirected to the job overview page for ${newJobName} and switched the namespace to ${newNamespace}`
    );
  });

  test('when the user doesn’t have permission to run a job, redirects to the job overview page', async function (assert) {
    window.localStorage.nomadTokenSecret = clientToken.secretId;

    await JobRun.visit();
    assert.equal(currentURL(), '/jobs');
  });

  test('when using client token user can still go to job page if they have correct permissions', async function (assert) {
    const clientTokenWithPolicy = server.create('token');
    const newNamespace = 'second-namespace';

    server.create('namespace', { id: newNamespace });
    server.create('job', {
      groupCount: 0,
      createAllocations: false,
      shallow: true,
      noActiveDeployment: true,
      namespaceId: newNamespace,
    });

    const policy = server.create('policy', {
      id: 'something',
      name: 'something',
      rulesJSON: {
        Namespaces: [
          {
            Name: newNamespace,
            Capabilities: ['scale-job', 'submit-job', 'read-job', 'list-jobs'],
          },
        ],
      },
    });

    clientTokenWithPolicy.policyIds = [policy.id];
    clientTokenWithPolicy.save();
    window.localStorage.nomadTokenSecret = clientTokenWithPolicy.secretId;

    await JobRun.visit({ namespace: newNamespace });
    assert.equal(currentURL(), `/jobs/run?namespace=${newNamespace}`);
  });

  module('job template flow', function () {
    test('allows user with the correct permissions to fill in the editor using a job template', async function (assert) {
      assert.expect(10);
      // Arrange
      await JobRun.visit();
      assert
        .dom('[data-test-choose-template]')
        .exists('A button allowing a user to select a template appears.');

      server.get('/vars', function (_server, fakeRequest) {
        assert.deepEqual(
          fakeRequest.queryParams,
          {
            prefix: 'nomad/job-templates',
            namespace: '*',
          },
          'It makes a request to the /vars endpoint with the appropriate query parameters for job templates.'
        );
        return [
          {
            ID: 'nomad/job-templates/foo',
            Namespace: 'default',
            Path: 'nomad/job-templates/foo',
          },
        ];
      });

      server.get(
        '/var/nomad%2Fjob-templates%2Ffoo',
        function (_server, fakeRequest) {
          assert.deepEqual(
            fakeRequest.queryParams,
            {
              namespace: 'default',
            },
            'Dispatches O(n+1) query to retrive items.'
          );
          return {
            ID: 'nomad/job-templates/foo',
            Namespace: 'default',
            Path: 'nomad/job-templates/foo',
            Items: {
              template: 'Hello World!',
              label: 'foo',
            },
          };
        }
      );
      // Act
      await click('[data-test-choose-template]');
      assert.equal(currentRouteName(), 'jobs.run.templates.index');

      // Assert
      assert
        .dom('[data-test-template-list]')
        .exists('A list of available job templates is rendered.');
      assert
        .dom('[data-test-apply]')
        .exists('A button to apply the selected templated is displayed.');
      assert
        .dom('[data-test-cancel]')
        .exists('A button to cancel the template selection is displayed.');

      await click('[data-test-template-card=Foo]');
      await click('[data-test-apply]');

      assert.equal(
        currentURL(),
        '/jobs/run?template=nomad%2Fjob-templates%2Ffoo%40default'
      );
      assert.dom('[data-test-editor]').containsText('Hello World!');
    });

    test('a user can create their own job template', async function (assert) {
      assert.expect(7);
      // Arrange
      await JobRun.visit();
      await click('[data-test-choose-template]');

      // Assert
      assert
        .dom('[data-test-template-card]')
        .exists({ count: 4 }, 'A list of default job templates is rendered.');

      await click('[data-test-create-new-button]');
      assert.equal(currentRouteName(), 'jobs.run.templates.new');

      await fillIn('[data-test-template-name]', 'foo');
      await fillIn('[data-test-template-description]', 'foo-bar-baz');
      const codeMirror = getCodeMirrorInstance('[data-test-template-json]');
      codeMirror.setValue(jsonJob());

      server.put('/var/:varId', function (_server, fakeRequest) {
        assert.deepEqual(
          JSON.parse(fakeRequest.requestBody),
          {
            Path: 'nomad/job-templates/foo',
            CreateIndex: null,
            ModifyIndex: null,
            Namespace: 'default',
            ID: 'nomad/job-templates/foo',
            Items: { description: 'foo-bar-baz', template: jsonJob() },
          },
          'It makes a PUT request to the /vars/:varId endpoint with the appropriate request body for job templates.'
        );
        return {
          Items: { description: 'foo-bar-baz', template: jsonJob() },
          Namespace: 'default',
          Path: 'nomad/job-templates/foo',
        };
      });

      server.get('/vars', function (_server, fakeRequest) {
        assert.deepEqual(
          fakeRequest.queryParams,
          {
            prefix: 'nomad/job-templates',
            namespace: '*',
          },
          'It makes a request to the /vars endpoint with the appropriate query parameters for job templates.'
        );
        return [
          {
            ID: 'nomad/job-templates/foo',
            Namespace: 'default',
            Path: 'nomad/job-templates/foo',
          },
        ];
      });

      server.get(
        '/var/nomad%2Fjob-templates%2Ffoo',
        function (_server, fakeRequest) {
          assert.deepEqual(
            fakeRequest.queryParams,
            {
              namespace: 'default',
            },
            'Dispatches O(n+1) query to retrive items.'
          );
          return {
            ID: 'nomad/job-templates/foo',
            Namespace: 'default',
            Path: 'nomad/job-templates/foo',
            Items: {
              template: 'qud',
              label: 'foo',
            },
          };
        }
      );

      await click('[data-test-save-template]');
      assert.equal(currentRouteName(), 'jobs.run.templates.index');
      assert
        .dom('[data-test-template-card=Foo]')
        .exists('The newly created template appears in the list.');
    });

    test('a toast notification alerts the user if there is an error saving the newly created job template', async function (assert) {
      assert.expect(5);
      // Arrange
      await JobRun.visit();
      await click('[data-test-choose-template]');

      // Assert
      assert
        .dom('[data-test-template-card]')
        .exists({ count: 4 }, 'A list of default job templates is rendered.');

      await click('[data-test-create-new-button]');
      assert.equal(currentRouteName(), 'jobs.run.templates.new');
      assert
        .dom('[data-test-save-template]')
        .isDisabled('the save button should be disabled if no path is set');

      await fillIn('[data-test-template-name]', 'try@');
      await fillIn('[data-test-template-description]', 'foo-bar-baz');
      const codeMirror = getCodeMirrorInstance('[data-test-template-json]');
      codeMirror.setValue(jsonJob());

      server.put('/var/:varId?cas=0', function () {
        return new AdapterError({
          detail: `invalid path "nomad/job-templates/try@"`,
          status: 500,
        });
      });

      await click('[data-test-save-template]');
      assert.equal(
        currentRouteName(),
        'jobs.run.templates.new',
        'We do not navigate away from the page if an error is returned by the API.'
      );
      assert
        .dom('.flash-message.alert-critical')
        .exists('A toast error message pops up.');
    });

    test('a user cannot create a job template if one with the same name and namespace already exists', async function (assert) {
      assert.expect(4);
      // Arrange
      await JobRun.visit();
      await click('[data-test-choose-template]');
      server.create('variable', {
        path: 'nomad/job-templates/foo',
        namespace: 'default',
        id: 'nomad/job-templates/foo',
      });
      server.create('namespace', { id: 'test' });

      this.system = this.owner.lookup('service:system');
      this.system.shouldShowNamespaces = true;

      // Assert
      assert
        .dom('[data-test-template-card]')
        .exists({ count: 4 }, 'A list of default job templates is rendered.');

      await click('[data-test-create-new-button]');
      assert.equal(currentRouteName(), 'jobs.run.templates.new');

      await fillIn('[data-test-template-name]', 'foo');
      assert
        .dom('[data-test-duplicate-error]')
        .exists('an error message alerts the user');

      await clickTrigger('[data-test-namespace-facet]');
      await selectChoose('[data-test-namespace-facet]', 'test');

      assert
        .dom('[data-test-duplicate-error]')
        .doesNotExist(
          'an error disappears when name or namespace combination is unique'
        );

      // Clean-up
      this.system.shouldShowNamespaces = false;
    });

    test('a user can save code from the editor as a template', async function (assert) {
      assert.expect(4);
      // Arrange
      await JobRun.visit();
      await JobRun.editor.editor.fillIn(jsonJob());

      await click('[data-test-save-as-template]');
      assert.equal(
        currentRouteName(),
        'jobs.run.templates.new',
        'We navigate template creation page.'
      );

      // Assert
      assert
        .dom('[data-test-template-name]')
        .hasNoText('No template name is prefilled.');
      assert
        .dom('[data-test-template-description]')
        .hasNoText('No template description is prefilled.');

      const codeMirror = getCodeMirrorInstance('[data-test-template-json]');
      const json = codeMirror.getValue();

      assert.equal(
        json,
        jsonJob(),
        'Template is filled out with text from the editor.'
      );
    });

    test('a user can edit a template', async function (assert) {
      assert.expect(5);

      // Arrange
      server.create('variable', {
        path: 'nomad/job-templates/foo',
        namespace: 'default',
        id: 'nomad/job-templates/foo',
        Items: {},
      });

      await visit('/jobs/run/templates/manage');

      assert.equal(currentRouteName(), 'jobs.run.templates.manage');
      assert
        .dom('[data-test-template-list]')
        .exists('A list of templates is visible');
      await percySnapshot(assert);
      await click('[data-test-edit-template="nomad/job-templates/foo"]');
      assert.equal(
        currentRouteName(),
        'jobs.run.templates.template',
        'Navigates to edit template view'
      );

      server.put('/var/:varId', function (_server, fakeRequest) {
        assert.deepEqual(
          JSON.parse(fakeRequest.requestBody),
          {
            Path: 'nomad/job-templates/foo',
            CreateIndex: null,
            ModifyIndex: null,
            Namespace: 'default',
            ID: 'nomad/job-templates/foo',
            Items: { description: 'baz qud thud' },
          },
          'It makes a PUT request to the /vars/:varId endpoint with the appropriate request body for job templates.'
        );

        return {
          Items: { description: 'baz qud thud' },
          Namespace: 'default',
          Path: 'nomad/job-templates/foo',
        };
      });

      await fillIn('[data-test-template-description]', 'baz qud thud');
      await click('[data-test-edit-template]');

      assert.equal(
        currentRouteName(),
        'jobs.run.templates.index',
        'We navigate back to the templates view.'
      );
    });

    test('a user can delete a template', async function (assert) {
      assert.expect(5);

      // Arrange
      server.create('variable', {
        path: 'nomad/job-templates/foo',
        namespace: 'default',
        id: 'nomad/job-templates/foo',
        Items: {},
      });

      server.create('variable', {
        path: 'nomad/job-templates/bar',
        namespace: 'default',
        id: 'nomad/job-templates/bar',
        Items: {},
      });

      server.create('variable', {
        path: 'nomad/job-templates/baz',
        namespace: 'default',
        id: 'nomad/job-templates/baz',
        Items: {},
      });

      await visit('/jobs/run/templates/manage');

      assert.equal(currentRouteName(), 'jobs.run.templates.manage');
      assert
        .dom('[data-test-template-list]')
        .exists('A list of templates is visible');

      await click('[data-test-idle-button]');
      await click('[data-test-confirm-button]');
      assert
        .dom('[data-test-edit-template="nomad/job-templates/foo"]')
        .doesNotExist('The template is removed from the list.');

      await click('[data-test-edit-template="nomad/job-templates/bar"]');
      await click('[data-test-idle-button]');
      await click('[data-test-confirm-button]');

      assert.equal(
        currentRouteName(),
        'jobs.run.templates.manage',
        'We navigate back to the templates manager view.'
      );

      assert
        .dom('[data-test-edit-template="nomad/job-templates/bar"]')
        .doesNotExist('The template is removed from the list.');
    });

    test('a user sees accurate template information', async function (assert) {
      assert.expect(3);

      // Arrange
      server.create('variable', {
        path: 'nomad/job-templates/foo',
        namespace: 'default',
        id: 'nomad/job-templates/foo',
        Items: {
          template: 'qud',
          label: 'foo',
          description: 'bar baz',
        },
      });

      await visit('/jobs/run/templates');

      assert.equal(currentRouteName(), 'jobs.run.templates.index');
      assert.dom('[data-test-template-card="Foo"]').exists();

      this.store = this.owner.lookup('service:store');
      this.store.unloadAll();
      await settled();

      assert
        .dom('[data-test-template-card="Foo"]')
        .doesNotExist(
          'The template reactively updates to changes in the Ember Data Store.'
        );
    });

    test('default templates', async function (assert) {
      assert.expect(4);
      const NUMBER_OF_DEFAULT_TEMPLATES = 4;

      await visit('/jobs/run/templates');

      assert.equal(currentRouteName(), 'jobs.run.templates.index');
      assert
        .dom('[data-test-template-card]')
        .exists({ count: NUMBER_OF_DEFAULT_TEMPLATES });

      await percySnapshot(assert);

      await click('[data-test-template-card="Hello world"]');
      await click('[data-test-apply]');

      assert.equal(
        currentURL(),
        '/jobs/run?template=nomad%2Fjob-templates%2Fdefault%2Fhello-world'
      );
      assert.dom('[data-test-editor]').includesText('job "hello-world"');
    });
  });
});
