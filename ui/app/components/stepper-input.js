/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { action } from '@ember/object';
import { debounce, join } from '@ember/runloop';
import { classNames, classNameBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

const ESC = 27;

@classic
@classNames('stepper-input')
@classNameBindings(
  'class',
  'disabled:is-disabled',
  'disabled:tooltip',
  'disabled:multiline',
)
export default class StepperInput extends Component {
  min = 0;
  max = 10;
  value = 0;
  internalValue = 0;
  debounce = 500;
  onChange() {}

  didReceiveAttrs() {
    super.didReceiveAttrs(...arguments);
    this.set('internalValue', Number(this.value ?? 0));
  }

  @action
  increment() {
    join(this, () => {
      if (this.internalValue < this.max) {
        const nextValue = this.internalValue + 1;
        this.set('internalValue', nextValue);
        this.update(nextValue);
      }
    });
  }

  @action
  decrement() {
    join(this, () => {
      if (this.internalValue > this.min) {
        const nextValue = this.internalValue - 1;
        this.set('internalValue', nextValue);
        this.update(nextValue);
      }
    });
  }

  @action
  setValue(e) {
    join(this, () => {
      if (e.target.value !== '') {
        const newValue = Math.floor(
          Math.min(this.max, Math.max(this.min, e.target.value)),
        );
        this.set('internalValue', newValue);
        this.update(newValue);
      } else {
        e.target.value = this.internalValue;
      }
    });
  }

  @action
  resetTextInput(e) {
    if (e.keyCode === ESC) {
      e.target.value = this.internalValue;
    }
  }

  @action
  selectValue(e) {
    e.target.select();
  }

  update(value) {
    debounce(this, sendUpdateAction, value, this.debounce);
  }
}

function sendUpdateAction(value) {
  return this.onChange(value);
}
