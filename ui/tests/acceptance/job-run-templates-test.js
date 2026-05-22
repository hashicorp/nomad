/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { a11yAudit } from 'ember-a11y-testing/test-support';
import { visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';

module('Acceptance | job run templates', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
    this.server.create('agent');
    this.server.create('node-pool');
    this.server.create('node');
    const managementToken = this.server.create('token', { type: 'management' });
    window.localStorage.nomadTokenSecret = managementToken.secretId;
  });

  hooks.afterEach(function () {
    window.localStorage.clear();
  });

  test('jobs.run.templates passes an accessibility audit', async function (assert) {
    await visit('/jobs/run/templates');
    await a11yAudit();
    assert.ok(true, 'no a11y errors found');
  });

  test('jobs.run.templates.new passes an accessibility audit', async function (assert) {
    await visit('/jobs/run/templates/new');
    await a11yAudit();
    assert.ok(true, 'no a11y errors found');
  });

  test('jobs.run.templates.manage passes an accessibility audit', async function (assert) {
    await visit('/jobs/run/templates/manage');
    await a11yAudit();
    assert.ok(true, 'no a11y errors found');
  });
});
