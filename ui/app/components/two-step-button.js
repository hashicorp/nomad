import Component from '@ember/component';
import { next } from '@ember/runloop';
import { equal } from '@ember/object/computed';
import { task, waitForEvent } from 'ember-concurrency';
import RSVP from 'rsvp';

export default Component.extend({
  classNames: ['two-step-button'],

  idleText: '',
  cancelText: '',
  confirmText: '',
  confirmationMessage: '',
  awaitingConfirmation: false,
  disabled: false,
  alignRight: false,
  isInfoAction: false,
  onConfirm() {},
  onCancel() {},

  state: 'idle',
  isIdle: equal('state', 'idle'),
  isPendingConfirmation: equal('state', 'prompt'),

  cancelOnClickOutside: task(function*() {
    while (true) {
      let ev = yield waitForEvent(document.body, 'click');
      if (!this.element.contains(ev.target) && !this.awaitingConfirmation) {
        this.send('setToIdle');
      }
    }
  }),

  actions: {
    setToIdle() {
      this.set('state', 'idle');
      this.cancelOnClickOutside.cancelAll();
    },
    promptForConfirmation() {
      this.set('state', 'prompt');
      next(() => {
        this.cancelOnClickOutside.perform();
      });
    },
    confirm() {
      RSVP.resolve(this.onConfirm()).then(() => {
        this.send('setToIdle');
      });
    },
  },
});
