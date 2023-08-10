/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  currentRouteName,
  currentURL,
  click,
  find,
  findAll,
  typeIn,
  visit,
} from '@ember/test-helpers';
import { setupMirage } from 'ember-cli-mirage/test-support';
import {
  selectChoose,
  clickTrigger,
} from 'ember-power-select/test-support/helpers';
import { setupApplicationTest } from 'ember-qunit';
import { module, test } from 'qunit';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import { allScenarios } from '../../mirage/scenarios/default';
import cleanWhitespace from '../utils/clean-whitespace';
import percySnapshot from '@percy/ember';
import faker from 'nomad-ui/mirage/faker';

import Variables from 'nomad-ui/tests/pages/variables';
import Layout from 'nomad-ui/tests/pages/layout';

const VARIABLE_TOKEN_ID = '53cur3-v4r14bl35';
const LIMITED_VARIABLE_TOKEN_ID = 'f3w3r-53cur3-v4r14bl35';

module('Acceptance | variables', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);
  hooks.beforeEach(async function () {
    faker.seed(1);
    server.createList('variable', 3);
  });

  test('it redirects to jobs and hides the gutter link when the token lacks permissions', async function (assert) {
    await Variables.visit();
    assert.equal(currentURL(), '/jobs');
    assert.ok(Layout.gutter.variables.isHidden);
  });

  test('it allows access for management level tokens', async function (assert) {
    allScenarios.variableTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await Variables.visit();
    assert.equal(currentURL(), '/variables');
    assert.ok(Layout.gutter.variables.isVisible, 'Menu section is visible');
  });

  test('it allows access for list-variables allowed ACL rules', async function (assert) {
    assert.expect(2);
    allScenarios.variableTestCluster(server);
    const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
    window.localStorage.nomadTokenSecret = variablesToken.secretId;

    await Variables.visit();
    assert.equal(currentURL(), '/variables');
    assert.ok(Layout.gutter.variables.isVisible);
    await percySnapshot(assert);
  });

  test('it correctly traverses to and deletes a variable', async function (assert) {
    assert.expect(13);
    allScenarios.variableTestCluster(server);
    const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
    window.localStorage.nomadTokenSecret = variablesToken.secretId;
    server.db.variables.update({ namespace: 'default' });
    const policy = server.db.policies.find('Variable Maker');
    policy.rulesJSON.Namespaces[0].Variables.Paths.find(
      (path) => path.PathSpec === '*'
    ).Capabilities = ['list', 'read', 'destroy'];

    await Variables.visit();
    assert.equal(currentURL(), '/variables');
    assert.ok(Layout.gutter.variables.isVisible);

    let abcLink = [...findAll('[data-test-folder-row]')].filter((a) =>
      a.textContent.includes('a/b/c')
    )[0];

    await click(abcLink);

    assert.equal(
      currentURL(),
      '/variables/path/a/b/c',
      'correctly traverses to a deeply nested path'
    );
    assert.equal(
      findAll('[data-test-folder-row]').length,
      2,
      'correctly shows 2 sub-folders'
    );
    assert.equal(
      findAll('[data-test-file-row]').length,
      2,
      'correctly shows 2 files'
    );
    let fooLink = [...findAll('[data-test-file-row]')].filter((a) =>
      a.textContent.includes('foo0')
    )[0];

    assert.ok(fooLink, 'foo0 file is present');

    await percySnapshot(assert);

    await click(fooLink);
    assert.ok(
      currentURL().includes('/variables/var/a/b/c/foo0'),
      'correctly traverses to a deeply nested variable file'
    );
    const deleteButton = find('[data-test-delete-button] button');
    assert.dom(deleteButton).exists('delete button is present');

    await percySnapshot('deeply nested variable');

    await click(deleteButton);
    assert
      .dom('[data-test-confirmation-message]')
      .exists('confirmation message is present');

    await click(find('[data-test-confirm-button]'));
    assert.equal(
      currentURL(),
      '/variables/path/a/b/c',
      'correctly returns to the parent path page after deletion'
    );

    assert.equal(
      findAll('[data-test-folder-row]').length,
      2,
      'still correctly shows 2 sub-folders'
    );
    assert.equal(
      findAll('[data-test-file-row]').length,
      1,
      'now correctly shows 1 file'
    );

    fooLink = [...findAll('[data-test-file-row]')].filter((a) =>
      a.textContent.includes('foo0')
    )[0];

    assert.notOk(fooLink, 'foo0 file is no longer present');
  });

  test('variables prefixed with nomad/jobs/ correctly link to entities', async function (assert) {
    assert.expect(29);
    allScenarios.variableTestCluster(server);
    const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);

    const variableLinkedJob = server.db.jobs[0];
    const variableLinkedGroup = server.db.taskGroups.findBy({
      jobId: variableLinkedJob.id,
    });
    const variableLinkedTask = server.db.tasks.findBy({
      taskGroupId: variableLinkedGroup.id,
    });
    const variableLinkedTaskAlloc = server.db.allocations
      .filterBy('taskGroup', variableLinkedGroup.name)
      ?.find((alloc) => alloc.taskStateIds.length);

    window.localStorage.nomadTokenSecret = variablesToken.secretId;

    // Non-job variable
    await Variables.visit();
    assert.equal(currentURL(), '/variables');
    assert.ok(Layout.gutter.variables.isVisible);

    let nonJobLink = [...findAll('[data-test-file-row]')].filter((a) =>
      a.textContent.includes('just some arbitrary file')
    )[0];

    assert.ok(nonJobLink, 'non-job file is present');

    await click(nonJobLink);
    assert.ok(
      currentURL().includes('/variables/var/just some arbitrary file'),
      'correctly traverses to a non-job file'
    );
    let relatedEntitiesBox = find('.related-entities');
    assert
      .dom(relatedEntitiesBox)
      .doesNotExist('Related Entities box is not present');

    // Job variable
    await Variables.visit();
    let jobsDirectoryLink = [...findAll('[data-test-folder-row]')].filter((a) =>
      a.textContent.includes('jobs')
    )[0];

    await click(jobsDirectoryLink);

    assert.equal(
      currentURL(),
      '/variables/path/nomad/jobs',
      'correctly traverses to the jobs directory'
    );
    let jobFileLink = find('[data-test-file-row]');

    assert.ok(jobFileLink, 'A job file is present');

    await click(jobFileLink);
    assert.ok(
      currentURL().startsWith('/variables/var/nomad/jobs/'),
      'correctly traverses to a job file'
    );
    relatedEntitiesBox = find('.related-entities');
    assert.dom(relatedEntitiesBox).exists('Related Entities box is present');
    assert.ok(
      cleanWhitespace(relatedEntitiesBox.textContent).includes(
        'This variable is accessible by job'
      ),
      'Related Entities box is job-oriented'
    );

    await percySnapshot('related entities box for job variable');

    let relatedJobLink = find('.related-entities a');
    await click(relatedJobLink);
    assert
      .dom('[data-test-job-stat="variables"]')
      .exists('Link from Job to Variable exists');
    let jobVariableLink = find('[data-test-job-stat="variables"] a');
    await click(jobVariableLink);
    assert.ok(
      currentURL().startsWith(
        `/variables/var/nomad/jobs/${variableLinkedJob.id}`
      ),
      'correctly traverses from job to variable'
    );

    // Group Variable
    await Variables.visit();
    jobsDirectoryLink = [...findAll('[data-test-folder-row]')].filter((a) =>
      a.textContent.includes('jobs')
    )[0];
    await click(jobsDirectoryLink);
    let groupDirectoryLink = [...findAll('[data-test-folder-row]')][0];
    await click(groupDirectoryLink);
    let groupFileLink = find('[data-test-file-row]');
    assert.ok(groupFileLink, 'A group file is present');
    await click(groupFileLink);
    relatedEntitiesBox = find('.related-entities');
    assert.dom(relatedEntitiesBox).exists('Related Entities box is present');
    assert.ok(
      cleanWhitespace(relatedEntitiesBox.textContent).includes(
        'This variable is accessible by group'
      ),
      'Related Entities box is group-oriented'
    );

    await percySnapshot('related entities box for group variable');

    let relatedGroupLink = find('.related-entities a');
    await click(relatedGroupLink);
    assert
      .dom('[data-test-task-group-stat="variables"]')
      .exists('Link from Group to Variable exists');
    let groupVariableLink = find('[data-test-task-group-stat="variables"] a');
    await click(groupVariableLink);
    assert.ok(
      currentURL().startsWith(
        `/variables/var/nomad/jobs/${variableLinkedJob.id}/${variableLinkedGroup.name}`
      ),
      'correctly traverses from group to variable'
    );

    // Task Variable
    await Variables.visit();
    jobsDirectoryLink = [...findAll('[data-test-folder-row]')].filter((a) =>
      a.textContent.includes('jobs')
    )[0];
    await click(jobsDirectoryLink);
    groupDirectoryLink = [...findAll('[data-test-folder-row]')][0];
    await click(groupDirectoryLink);
    let taskDirectoryLink = [...findAll('[data-test-folder-row]')][0];
    await click(taskDirectoryLink);
    let taskFileLink = find('[data-test-file-row]');
    assert.ok(taskFileLink, 'A task file is present');
    await click(taskFileLink);
    relatedEntitiesBox = find('.related-entities');
    assert.dom(relatedEntitiesBox).exists('Related Entities box is present');
    assert.ok(
      cleanWhitespace(relatedEntitiesBox.textContent).includes(
        'This variable is accessible by task'
      ),
      'Related Entities box is task-oriented'
    );

    await percySnapshot('related entities box for task variable');

    let relatedTaskLink = find('.related-entities a');
    await click(relatedTaskLink);
    // Gotta go the long way and click into the alloc/then task from here; but we know this one by virtue of stable test env.
    await visit(
      `/allocations/${variableLinkedTaskAlloc.id}/${variableLinkedTask.name}`
    );
    assert
      .dom('[data-test-task-stat="variables"]')
      .exists('Link from Task to Variable exists');
    let taskVariableLink = find('[data-test-task-stat="variables"] a');
    await click(taskVariableLink);
    assert.ok(
      currentURL().startsWith(
        `/variables/var/nomad/jobs/${variableLinkedJob.id}/${variableLinkedGroup.name}/${variableLinkedTask.name}`
      ),
      'correctly traverses from task to variable'
    );

    // A non-variable-having job
    await visit(`/jobs/${server.db.jobs[1].id}`);
    assert
      .dom('[data-test-task-stat="variables"]')
      .doesNotExist('Link from Variable-less Job to Variable does not exist');

    // Related Entities during the Variable creation process
    await Variables.visitNew();
    assert
      .dom('.related-entities.notification')
      .doesNotExist('Related Entities notification is not present by default');
    await typeIn('[data-test-path-input]', 'foo/bar');
    assert
      .dom('.related-entities.notification')
      .doesNotExist(
        'Related Entities notification is not present when path is generic'
      );
    document.querySelector('[data-test-path-input]').value = ''; // clear path input
    await typeIn('[data-test-path-input]', 'nomad/jobs/abc');
    assert
      .dom('.related-entities.notification')
      .exists(
        'Related Entities notification is present when path is job-oriented'
      );
    assert
      .dom('.related-entities.notification')
      .containsText(
        'This variable will be accessible by job',
        'Related Entities notification is job-oriented'
      );
    await typeIn('[data-test-path-input]', '/def');
    assert
      .dom('.related-entities.notification')
      .containsText(
        'This variable will be accessible by group',
        'Related Entities notification is group-oriented'
      );
    await typeIn('[data-test-path-input]', '/ghi');
    assert
      .dom('.related-entities.notification')
      .containsText(
        'This variable will be accessible by task',
        'Related Entities notification is task-oriented'
      );
  });

  test('it does not allow you to save if you lack Items', async function (assert) {
    assert.expect(5);
    allScenarios.variableTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await Variables.visitNew();
    assert.equal(currentURL(), '/variables/new');
    await typeIn('.path-input', 'foo/bar');
    await click('button[type="submit"]');
    assert.dom('.flash-message.alert-critical').exists();
    await click('.flash-message.alert-critical .hds-dismiss-button');
    assert.dom('.flash-message.alert-critical').doesNotExist();

    await typeIn('.key-value label:nth-child(1) input', 'myKey');
    await typeIn('.key-value label:nth-child(2) input', 'superSecret');

    await percySnapshot(assert);

    await click('button[type="submit"]');

    assert.dom('.flash-message.alert-success').exists();
    assert.ok(
      currentURL().includes('/variables/var/foo'),
      'drops you back off to the parent page'
    );
  });

  test('it passes an accessibility audit', async function (assert) {
    assert.expect(1);
    allScenarios.variableTestCluster(server);
    const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
    window.localStorage.nomadTokenSecret = variablesToken.secretId;
    await Variables.visit();
    await a11yAudit(assert);
  });

  module('create flow', function () {
    test('allows a user with correct permissions to create a variable', async function (assert) {
      // Arrange Test Set-up
      allScenarios.variableTestCluster(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      await Variables.visit();
      // End Test Set-up

      assert
        .dom('[data-test-create-var]')
        .exists('It should display an enabled button to create a variable');
      await click('[data-test-create-var]');

      assert.equal(currentRouteName(), 'variables.new');

      await typeIn('[data-test-path-input]', 'foo/bar');
      await clickTrigger('[data-test-variable-namespace-filter]');

      assert.dom('.dropdown-options').exists('Namespace can be edited.');
      assert
        .dom('[data-test-variable-namespace-filter]')
        .containsText(
          'default',
          'The first alphabetically sorted namespace should be selected as the default option.'
        );

      await selectChoose(
        '[data-test-variable-namespace-filter]',
        'namespace-1'
      );
      await typeIn('[data-test-var-key]', 'kiki');
      await typeIn('[data-test-var-value]', 'do you love me');
      await click('[data-test-submit-var]');

      assert.equal(
        currentRouteName(),
        'variables.variable.index',
        'Navigates user back to variables list page after creating variable.'
      );
      assert
        .dom('.flash-message.alert.alert-success')
        .exists('Shows a success toast notification on creation.');
      assert
        .dom('[data-test-var=kiki]')
        .exists('The new variable key should appear in the list.');

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });

    test('prevents users from creating a variable without proper permissions', async function (assert) {
      // Arrange Test Set-up
      allScenarios.variableTestCluster(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      const policy = server.db.policies.find('Variable Maker');
      policy.rulesJSON.Namespaces[0].Variables.Paths.find(
        (path) => path.PathSpec === '*'
      ).Capabilities = ['list'];
      await Variables.visit();
      // End Test Set-up

      assert
        .dom('[data-test-disabled-create-var]')
        .exists(
          'It should display an disabled button to create a variable on the main listings page'
        );

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });

    test('allows creating a variable that starts with nomad/jobs/', async function (assert) {
      // Arrange Test Set-up
      allScenarios.variableTestCluster(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      await Variables.visitNew();
      // End Test Set-up

      await typeIn('[data-test-path-input]', 'nomad/jobs/foo/bar');
      await typeIn('[data-test-var-key]', 'my-test-key');
      await typeIn('[data-test-var-value]', 'my_test_value');
      await click('[data-test-submit-var]');

      assert.equal(
        currentRouteName(),
        'variables.variable.index',
        'Navigates user back to variables list page after creating variable.'
      );
      assert
        .dom('.flash-message.alert.alert-success')
        .exists('Shows a success toast notification on creation.');

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });

    test('disallows creating a variable that starts with nomad/<something-other-than-jobs>/', async function (assert) {
      // Arrange Test Set-up
      allScenarios.variableTestCluster(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      await Variables.visitNew();
      // End Test Set-up

      await typeIn('[data-test-path-input]', 'nomad/foo/');
      await typeIn('[data-test-var-key]', 'my-test-key');
      await typeIn('[data-test-var-value]', 'my_test_value');
      assert
        .dom('[data-test-submit-var]')
        .isDisabled(
          'Cannot submit a variable that begins with nomad/<not-jobs>/'
        );

      document.querySelector('[data-test-path-input]').value = ''; // clear current input
      await typeIn('[data-test-path-input]', 'nomad/jobs/');
      assert
        .dom('[data-test-submit-var]')
        .isNotDisabled('Can submit a variable that begins with nomad/jobs/');

      document.querySelector('[data-test-path-input]').value = ''; // clear current input
      await typeIn('[data-test-path-input]', 'nomad/another-foo/');
      assert
        .dom('[data-test-submit-var]')
        .isDisabled('Disabled state re-evaluated when path input changes');

      document.querySelector('[data-test-path-input]').value = ''; // clear current input
      await typeIn('[data-test-path-input]', 'nomad/jobs/job-templates/');
      assert
        .dom('[data-test-submit-var]')
        .isNotDisabled(
          'Can submit a variable that begins with nomad/job-templates/'
        );

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });

    test('shows a custom editor when editing a job template variable', async function (assert) {
      // Arrange Test Set-up
      allScenarios.variableTestCluster(server);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      await Variables.visitNew();
      // End Test Set-up

      assert
        .dom('.related-entities-hint')
        .exists('Shows a hint about related entities by default');
      assert.dom('.CodeMirror').doesNotExist();
      await typeIn('[data-test-path-input]', 'nomad/job-templates/hello-world');
      assert
        .dom('.related-entities-hint')
        .doesNotExist('Hides the hint when editing a job template variable');
      assert
        .dom('.job-template-hint')
        .exists('Shows a hint about job templates');
      assert
        .dom('.CodeMirror')
        .exists('Shows a custom editor for job templates');

      document.querySelector('[data-test-path-input]').value = ''; // clear current input
      await typeIn('[data-test-path-input]', 'hello-world-non-template');
      assert
        .dom('.related-entities-hint')
        .exists('Shows a hint about related entities by default');
      assert.dom('.CodeMirror').doesNotExist();
      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });
  });

  module('edit flow', function () {
    test('allows a user with correct permissions to edit a variable', async function (assert) {
      assert.expect(8);
      // Arrange Test Set-up
      allScenarios.variableTestCluster(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      const policy = server.db.policies.find('Variable Maker');
      policy.rulesJSON.Namespaces[0].Variables.Paths.find(
        (path) => path.PathSpec === '*'
      ).Capabilities = ['list', 'read', 'write'];
      server.db.variables.update({ namespace: 'default' });
      await Variables.visit();
      await click('[data-test-file-row]');
      // End Test Set-up

      assert.equal(currentRouteName(), 'variables.variable.index');
      assert
        .dom('[data-test-edit-button]')
        .exists('The edit button is enabled in the view.');
      await click('[data-test-edit-button]');
      assert.equal(
        currentRouteName(),
        'variables.variable.edit',
        'Clicking the button navigates you to editing view.'
      );

      await percySnapshot(assert);

      assert.dom('[data-test-path-input]').isDisabled('Path cannot be edited');
      await clickTrigger('[data-test-variable-namespace-filter]');
      assert
        .dom('.dropdown-options')
        .doesNotExist('Namespace cannot be edited.');

      document.querySelector('[data-test-var-key]').value = ''; // clear current input
      await typeIn('[data-test-var-key]', 'kiki');
      await typeIn('[data-test-var-value]', 'do you love me');
      await click('[data-test-submit-var]');
      assert.equal(
        currentRouteName(),
        'variables.variable.index',
        'Navigates user back to variables list page after creating variable.'
      );
      assert
        .dom('.flash-message.alert.alert-success')
        .exists('Shows a success toast notification on edit.');
      assert
        .dom('[data-test-var=kiki]')
        .exists('The edited variable key should appear in the list.');

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });

    test('prevents users from editing a variable without proper permissions', async function (assert) {
      // Arrange Test Set-up
      allScenarios.variableTestCluster(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      const policy = server.db.policies.find('Variable Maker');
      policy.rulesJSON.Namespaces[0].Variables.Paths.find(
        (path) => path.PathSpec === '*'
      ).Capabilities = ['list', 'read'];
      await Variables.visit();
      await click('[data-test-file-row]');
      // End Test Set-up

      assert.equal(currentRouteName(), 'variables.variable.index');
      assert
        .dom('[data-test-edit-button]')
        .doesNotExist('The edit button is hidden in the view.');

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });
    test('handles conflicts on save', async function (assert) {
      // Arrange Test Set-up
      allScenarios.variableTestCluster(server);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      // End Test Set-up

      await Variables.visitConflicting();
      await click('button[type="submit"]');

      assert
        .dom('.notification.conflict')
        .exists('Notification alerting user of conflict is present');

      document.querySelector('[data-test-var-key]').value = ''; // clear current input
      await typeIn('[data-test-var-key]', 'buddy');
      await typeIn('[data-test-var-value]', 'pal');
      await click('[data-test-submit-var]');

      await click('button[data-test-overwrite-button]');
      assert.equal(
        currentURL(),
        '/variables/var/Auto-conflicting Variable@default',
        'Selecting overwrite forces a save and redirects'
      );

      assert
        .dom('.flash-message.alert.alert-success')
        .exists('Shows a success toast notification on edit.');

      assert
        .dom('[data-test-var=buddy]')
        .exists('The edited variable key should appear in the list.');

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });

    test('warns you if you try to leave with an unsaved form', async function (assert) {
      // Arrange Test Set-up
      allScenarios.variableTestCluster(server);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;

      const originalWindowConfirm = window.confirm;
      let confirmFired = false;
      let leave = true;
      window.confirm = function () {
        confirmFired = true;
        return leave;
      };
      // End Test Set-up

      await Variables.visitConflicting();
      document.querySelector('[data-test-var-key]').value = ''; // clear current input
      await typeIn('[data-test-var-key]', 'buddy');
      await typeIn('[data-test-var-value]', 'pal');
      await click('[data-test-gutter-link="jobs"]');
      assert.ok(confirmFired, 'Confirm fired when leaving with unsaved form');
      assert.equal(
        currentURL(),
        '/jobs?namespace=*',
        'Opted to leave, ended up on desired page'
      );

      // Reset checks
      confirmFired = false;
      leave = false;

      await Variables.visitConflicting();
      document.querySelector('[data-test-var-key]').value = ''; // clear current input
      await typeIn('[data-test-var-key]', 'buddy');
      await typeIn('[data-test-var-value]', 'pal');
      await click('[data-test-gutter-link="jobs"]');
      assert.ok(confirmFired, 'Confirm fired when leaving with unsaved form');
      assert.equal(
        currentURL(),
        '/variables/var/Auto-conflicting%20Variable@default/edit',
        'Opted to stay, did not leave page'
      );

      // Reset checks
      confirmFired = false;

      await Variables.visitConflicting();
      document.querySelector('[data-test-var-key]').value = ''; // clear current input
      await typeIn('[data-test-var-key]', 'buddy');
      await typeIn('[data-test-var-value]', 'pal');
      await click('[data-test-json-toggle]');
      assert.notOk(
        confirmFired,
        'Confirm did not fire when only transitioning queryParams'
      );
      assert.equal(
        currentURL(),
        '/variables/var/Auto-conflicting%20Variable@default/edit?view=json',
        'Stayed on page, queryParams changed'
      );

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
      // Restore the original window.confirm implementation
      window.confirm = originalWindowConfirm;
    });
  });

  module('delete flow', function () {
    test('allows a user with correct permissions to delete a variable', async function (assert) {
      // Arrange Test Set-up
      allScenarios.variableTestCluster(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      const policy = server.db.policies.find('Variable Maker');
      policy.rulesJSON.Namespaces[0].Variables.Paths.find(
        (path) => path.PathSpec === '*'
      ).Capabilities = ['list', 'read', 'destroy'];
      server.db.variables.update({ namespace: 'default' });
      await Variables.visit();
      await click('[data-test-file-row]');
      // End Test Set-up
      assert.equal(currentRouteName(), 'variables.variable.index');
      assert
        .dom('[data-test-delete-button]')
        .exists('The delete button is enabled in the view.');
      await click('[data-test-idle-button]');

      assert
        .dom('[data-test-confirmation-message]')
        .exists('Deleting a variable requires two-step confirmation.');

      await click('[data-test-confirm-button]');

      assert.equal(
        currentRouteName(),
        'variables.index',
        'Navigates user back to variables list page after destroying a variable.'
      );

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });

    test('prevents users from delete a variable without proper permissions', async function (assert) {
      // Arrange Test Set-up
      allScenarios.variableTestCluster(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      const policy = server.db.policies.find('Variable Maker');
      policy.rulesJSON.Namespaces[0].Variables.Paths.find(
        (path) => path.PathSpec === '*'
      ).Capabilities = ['list', 'read'];
      await Variables.visit();
      await click('[data-test-file-row]');
      // End Test Set-up

      assert.equal(currentRouteName(), 'variables.variable.index');
      assert
        .dom('[data-test-delete-button]')
        .doesNotExist('The delete button is hidden in the view.');

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });
  });

  module('read flow', function () {
    test('allows a user with correct permissions to read a variable', async function (assert) {
      allScenarios.variableTestCluster(server);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      await Variables.visit();

      assert
        .dom('[data-test-file-row]:not(.inaccessible)')
        .exists(
          { count: 4 },
          'Shows 4 variable files, none of which are inaccessible'
        );

      await click('[data-test-file-row]');
      assert.equal(currentRouteName(), 'variables.variable.index');

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });

    test('prevents users from reading a variable without proper permissions', async function (assert) {
      allScenarios.variableTestCluster(server);
      const variablesToken = server.db.tokens.find(LIMITED_VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      await Variables.visit();

      assert
        .dom('[data-test-file-row].inaccessible')
        .exists(
          { count: 4 },
          'Shows 4 variable files, all of which are inaccessible'
        );

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });
  });

  module('namespace filtering', function () {
    test('allows a user to filter variables by namespace', async function (assert) {
      assert.expect(3);

      // Arrange
      allScenarios.variableTestCluster(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      await Variables.visit();

      assert
        .dom('[data-test-variable-namespace-filter]')
        .exists('Shows a dropdown of namespaces');

      // Assert Side Side Effect
      server.get('/vars', function (_server, fakeRequest) {
        assert.deepEqual(
          fakeRequest.queryParams,
          {
            namespace: 'default',
          },
          'It makes another server request using the options selected by the user'
        );
        return [];
      });

      // Act
      await clickTrigger('[data-test-variable-namespace-filter]');
      await selectChoose('[data-test-variable-namespace-filter]', 'default');

      assert
        .dom('[data-test-no-matching-variables-list-headline]')
        .exists('Renders an empty list.');
    });

    test('does not show namespace filtering if the user only has access to one namespace', async function (assert) {
      allScenarios.variableTestCluster(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      const twoTokens = server.db.namespaces.slice(0, 2);
      server.db.namespaces.remove(twoTokens);
      await Variables.visit();

      assert.equal(
        server.db.namespaces.length,
        1,
        'There should only be one namespace.'
      );
      assert
        .dom('[data-test-variable-namespace-filter]')
        .doesNotExist('Does not show a dropdown of namespaces');
    });

    module('path route', function () {
      test('allows a user to filter variables by namespace', async function (assert) {
        assert.expect(4);

        // Arrange
        allScenarios.variableTestCluster(server);
        server.createList('variable', 3);
        const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
        window.localStorage.nomadTokenSecret = variablesToken.secretId;
        await Variables.visit();
        await click('[data-test-folder-row]');

        assert.equal(
          currentRouteName(),
          'variables.path',
          'It navigates a user to the path subroute'
        );

        assert
          .dom('[data-test-variable-namespace-filter]')
          .exists('Shows a dropdown of namespaces');

        // Assert Side Side Effect
        server.get('/vars', function (_server, fakeRequest) {
          assert.deepEqual(
            fakeRequest.queryParams,
            {
              namespace: 'default',
            },
            'It makes another server request using the options selected by the user'
          );
          return [];
        });

        // Act
        await clickTrigger('[data-test-variable-namespace-filter]');
        await selectChoose('[data-test-variable-namespace-filter]', 'default');

        assert
          .dom('[data-test-no-matching-variables-list-headline]')
          .exists('Renders an empty list.');
      });

      test('does not show namespace filtering if the user only has access to one namespace', async function (assert) {
        allScenarios.variableTestCluster(server);
        server.createList('variable', 3);
        const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
        window.localStorage.nomadTokenSecret = variablesToken.secretId;
        const twoTokens = server.db.namespaces.slice(0, 2);
        server.db.namespaces.remove(twoTokens);
        await Variables.visit();

        assert.equal(
          server.db.namespaces.length,
          1,
          'There should only be one namespace.'
        );

        await click('[data-test-folder-row]');

        assert.equal(
          currentRouteName(),
          'variables.path',
          'It navigates a user to the path subroute'
        );

        assert
          .dom('[data-test-variable-namespace-filter]')
          .doesNotExist('Does not show a dropdown of namespaces');
      });
    });
  });

  module('Job Variables Page', function () {
    test('If the user has no variable read access, no subnav exists', async function (assert) {
      allScenarios.variableTestCluster(server);
      const variablesToken = server.db.tokens.find('n0-v4r5-4cc355');
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      await visit(
        `/jobs/${server.db.jobs[0].id}@${server.db.jobs[0].namespace}`
      );
      // Variables tab isn't in subnav
      assert.dom('[data-test-tab="variables"]').doesNotExist();

      // Attempting to access it directly will boot you to /jobs
      await visit(
        `/jobs/${server.db.jobs[0].id}@${server.db.jobs[0].namespace}/variables`
      );
      assert.equal(currentURL(), '/jobs?namespace=*');

      window.localStorage.nomadTokenSecret = null; // Reset Token
    });

    test('If the user has variable read access, but no variables, the subnav exists but contains only a message', async function (assert) {
      allScenarios.variableTestCluster(server);
      const variablesToken = server.db.tokens.find(LIMITED_VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      await visit(
        `/jobs/${server.db.jobs[1].id}@${server.db.jobs[1].namespace}`
      );
      assert.dom('[data-test-tab="variables"]').exists();
      await click('[data-test-tab="variables"] a');
      assert.equal(
        currentURL(),
        `/jobs/${server.db.jobs[1].id}@${server.db.jobs[1].namespace}/variables`
      );
      assert.dom('[data-test-no-auto-vars-message]').exists();
      assert.dom('[data-test-create-variable-button]').doesNotExist();

      window.localStorage.nomadTokenSecret = null; // Reset Token
    });

    test('If the user has variable write access, but no variables, the subnav exists but contains only a message and a create button', async function (assert) {
      assert.expect(4);
      allScenarios.variableTestCluster(server);
      const variablesToken = server.db.tokens.find(VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      await visit(
        `/jobs/${server.db.jobs[1].id}@${server.db.jobs[1].namespace}`
      );
      assert.dom('[data-test-tab="variables"]').exists();
      await click('[data-test-tab="variables"] a');
      assert.equal(
        currentURL(),
        `/jobs/${server.db.jobs[1].id}@${server.db.jobs[1].namespace}/variables`
      );
      assert.dom('[data-test-no-auto-vars-message]').exists();
      assert.dom('[data-test-create-variable-button]').exists();

      await percySnapshot(assert);
      window.localStorage.nomadTokenSecret = null; // Reset Token
    });

    test('If the user has variable read access, and variables, the subnav exists and contains a list of variables', async function (assert) {
      allScenarios.variableTestCluster(server);
      const variablesToken = server.db.tokens.find(LIMITED_VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;

      // in variablesTestCluster, job0 has path-linked variables, others do not.
      await visit(
        `/jobs/${server.db.jobs[0].id}@${server.db.jobs[0].namespace}`
      );
      assert.dom('[data-test-tab="variables"]').exists();
      await click('[data-test-tab="variables"] a');
      assert.equal(
        currentURL(),
        `/jobs/${server.db.jobs[0].id}@${server.db.jobs[0].namespace}/variables`
      );
      assert.dom('[data-test-file-row]').exists({ count: 3 });
      window.localStorage.nomadTokenSecret = null; // Reset Token
    });

    test('The nomad/jobs variable is always included, if it exists', async function (assert) {
      allScenarios.variableTestCluster(server);
      const variablesToken = server.db.tokens.find(LIMITED_VARIABLE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;

      server.create('variable', {
        id: 'nomad/jobs',
        keyValues: [],
      });

      // in variablesTestCluster, job0 has path-linked variables, others do not.
      await visit(
        `/jobs/${server.db.jobs[1].id}@${server.db.jobs[1].namespace}`
      );
      assert.dom('[data-test-tab="variables"]').exists();
      await click('[data-test-tab="variables"] a');
      assert.equal(
        currentURL(),
        `/jobs/${server.db.jobs[1].id}@${server.db.jobs[1].namespace}/variables`
      );
      assert.dom('[data-test-file-row]').exists({ count: 1 });
      assert.dom('[data-test-file-row="nomad/jobs"]').exists();
    });

    test('Multiple task variables are included, and make a maximum of 1 API request', async function (assert) {
      //#region setup
      server.create('node-pool');
      server.create('node');
      let token = server.create('token', { type: 'management' });
      let job = server.create('job', {
        createAllocations: true,
        groupTaskCount: 10,
        resourceSpec: Array(3).fill('M: 257, C: 500'), // 3 groups
        shallow: false,
        name: 'test-job',
        id: 'test-job',
        type: 'service',
        activeDeployment: false,
        namespaceId: 'default',
      });

      server.create('variable', {
        id: 'nomad/jobs',
        keyValues: [],
      });
      server.create('variable', {
        id: 'nomad/jobs/test-job',
        keyValues: [],
      });
      // Create a variable for each task

      server.db.tasks.forEach((task) => {
        let groupName = server.db.taskGroups.findBy(
          (group) => group.id === task.taskGroupId
        ).name;
        server.create('variable', {
          id: `nomad/jobs/test-job/${groupName}/${task.name}`,
          keyValues: [],
        });
      });
      window.localStorage.nomadTokenSecret = token.secretId;

      //#endregion setup

      //#region operation
      await visit(`/jobs/${job.id}@${job.namespace}/variables`);

      // 2 requests: one for the main nomad/vars variable, and one for a prefix of job name
      let requests = server.pretender.handledRequests.filter(
        (request) =>
          request.url === '/v1/vars?path=nomad%2Fjobs' ||
          request.url === `/v1/vars?prefix=nomad%2Fjobs%2F${job.name}`
      );
      assert.equal(requests.length, 2);

      // Should see 32 rows: nomad/jobs, job-name, and 30 task variables
      assert.dom('[data-test-file-row]').exists({ count: 32 });
      //#endregion operation

      window.localStorage.nomadTokenSecret = null; // Reset Token
    });

    // Test: Intro text shows examples of variables at groups and tasks
    test('The intro text shows examples of variables at groups and tasks', async function (assert) {
      //#region setup
      server.create('node-pool');
      server.create('node');
      let token = server.create('token', { type: 'management' });
      let job = server.create('job', {
        createAllocations: true,
        groupTaskCount: 2,
        resourceSpec: Array(1).fill('M: 257, C: 500'), // 1 group
        shallow: false,
        name: 'test-job',
        id: 'test-job',
        type: 'service',
        activeDeployment: false,
        namespaceId: 'default',
      });
      server.create('variable', {
        id: 'nomad/jobs/test-job',
        keyValues: [],
      });
      // Create a variable for each taskGroup
      server.db.taskGroups.forEach((group) => {
        server.create('variable', {
          id: `nomad/jobs/test-job/${group.name}`,
          keyValues: [],
        });
      });

      window.localStorage.nomadTokenSecret = token.secretId;

      //#endregion setup

      await visit(`/jobs/${job.id}@${job.namespace}`);
      assert.dom('[data-test-tab="variables"]').exists();
      await click('[data-test-tab="variables"] a');
      assert.equal(currentURL(), `/jobs/${job.id}@${job.namespace}/variables`);

      assert.dom('.job-variables-intro').exists();

      // All-jobs reminder is there, link is to create a new variable
      assert.dom('[data-test-variables-intro-all-jobs]').exists();
      assert.dom('[data-test-variables-intro-all-jobs] a').exists();
      assert
        .dom('[data-test-variables-intro-all-jobs] a')
        .hasAttribute('href', '/ui/variables/new?path=nomad%2Fjobs');

      // This-job reminder is there, and since the variable exists, link is to edit it
      assert.dom('[data-test-variables-intro-job]').exists();
      assert.dom('[data-test-variables-intro-job] a').exists();
      assert
        .dom('[data-test-variables-intro-job] a')
        .hasAttribute(
          'href',
          `/ui/variables/var/nomad/jobs/${job.id}@${job.namespace}/edit`
        );

      // Group reminder is there, and since the variable exists, link is to edit it
      assert.dom('[data-test-variables-intro-groups]').exists();
      assert.dom('[data-test-variables-intro-groups] a').exists({ count: 1 });
      assert
        .dom('[data-test-variables-intro-groups]')
        .doesNotContainText('etc.');
      assert
        .dom('[data-test-variables-intro-groups] a')
        .hasAttribute(
          'href',
          `/ui/variables/var/nomad/jobs/${job.id}/${server.db.taskGroups[0].name}@${job.namespace}/edit`
        );

      // Task reminder is there, and variables don't exist, so link is to create them, plus etc. reminder text
      assert.dom('[data-test-variables-intro-tasks]').exists();
      assert.dom('[data-test-variables-intro-tasks] a').exists({ count: 2 });
      assert.dom('[data-test-variables-intro-tasks]').containsText('etc.');
      assert
        .dom('[data-test-variables-intro-tasks] code:nth-of-type(1) a')
        .hasAttribute(
          'href',
          `/ui/variables/new?path=nomad%2Fjobs%2F${job.id}%2F${server.db.taskGroups[0].name}%2F${server.db.tasks[0].name}`
        );
      assert
        .dom('[data-test-variables-intro-tasks] code:nth-of-type(2) a')
        .hasAttribute(
          'href',
          `/ui/variables/new?path=nomad%2Fjobs%2F${job.id}%2F${server.db.taskGroups[0].name}%2F${server.db.tasks[1].name}`
        );
    });
  });
});
