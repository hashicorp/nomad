/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { action } from '@ember/object';
import { debounce } from '@ember/runloop';
import { oneWay } from '@ember/object/computed';
import { classNames, classNameBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

const ESC = 27;

@classic
@classNames('stepper-input')
@classNameBindings(
  'class',
  'disabled:is-disabled',
  'disabled:tooltip',
  'disabled:multiline'
)
export default class StepperInput extends Component {
  min = 0;
  max = 10;
  value = 0;
  debounce = 500;
  onChange() {}

  // Internal value changes immediately for instant visual feedback.
  // Value is still the public API and is expected to mutate and re-render
  // On onChange which is debounced.
  @oneWay('value') internalValue;

  @action
  increment() {
    if (this.internalValue < this.max) {
      this.incrementProperty('internalValue');
      this.update(this.internalValue);
    }
  }

  @action
  decrement() {
    if (this.internalValue > this.min) {
      this.decrementProperty('internalValue');
      this.update(this.internalValue);
    }
  }

  @action
  setValue(e) {
    if (e.target.value !== '') {
      const newValue = Math.floor(
        Math.min(this.max, Math.max(this.min, e.target.value))
      );
      this.set('internalValue', newValue);
      this.update(this.internalValue);
    } else {
      e.target.value = this.internalValue;
    }
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
