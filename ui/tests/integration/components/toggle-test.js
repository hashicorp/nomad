/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { find, render, settled } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import sinon from 'sinon';
import { create } from 'ember-cli-page-object';
import togglePageObject from 'nomad-ui/tests/pages/components/toggle';

const Toggle = create(togglePageObject());

module('Integration | Component | toggle', function (hooks) {
  setupRenderingTest(hooks);

  const commonProperties = () => ({
    isActive: false,
    isDisabled: false,
    label: 'Label',
    onToggle: sinon.spy(),
  });

  const commonTemplate = hbs`
    <Toggle
      @isActive={{isActive}}
      @isDisabled={{isDisabled}}
      @onToggle={{onToggle}}>
      {{label}}
    </Toggle>
  `;

  test('presents as a label with an inner checkbox and display span, and text', async function (assert) {
    assert.expect(7);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    assert.equal(Toggle.label, props.label, `Label should be ${props.label}`);
    assert.ok(Toggle.isPresent);
    assert.notOk(Toggle.isActive);
    assert.ok(find('[data-test-toggler]'));
    assert.equal(
      find('[data-test-input]').tagName.toLowerCase(),
      'input',
      'The input is a real HTML input'
    );
    assert.equal(
      find('[data-test-input]').getAttribute('type'),
      'checkbox',
      'The input type is checkbox'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('the isActive property dictates the active state and class', async function (assert) {
    assert.expect(5);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    assert.notOk(Toggle.isActive);
    assert.notOk(Toggle.hasActiveClass);

    this.set('isActive', true);
    await settled();

    assert.ok(Toggle.isActive);
    assert.ok(Toggle.hasActiveClass);

    await componentA11yAudit(this.element, assert);
  });

  test('the isDisabled property dictates the disabled state and class', async function (assert) {
    assert.expect(5);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    assert.notOk(Toggle.isDisabled);
    assert.notOk(Toggle.hasDisabledClass);

    this.set('isDisabled', true);
    await settled();

    assert.ok(Toggle.isDisabled);
    assert.ok(Toggle.hasDisabledClass);

    await componentA11yAudit(this.element, assert);
  });

  test('toggling the input calls the onToggle action', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await Toggle.toggle();
    assert.equal(props.onToggle.callCount, 1);
  });
});
