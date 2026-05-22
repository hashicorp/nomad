/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { a11yAudit } from 'ember-a11y-testing/test-support';
import { visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import { allScenarios } from '../../mirage/scenarios/default';

module('Acceptance | administration tokens', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
    allScenarios.rolesTestCluster(this.server);
    const managementToken = this.server.db.tokens.findBy({
      type: 'management',
    });
    window.localStorage.nomadTokenSecret = managementToken.secretId;
  });

  hooks.afterEach(function () {
    window.localStorage.clear();
  });

  test('administration.tokens passes an accessibility audit', async function (assert) {
    await visit('/administration/tokens');
    await a11yAudit();
    assert.ok(true, 'no a11y errors found');
  });

  test('administration.tokens.new passes an accessibility audit', async function (assert) {
    await visit('/administration/tokens/new');
    await a11yAudit();
    assert.ok(true, 'no a11y errors found');
  });

  test('administration.tokens.token passes an accessibility audit', async function (assert) {
    const token = this.server.db.tokens.findBy({ type: 'management' });
    await visit(`/administration/tokens/${token.id}`);
    await a11yAudit();
    assert.ok(true, 'no a11y errors found');
  });
});
