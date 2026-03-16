/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { find, render, settled, triggerEvent } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { hbs } from 'ember-cli-htmlbars';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import sinon from 'sinon';
import { create } from 'ember-cli-page-object';
import stepperInput from 'nomad-ui/tests/pages/components/stepper-input';

const StepperInput = create(stepperInput());

module('Integration | Component | stepper input', function (hooks) {
  setupRenderingTest(hooks);

  const commonProperties = () => ({
    min: 0,
    max: 10,
    value: 5,
    label: 'Stepper',
    classVariant: 'is-primary',
    disabled: false,
    onChange: sinon.spy(),
  });

  const commonTemplate = hbs`
    <StepperInput
      @debounce="50"
      @min={{this.min}}
      @max={{this.max}}
      @value={{this.value}}
      @class={{this.classVariant}}
      @disabled={{this.disabled}}
      @onChange={{this.onChange}}>
      {{this.label}}
    </StepperInput>
  `;

  test('basic appearance includes a label, an input, and two buttons', async function (assert) {
    this.setProperties(commonProperties());

    await render(commonTemplate);

    assert.strictEqual(StepperInput.label, this.label);
    assert.strictEqual(Number(StepperInput.input.value), this.value);
    assert.ok(StepperInput.decrement.isPresent);
    assert.ok(StepperInput.increment.isPresent);
    assert.ok(
      StepperInput.decrement.classNames.split(' ').includes(this.classVariant),
    );
    assert.ok(
      StepperInput.increment.classNames.split(' ').includes(this.classVariant),
    );

    await componentA11yAudit(this.element, assert);
  });

  test('clicking the increment and decrement buttons immediately changes the shown value in the input but debounces the onUpdate call', async function (assert) {
    this.setProperties(commonProperties());

    const baseValue = this.value;

    await render(commonTemplate);

    const incrementButton = find('[data-test-stepper-increment]');
    const decrementButton = find('[data-test-stepper-decrement]');

    incrementButton.click();
    assert.strictEqual(Number(StepperInput.input.value), baseValue + 1);
    assert.strictEqual(this.onChange.callCount, 0);

    decrementButton.click();
    assert.strictEqual(Number(StepperInput.input.value), baseValue);
    assert.strictEqual(this.onChange.callCount, 0);

    decrementButton.click();
    assert.strictEqual(Number(StepperInput.input.value), baseValue - 1);
    assert.strictEqual(this.onChange.callCount, 0);

    await settled();
    assert.ok(this.onChange.calledOnceWithExactly(baseValue - 1));
  });

  test('the increment button is disabled when the internal value is the max value', async function (assert) {
    this.setProperties(commonProperties());
    this.set('value', this.max);

    await render(commonTemplate);

    assert.ok(StepperInput.increment.isDisabled);
  });

  test('the decrement button is disabled when the internal value is the min value', async function (assert) {
    this.setProperties(commonProperties());
    this.set('value', this.min);

    await render(commonTemplate);

    assert.ok(StepperInput.decrement.isDisabled);
  });

  test('the text input does not call the onUpdate function on oninput', async function (assert) {
    this.setProperties(commonProperties());
    const newValue = 8;

    await render(commonTemplate);

    const input = find('[data-test-stepper-input]');

    input.value = newValue;
    assert.strictEqual(Number(StepperInput.input.value), newValue);
    assert.notOk(this.onChange.called);

    await triggerEvent(input, 'input');
    assert.strictEqual(Number(StepperInput.input.value), newValue);
    assert.notOk(this.onChange.called);

    await triggerEvent(input, 'change');
    assert.strictEqual(Number(StepperInput.input.value), newValue);
    assert.ok(this.onChange.calledWith(newValue));
  });

  test('the text input does call the onUpdate function on onchange', async function (assert) {
    this.setProperties(commonProperties());
    const newValue = 8;

    await render(commonTemplate);

    await StepperInput.input.fill(newValue);

    await settled();
    assert.strictEqual(Number(StepperInput.input.value), newValue);
    assert.ok(this.onChange.calledWith(newValue));
  });

  test('text input limits input to the bounds of the min/max range', async function (assert) {
    this.setProperties(commonProperties());
    let newValue = this.max + 1;

    await render(commonTemplate);

    await StepperInput.input.fill(newValue);
    await settled();

    assert.strictEqual(Number(StepperInput.input.value), this.max);
    assert.ok(this.onChange.calledWith(this.max));

    newValue = this.min - 1;

    await StepperInput.input.fill(newValue);
    await settled();

    assert.strictEqual(Number(StepperInput.input.value), this.min);
    assert.ok(this.onChange.calledWith(this.min));
  });

  test('pressing ESC in the text input reverts the text value back to the current value', async function (assert) {
    this.setProperties(commonProperties());
    const newValue = 8;

    await render(commonTemplate);

    const input = find('[data-test-stepper-input]');

    input.value = newValue;
    assert.strictEqual(Number(StepperInput.input.value), newValue);

    await StepperInput.input.esc();
    assert.strictEqual(Number(StepperInput.input.value), this.value);
  });

  test('clicking the label focuses in the input', async function (assert) {
    this.setProperties(commonProperties());

    await render(commonTemplate);
    await StepperInput.clickLabel();

    const input = find('[data-test-stepper-input]');
    assert.strictEqual(document.activeElement, input);
  });

  test('focusing the input selects the input value', async function (assert) {
    this.setProperties(commonProperties());

    await render(commonTemplate);
    await StepperInput.input.focus();

    assert.strictEqual(
      window.getSelection().toString().trim(),
      this.value.toString(),
    );
  });

  test('entering a fractional value floors the value', async function (assert) {
    this.setProperties(commonProperties());
    const newValue = 3.14159;

    await render(commonTemplate);

    await StepperInput.input.fill(newValue);

    await settled();
    assert.strictEqual(Number(StepperInput.input.value), Math.floor(newValue));
    assert.ok(this.onChange.calledWith(Math.floor(newValue)));
  });

  test('entering an invalid value reverts the value', async function (assert) {
    this.setProperties(commonProperties());
    const newValue = 'NaN';

    await render(commonTemplate);

    await StepperInput.input.fill(newValue);

    await settled();
    assert.strictEqual(Number(StepperInput.input.value), this.value);
    assert.notOk(this.onChange.called);
  });
});
