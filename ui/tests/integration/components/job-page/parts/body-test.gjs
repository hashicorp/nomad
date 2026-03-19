/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { find, findAll, render } from '@ember/test-helpers';
import { startMirage } from 'nomad-ui/tests/helpers/start-mirage';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import JobPagePartsBody from 'nomad-ui/components/job-page/parts/body';

module('Integration | Component | job-page/parts/body', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
    this.server = startMirage();
    this.server.createList('namespace', 3);
  });

  hooks.afterEach(function () {
    this.server.shutdown();
    window.localStorage.clear();
  });

  test('includes a subnav for the job', async function (assert) {
    const job = {};

    await render(
      <template>
        <JobPagePartsBody @job={{job}}>
          <div class="inner-content">Inner content</div>
        </JobPagePartsBody>
      </template>,
    );
    assert.ok(find('[data-test-subnav="job"]'), 'Job subnav is rendered');
  });

  test('the subnav includes the deployments link when the job is a service', async function (assert) {
    const store = this.owner.lookup('service:store');
    const job = await store.createRecord('job', {
      id: '["service-job","default"]',
      type: 'service',
    });

    await render(
      <template>
        <JobPagePartsBody @job={{job}}>
          <div class="inner-content">Inner content</div>
        </JobPagePartsBody>
      </template>,
    );

    const subnavLabels = findAll('[data-test-tab]').map((anchor) =>
      anchor.textContent.trim(),
    );
    assert.ok(
      subnavLabels.some((label) => label === 'Definition'),
      'Definition link',
    );
    assert.ok(
      subnavLabels.some((label) => label === 'Versions'),
      'Versions link',
    );

    assert.ok(
      subnavLabels.some((label) => label === 'Deployments'),
      'Deployments link',
    );

    await componentA11yAudit(this.element, assert);
  });

  test('the subnav does not include the deployments link when the job is not a service', async function (assert) {
    const store = this.owner.lookup('service:store');
    const job = await store.createRecord('job', {
      id: '["batch-job","default"]',
      type: 'batch',
    });

    await render(
      <template>
        <JobPagePartsBody @job={{job}}>
          <div class="inner-content">Inner content</div>
        </JobPagePartsBody>
      </template>,
    );

    const subnavLabels = findAll('[data-test-tab]').map((anchor) =>
      anchor.textContent.trim(),
    );
    assert.ok(
      subnavLabels.some((label) => label === 'Definition'),
      'Definition link',
    );
    assert.ok(
      subnavLabels.some((label) => label === 'Versions'),
      'Versions link',
    );
    assert.notOk(
      subnavLabels.some((label) => label === 'Deployments'),
      'Deployments link',
    );
  });

  test('body yields content to a section after the subnav', async function (assert) {
    const job = {};

    await render(
      <template>
        <JobPagePartsBody @job={{job}}>
          <div class="inner-content">Inner content</div>
        </JobPagePartsBody>
      </template>,
    );

    assert.ok(
      find('[data-test-subnav="job"] + .section > .inner-content'),
      'Content is rendered immediately after the subnav',
    );
  });
});
