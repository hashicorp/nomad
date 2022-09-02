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

import Variables from 'nomad-ui/tests/pages/variables';
import Layout from 'nomad-ui/tests/pages/layout';

const VARIABLE_TOKEN_ID = '53cur3-v4r14bl35';
const LIMITED_VARIABLE_TOKEN_ID = 'f3w3r-53cur3-v4r14bl35';

module('Acceptance | variables', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);
  hooks.beforeEach(async function () {
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
    assert.expect(23);
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
  });

  test('it does not allow you to save if you lack Items', async function (assert) {
    assert.expect(5);
    allScenarios.variableTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await Variables.visitNew();
    assert.equal(currentURL(), '/variables/new');
    await typeIn('.path-input', 'foo/bar');
    await click('button[type="submit"]');
    assert.dom('.flash-message.alert-error').exists();
    await click('.flash-message.alert-error .close-button');
    assert.dom('.flash-message.alert-error').doesNotExist();

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
});
