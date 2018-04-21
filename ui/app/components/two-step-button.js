import Component from '@ember/component';
import { equal } from '@ember/object/computed';

export default Component.extend({
  classNames: ['two-step-button'],

  idleText: '',
  cancelText: '',
  confirmText: '',
  confirmationMessage: '',
  onConfirm() {},
  onCancel() {},

  state: 'idle',
  isIdle: equal('state', 'idle'),
  isPendingConfirmation: equal('state', 'prompt'),

  actions: {
    setToIdle() {
      this.set('state', 'idle');
    },
    promptForConfirmation() {
      this.set('state', 'prompt');
    },
  },
});
