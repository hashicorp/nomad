/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { a11yAudit } from 'ember-a11y-testing/test-support';
import { visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';

module('Acceptance | not found', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    this.server.create('agent');
  });

  test('not-found route passes an accessibility audit', async function (assert) {
    await visit('/this-route-definitely-does-not-exist');
    await a11yAudit();
    assert.ok(true, 'no a11y errors found');
  });
});
