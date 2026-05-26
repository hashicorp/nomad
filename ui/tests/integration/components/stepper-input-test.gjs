/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { find, render, settled, triggerEvent } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import sinon from 'sinon';
import { create } from 'ember-cli-page-object';
import StepperInputComponent from 'nomad-ui/components/stepper-input';
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

  const renderStepperInput = async (props) => {
    await render(
      <template>
        <StepperInputComponent
          @debounce="50"
          @min={{props.min}}
          @max={{props.max}}
          @value={{props.value}}
          @class={{props.classVariant}}
          @disabled={{props.disabled}}
          @onChange={{props.onChange}}
        >
          {{props.label}}
        </StepperInputComponent>
      </template>,
    );
  };

  test('basic appearance includes a label, an input, and two buttons', async function (assert) {
    const props = commonProperties();

    await renderStepperInput(props);

    assert.strictEqual(StepperInput.label, props.label);
    assert.strictEqual(Number(StepperInput.input.value), props.value);
    assert.ok(StepperInput.decrement.isPresent);
    assert.ok(StepperInput.increment.isPresent);
    assert.ok(
      StepperInput.decrement.classNames.split(' ').includes(props.classVariant),
    );
    assert.ok(
      StepperInput.increment.classNames.split(' ').includes(props.classVariant),
    );

    await componentA11yAudit(this.element, assert);
  });

  test('clicking the increment and decrement buttons immediately changes the shown value in the input but debounces the onUpdate call', async function (assert) {
    const props = commonProperties();

    const baseValue = props.value;

    await renderStepperInput(props);

    const incrementButton = find('[data-test-stepper-increment]');
    const decrementButton = find('[data-test-stepper-decrement]');

    incrementButton.click();
    assert.strictEqual(Number(StepperInput.input.value), baseValue + 1);
    assert.strictEqual(props.onChange.callCount, 0);

    decrementButton.click();
    assert.strictEqual(Number(StepperInput.input.value), baseValue);
    assert.strictEqual(props.onChange.callCount, 0);

    decrementButton.click();
    assert.strictEqual(Number(StepperInput.input.value), baseValue - 1);
    assert.strictEqual(props.onChange.callCount, 0);

    await settled();
    assert.ok(props.onChange.calledOnceWithExactly(baseValue - 1));
  });

  test('the increment button is disabled when the internal value is the max value', async function (assert) {
    const props = commonProperties();
    props.value = props.max;

    await renderStepperInput(props);

    assert.ok(StepperInput.increment.isDisabled);
  });

  test('the decrement button is disabled when the internal value is the min value', async function (assert) {
    const props = commonProperties();
    props.value = props.min;

    await renderStepperInput(props);

    assert.ok(StepperInput.decrement.isDisabled);
  });

  test('the text input does not call the onUpdate function on oninput', async function (assert) {
    const props = commonProperties();
    const newValue = 8;

    await renderStepperInput(props);

    const input = find('[data-test-stepper-input]');

    input.value = newValue;
    assert.strictEqual(Number(StepperInput.input.value), newValue);
    assert.notOk(props.onChange.called);

    await triggerEvent(input, 'input');
    assert.strictEqual(Number(StepperInput.input.value), newValue);
    assert.notOk(props.onChange.called);

    await triggerEvent(input, 'change');
    assert.strictEqual(Number(StepperInput.input.value), newValue);
    assert.ok(props.onChange.calledWith(newValue));
  });

  test('the text input does call the onUpdate function on onchange', async function (assert) {
    const props = commonProperties();
    const newValue = 8;

    await renderStepperInput(props);

    await StepperInput.input.fill(newValue);

    await settled();
    assert.strictEqual(Number(StepperInput.input.value), newValue);
    assert.ok(props.onChange.calledWith(newValue));
  });

  test('text input limits input to the bounds of the min/max range', async function (assert) {
    const props = commonProperties();
    let newValue = props.max + 1;

    await renderStepperInput(props);

    await StepperInput.input.fill(newValue);
    await settled();

    assert.strictEqual(Number(StepperInput.input.value), props.max);
    assert.ok(props.onChange.calledWith(props.max));

    newValue = props.min - 1;

    await StepperInput.input.fill(newValue);
    await settled();

    assert.strictEqual(Number(StepperInput.input.value), props.min);
    assert.ok(props.onChange.calledWith(props.min));
  });

  test('pressing ESC in the text input reverts the text value back to the current value', async function (assert) {
    const props = commonProperties();
    const newValue = 8;

    await renderStepperInput(props);

    const input = find('[data-test-stepper-input]');

    input.value = newValue;
    assert.strictEqual(Number(StepperInput.input.value), newValue);

    await StepperInput.input.esc();
    assert.strictEqual(Number(StepperInput.input.value), props.value);
  });

  test('clicking the label focuses in the input', async function (assert) {
    const props = commonProperties();

    await renderStepperInput(props);
    await StepperInput.clickLabel();

    const input = find('[data-test-stepper-input]');
    assert.strictEqual(document.activeElement, input);
  });

  test('focusing the input selects the input value', async function (assert) {
    const props = commonProperties();

    await renderStepperInput(props);
    await StepperInput.input.focus();

    assert.strictEqual(
      window.getSelection().toString().trim(),
      props.value.toString(),
    );
  });

  test('entering a fractional value floors the value', async function (assert) {
    const props = commonProperties();
    const newValue = 3.14159;

    await renderStepperInput(props);

    await StepperInput.input.fill(newValue);

    await settled();
    assert.strictEqual(Number(StepperInput.input.value), Math.floor(newValue));
    assert.ok(props.onChange.calledWith(Math.floor(newValue)));
  });

  test('entering an invalid value reverts the value', async function (assert) {
    const props = commonProperties();
    const newValue = 'NaN';

    await renderStepperInput(props);

    await StepperInput.input.fill(newValue);

    await settled();
    assert.strictEqual(Number(StepperInput.input.value), props.value);
    assert.notOk(props.onChange.called);
  });
});
