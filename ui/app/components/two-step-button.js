import Component from '@ember/component';
import { action } from '@ember/object';
import { next } from '@ember/runloop';
import { equal } from '@ember/object/computed';
import { task, waitForEvent } from 'ember-concurrency';
import RSVP from 'rsvp';
import { classNames, classNameBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('two-step-button')
@classNameBindings('inlineText:has-inline-text')
export default class TwoStepButton extends Component {
  idleText = '';
  cancelText = '';
  confirmText = '';
  confirmationMessage = '';
  awaitingConfirmation = false;
  disabled = false;
  alignRight = false;
  inlineText = false;
  onConfirm() {}
  onCancel() {}

  state = 'idle';
  @equal('state', 'idle') isIdle;
  @equal('state', 'prompt') isPendingConfirmation;

  @task(function*() {
    while (true) {
      let ev = yield waitForEvent(document.body, 'click');
      if (!this.element.contains(ev.target) && !this.awaitingConfirmation) {
        this.send('setToIdle');
      }
    }
  })
  cancelOnClickOutside;

  @action
  setToIdle() {
    this.set('state', 'idle');
    this.cancelOnClickOutside.cancelAll();
  }

  @action
  promptForConfirmation() {
    this.set('state', 'prompt');
    next(() => {
      this.cancelOnClickOutside.perform();
    });
  }

  @action
  confirm() {
    RSVP.resolve(this.onConfirm()).then(() => {
      this.send('setToIdle');
    });
  }
}
