/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
// @ts-check
import { module, test } from 'qunit';
import { visit, click } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { allScenarios } from '../../mirage/scenarios/default';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Tokens from 'nomad-ui/tests/pages/settings/tokens';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import percySnapshot from '@percy/ember';

module('Acceptance | actions', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
  });

  test('Actions show up on the Job Index page, permissions allowing', async function (assert) {
    assert.expect(7);
    allScenarios.smallCluster(server);
    let managementToken = server.create('token', {
      type: 'management',
      name: 'Management Token',
    });

    let clientReaderToken = server.create('token', {
      type: 'client',
      name: "N. O'DeReader",
    });

    const allocExecPolicy = server.create('policy', {
      id: 'alloc-exec',
      rules: `
        namespace "*" {
          policy = "read"
          capabilities = ["list-jobs", "alloc-exec", "read-logs"]
        }
      `,
      rulesJSON: {
        Namespaces: [
          {
            Capabilities: ['list-jobs', 'alloc-exec', 'read-logs'],
            Name: '*',
          },
        ],
      },
    });

    let allocExecToken = server.create('token', {
      type: 'client',
      name: 'Alloc Exec Token',
      policyIds: [allocExecPolicy.id],
    });

    await visit('/jobs/actionable-job@default');
    // no actions dropdown by default
    assert
      .dom('.actions-dropdown')
      .doesNotExist('Signed out user does not see an Actions dropdown'); // in neither title nor task sub row
    await Tokens.visit();
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();
    await visit('/jobs/actionable-job@default');
    assert
      .dom('.job-page-header .actions-dropdown')
      .exists('Management token sees it');
    assert
      .dom('td[data-test-actions] .actions-dropdown')
      .exists('Exists within Task Sub-Row');
    await a11yAudit(assert);

    // Sign out and sign back in as a token without alloc exec
    await Tokens.visit();
    await Tokens.clear();
    await Tokens.secret(clientReaderToken.secretId).submit();
    await visit('/jobs/actionable-job@default');
    assert
      .dom('.actions-dropdown')
      .doesNotExist('Non-management token does not see it');

    // Sign out and sign back in as a token with alloc exec
    await Tokens.visit();
    await Tokens.clear();
    await Tokens.secret(allocExecToken.secretId).submit();
    await visit('/jobs/actionable-job@default');
    assert
      .dom('.job-page-header .actions-dropdown')
      .exists('Alloc exec token sees it');
    assert
      .dom('td[data-test-actions] .actions-dropdown')
      .exists('Exists within Task Sub-Row');
  });

  // Running actions test
  test('Running actions and notifications', async function (assert) {
    assert.expect(14);
    allScenarios.smallCluster(server);
    let managementToken = server.create('token', {
      type: 'management',
      name: 'Management Token',
    });

    await Tokens.visit();
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();
    await visit('/jobs/actionable-job@default');
    assert.dom('.job-page-header .actions-dropdown').exists();

    // Open the dropdown
    await click('.job-page-header .actions-dropdown button');
    // 5 actions should show up: 2 from each of the 5-alloc task-groups and one single-alloc action
    assert
      .dom('.job-page-header .actions-dropdown .hds-dropdown__list')
      .exists();
    assert
      .dom('.job-page-header .actions-dropdown .hds-dropdown__list li')
      .exists({ count: 5 });

    // First four should be dropdowns, last one should be a button
    assert
      .dom(
        '.job-page-header .actions-dropdown .hds-dropdown__list li.hds-dropdown-list-item--variant-generic'
      )
      .exists({ count: 4 });
    assert
      .dom(
        '.job-page-header .actions-dropdown .hds-dropdown__list li.hds-dropdown-list-item--variant-interactive'
      )
      .exists({ count: 1 });

    // Click the first one; should open a further dropdown
    assert
      .dom(
        '.job-page-header .actions-dropdown .hds-dropdown__list li:nth-of-type(1) button'
      )
      .hasAttribute('aria-expanded', 'false');
    assert
      .dom(
        '.job-page-header .actions-dropdown .hds-disclosure-primitive__content'
      )
      .doesNotExist('Sub-dropdown does not yet exist');
    await click(
      '.job-page-header .actions-dropdown .hds-dropdown__list li:nth-of-type(1) button'
    );
    assert
      .dom(
        '.job-page-header .actions-dropdown .hds-dropdown__list li:nth-of-type(1) button'
      )
      .hasAttribute('aria-expanded', 'true');

    assert
      .dom(
        '.job-page-header .actions-dropdown .hds-disclosure-primitive__content'
      )
      .exists('Sub-dropdown exists after clicking parent reveal toggle');

    await percySnapshot(assert);

    // run on a random alloc
    await click(
      '.job-page-header .actions-dropdown .hds-disclosure-primitive__content li:nth-of-type(1) button'
    );

    assert
      .dom('.hds-toast')
      .exists(
        { count: 1 },
        'A toast notification pops up upon running an action'
      );
    assert
      .dom('.hds-toast code')
      .containsText(
        'Message Received',
        'The notification contains the message from the action'
      );
    assert.dom('.hds-toast .hds-alert__title').containsText('Finished');

    // run on all allocs
    await click(
      '.job-page-header .actions-dropdown .hds-disclosure-primitive__content li:nth-of-type(2) button'
    );
    assert
      .dom('.hds-toast')
      .exists(
        { count: 6 },
        'Running on all allocs in the group (5) results in 6 total toasts'
      );

    // Click the orphan alloc action
    await click(
      '.job-page-header .actions-dropdown .hds-dropdown__list li:nth-of-type(5) button'
    );
    assert
      .dom('.hds-toast')
      .exists(
        { count: 7 },
        'It contains no dropdown, just a button, and should run 1 further action/toast'
      );

    await percySnapshot(assert);
  });
});
