/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { find, click, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import sinon from 'sinon';
import { create } from 'ember-cli-page-object';
import twoStepButton from 'nomad-ui/tests/pages/components/two-step-button';

const TwoStepButton = create(twoStepButton());

module('Integration | Component | two step button', function (hooks) {
  setupRenderingTest(hooks);

  const commonProperties = () => ({
    idleText: 'Idle State Button',
    cancelText: 'Cancel Action',
    confirmText: 'Confirm Action',
    confirmationMessage: 'Are you certain',
    awaitingConfirmation: false,
    disabled: false,
    onConfirm: sinon.spy(),
    onCancel: sinon.spy(),
  });

  const commonTemplate = hbs`
    <TwoStepButton
      @idleText={{idleText}}
      @cancelText={{cancelText}}
      @confirmText={{confirmText}}
      @confirmationMessage={{confirmationMessage}}
      @awaitingConfirmation={{awaitingConfirmation}}
      @disabled={{disabled}}
      @onConfirm={{onConfirm}}
      @onCancel={{onCancel}} />
  `;

  test('presents as a button in the idle state', async function (assert) {
    assert.expect(6);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    assert.ok(find('[data-test-idle-button]'), 'Idle button is rendered');
    assert.equal(
      TwoStepButton.idleText,
      props.idleText,
      'Button is labeled correctly'
    );

    assert.notOk(find('[data-test-cancel-button]'), 'No cancel button yet');
    assert.notOk(find('[data-test-confirm-button]'), 'No confirm button yet');
    assert.notOk(
      find('[data-test-confirmation-message]'),
      'No confirmation message yet'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('clicking the idle state button transitions into the promptForConfirmation state', async function (assert) {
    assert.expect(7);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await TwoStepButton.idle();

    assert.ok(find('[data-test-cancel-button]'), 'Cancel button is rendered');
    assert.equal(
      TwoStepButton.cancelText,
      props.cancelText,
      'Button is labeled correctly'
    );

    assert.ok(find('[data-test-confirm-button]'), 'Confirm button is rendered');
    assert.equal(
      TwoStepButton.confirmText,
      props.confirmText,
      'Button is labeled correctly'
    );

    assert.equal(
      TwoStepButton.confirmationMessage,
      props.confirmationMessage,
      'Confirmation message is shown'
    );

    assert.notOk(find('[data-test-idle-button]'), 'No more idle button');
    await componentA11yAudit(this.element, assert);
  });

  test('canceling in the promptForConfirmation state calls the onCancel hook and resets to the idle state', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await TwoStepButton.idle();

    await TwoStepButton.cancel();

    assert.ok(props.onCancel.calledOnce, 'The onCancel hook fired');
    assert.ok(find('[data-test-idle-button]'), 'Idle button is back');
  });

  test('confirming the promptForConfirmation state calls the onConfirm hook and resets to the idle state', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await TwoStepButton.idle();

    await TwoStepButton.confirm();

    assert.ok(props.onConfirm.calledOnce, 'The onConfirm hook fired');
    assert.ok(find('[data-test-idle-button]'), 'Idle button is back');
  });

  test('when awaitingConfirmation is true, the cancel and submit buttons are disabled and the submit button is loading', async function (assert) {
    assert.expect(4);

    const props = commonProperties();
    props.awaitingConfirmation = true;
    this.setProperties(props);
    await render(commonTemplate);

    await TwoStepButton.idle();

    assert.ok(TwoStepButton.cancelIsDisabled, 'The cancel button is disabled');
    assert.ok(
      TwoStepButton.confirmIsDisabled,
      'The confirm button is disabled'
    );
    assert.ok(
      TwoStepButton.isRunning,
      'The confirm button is in a loading state'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('when in the prompt state, clicking outside will reset state back to idle', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await TwoStepButton.idle();

    assert.ok(find('[data-test-cancel-button]'), 'In the prompt state');

    await click(document.body);

    assert.ok(find('[data-test-idle-button]'), 'Back in the idle state');
  });

  test('when in the prompt state, clicking inside will not reset state back to idle', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await TwoStepButton.idle();

    assert.ok(find('[data-test-cancel-button]'), 'In the prompt state');

    await click('[data-test-confirmation-message]');

    assert.notOk(find('[data-test-idle-button]'), 'Still in the prompt state');
  });

  test('when awaitingConfirmation is true, clicking outside does nothing', async function (assert) {
    const props = commonProperties();
    props.awaitingConfirmation = true;
    this.setProperties(props);
    await render(commonTemplate);

    await TwoStepButton.idle();

    assert.ok(find('[data-test-cancel-button]'), 'In the prompt state');

    await click(document.body);

    assert.notOk(find('[data-test-idle-button]'), 'Still in the prompt state');
  });

  test('when disabled is true, the idle button is disabled', async function (assert) {
    assert.expect(2);

    const props = commonProperties();
    props.disabled = true;
    this.setProperties(props);
    await render(commonTemplate);

    assert.ok(TwoStepButton.isDisabled, 'The idle button is disabled');

    await componentA11yAudit(this.element, assert);
  });
});
