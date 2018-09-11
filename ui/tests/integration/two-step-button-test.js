import { find, click } from 'ember-native-dom-helpers';
import { test, moduleForComponent } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import sinon from 'sinon';

moduleForComponent('two-step-button', 'Integration | Component | two step button', {
  integration: true,
});

const commonProperties = () => ({
  idleText: 'Idle State Button',
  cancelText: 'Cancel Action',
  confirmText: 'Confirm Action',
  confirmationMessage: 'Are you certain',
  awaitingConfirmation: false,
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
    onConfirm=onConfirm
    onCancel=onCancel}}
`;

test('presents as a button in the idle state', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

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

test('clicking the idle state button transitions into the promptForConfirmation state', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-idle-button]');

  return wait().then(() => {
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

test('canceling in the promptForConfirmation state calls the onCancel hook and resets to the idle state', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-idle-button]');

  return wait().then(() => {
    click('[data-test-cancel-button]');

    return wait().then(() => {
      assert.ok(props.onCancel.calledOnce, 'The onCancel hook fired');
      assert.ok(find('[data-test-idle-button]'), 'Idle button is back');
    });
  });
});

test('confirming the promptForConfirmation state calls the onConfirm hook and resets to the idle state', function(assert) {
  const props = commonProperties();
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-idle-button]');

  return wait().then(() => {
    click('[data-test-confirm-button]');

    return wait().then(() => {
      assert.ok(props.onConfirm.calledOnce, 'The onConfirm hook fired');
      assert.ok(find('[data-test-idle-button]'), 'Idle button is back');
    });
  });
});

test('when awaitingConfirmation is true, the cancel and submit buttons are disabled and the submit button is loading', function(assert) {
  const props = commonProperties();
  props.awaitingConfirmation = true;
  this.setProperties(props);
  this.render(commonTemplate);

  click('[data-test-idle-button]');

  return wait().then(() => {
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
