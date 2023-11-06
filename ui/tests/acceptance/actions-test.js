/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
// @ts-check
import { module, test } from 'qunit';
import { visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { allScenarios } from '../../mirage/scenarios/default';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Tokens from 'nomad-ui/tests/pages/settings/tokens';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';

// let managementToken;
// let noExecToken;

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
      // policyIds: [clientReaderPolicy.id],
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

    await visit('/jobs/action-having-job@default');
    // no actions dropdown by default
    assert
      .dom('.actions-dropdown')
      .doesNotExist('Signed out user does not see an Actions dropdown'); // in neither title nor task sub row
    await Tokens.visit();
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();
    await visit('/jobs/action-having-job@default');
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
    await visit('/jobs/action-having-job@default');
    assert
      .dom('.actions-dropdown')
      .doesNotExist('Non-management token does not see it');

    // Sign out and sign back in as a token with alloc exec
    await Tokens.visit();
    await Tokens.clear();
    await Tokens.secret(allocExecToken.secretId).submit();
    await visit('/jobs/action-having-job@default');
    assert
      .dom('.job-page-header .actions-dropdown')
      .exists('Alloc exec token sees it');
    assert
      .dom('td[data-test-actions] .actions-dropdown')
      .exists('Exists within Task Sub-Row');
  });
});
