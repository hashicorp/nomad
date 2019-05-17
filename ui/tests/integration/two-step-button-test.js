import { find, click } from 'ember-native-dom-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import sinon from 'sinon';

module('Integration | Component | two step button', function(hooks) {
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
    {{two-step-button
      idleText=idleText
      cancelText=cancelText
      confirmText=confirmText
      confirmationMessage=confirmationMessage
      awaitingConfirmation=awaitingConfirmation
      disabled=disabled
      onConfirm=onConfirm
      onCancel=onCancel}}
  `;

  test('presents as a button in the idle state', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    assert.ok(find('[data-test-idle-button]'), 'Idle button is rendered');
    assert.equal(
      find('[data-test-idle-button]').textContent.trim(),
      props.idleText,
      'Button is labeled correctly'
    );

    assert.notOk(find('[data-test-cancel-button]'), 'No cancel button yet');
    assert.notOk(find('[data-test-confirm-button]'), 'No confirm button yet');
    assert.notOk(find('[data-test-confirmation-message]'), 'No confirmation message yet');
  });

  test('clicking the idle state button transitions into the promptForConfirmation state', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    click('[data-test-idle-button]');

    return settled().then(() => {
      assert.ok(find('[data-test-cancel-button]'), 'Cancel button is rendered');
      assert.equal(
        find('[data-test-cancel-button]').textContent.trim(),
        props.cancelText,
        'Button is labeled correctly'
      );

      assert.ok(find('[data-test-confirm-button]'), 'Confirm button is rendered');
      assert.equal(
        find('[data-test-confirm-button]').textContent.trim(),
        props.confirmText,
        'Button is labeled correctly'
      );

      assert.equal(
        find('[data-test-confirmation-message]').textContent.trim(),
        props.confirmationMessage,
        'Confirmation message is shown'
      );

      assert.notOk(find('[data-test-idle-button]'), 'No more idle button');
    });
  });

  test('canceling in the promptForConfirmation state calls the onCancel hook and resets to the idle state', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    click('[data-test-idle-button]');

    return settled().then(() => {
      click('[data-test-cancel-button]');

      return settled().then(() => {
        assert.ok(props.onCancel.calledOnce, 'The onCancel hook fired');
        assert.ok(find('[data-test-idle-button]'), 'Idle button is back');
      });
    });
  });

  test('confirming the promptForConfirmation state calls the onConfirm hook and resets to the idle state', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    click('[data-test-idle-button]');

    return settled().then(() => {
      click('[data-test-confirm-button]');

      return settled().then(() => {
        assert.ok(props.onConfirm.calledOnce, 'The onConfirm hook fired');
        assert.ok(find('[data-test-idle-button]'), 'Idle button is back');
      });
    });
  });

  test('when awaitingConfirmation is true, the cancel and submit buttons are disabled and the submit button is loading', async function(assert) {
    const props = commonProperties();
    props.awaitingConfirmation = true;
    this.setProperties(props);
    await render(commonTemplate);

    click('[data-test-idle-button]');

    return settled().then(() => {
      assert.ok(
        find('[data-test-cancel-button]').hasAttribute('disabled'),
        'The cancel button is disabled'
      );
      assert.ok(
        find('[data-test-confirm-button]').hasAttribute('disabled'),
        'The confirm button is disabled'
      );
      assert.ok(
        find('[data-test-confirm-button]').classList.contains('is-loading'),
        'The confirm button is in a loading state'
      );
    });
  });

  test('when in the prompt state, clicking outside will reset state back to idle', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    click('[data-test-idle-button]');
    await settled();

    assert.ok(find('[data-test-cancel-button]'), 'In the prompt state');

    click(document.body);
    await settled();

    assert.ok(find('[data-test-idle-button]'), 'Back in the idle state');
  });

  test('when in the prompt state, clicking inside will not reset state back to idle', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    click('[data-test-idle-button]');
    await settled();

    assert.ok(find('[data-test-cancel-button]'), 'In the prompt state');

    click('[data-test-confirmation-message]');
    await settled();

    assert.notOk(find('[data-test-idle-button]'), 'Still in the prompt state');
  });

  test('when awaitingConfirmation is true, clicking outside does nothing', async function(assert) {
    const props = commonProperties();
    props.awaitingConfirmation = true;
    this.setProperties(props);
    await render(commonTemplate);

    click('[data-test-idle-button]');
    await settled();

    assert.ok(find('[data-test-cancel-button]'), 'In the prompt state');

    click(document.body);
    await settled();

    assert.notOk(find('[data-test-idle-button]'), 'Still in the prompt state');
  });

  test('when disabled is true, the idle button is disabled', async function(assert) {
    const props = commonProperties();
    props.disabled = true;
    this.setProperties(props);
    await render(commonTemplate);

    assert.ok(
      find('[data-test-idle-button]').hasAttribute('disabled'),
      'The idle button is disabled'
    );

    click('[data-test-idle-button]');
    assert.ok(find('[data-test-idle-button]'), 'Still in the idle state after clicking');
  });
});
