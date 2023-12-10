/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import sinon from 'sinon';

module('Integration | Component | das/dismissed', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
  });

  test('it renders the dismissal interstitial with a button to proceed and an option to never show again and proceeds manually', async function (assert) {
    assert.expect(3);

    const proceedSpy = sinon.spy();
    this.set('proceedSpy', proceedSpy);

    await render(hbs`<Das::Dismissed @proceed={{proceedSpy}} />`);

    await componentA11yAudit(this.element, assert);

    await click('input[type=checkbox]');
    await click('[data-test-understood]');

    assert.ok(proceedSpy.calledWith({ manuallyDismissed: true }));
    assert.equal(
      window.localStorage.getItem('nomadRecommendationDismssalUnderstood'),
      'true'
    );
  });

  test('it renders the dismissal interstitial with no button when the option to never show again has been chosen and proceeds automatically', async function (assert) {
    assert.expect(3);

    window.localStorage.setItem('nomadRecommendationDismssalUnderstood', true);

    const proceedSpy = sinon.spy();
    this.set('proceedSpy', proceedSpy);

    await render(hbs`<Das::Dismissed @proceed={{proceedSpy}} />`);

    assert.dom('[data-test-understood]').doesNotExist();

    await componentA11yAudit(this.element, assert);

    assert.ok(proceedSpy.calledWith({ manuallyDismissed: false }));
  });
});
