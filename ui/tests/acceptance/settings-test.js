/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { a11yAudit } from 'ember-a11y-testing/test-support';
import { visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';

module('Acceptance | settings', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
    this.server.create('agent');
  });

  test('settings index passes an accessibility audit', async function (assert) {
    await visit('/settings');
    await a11yAudit();
    assert.ok(true, 'no a11y errors found');
  });

  test('settings.tokens passes an accessibility audit', async function (assert) {
    await visit('/settings/tokens');
    await a11yAudit();
    assert.ok(true, 'no a11y errors found');
  });

  test('settings.user-settings passes an accessibility audit', async function (assert) {
    await visit('/settings/user-settings');
    await a11yAudit();
    assert.ok(true, 'no a11y errors found');
  });
});
