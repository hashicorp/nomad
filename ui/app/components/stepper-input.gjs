/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { debounce } from '@ember/runloop';
import { guidFor } from '@ember/object/internals';
import { on } from '@ember/modifier';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import didUpdate from '@ember/render-modifiers/modifiers/did-update';

const ESC = 27;

export default class StepperInput extends Component {
  @tracked internalValue = Number(this.args.value ?? 0);

  get inputId() {
    return `stepper-input-${guidFor(this)}`;
  }

  get min() {
    return this.args.min ?? 0;
  }

  get max() {
    return this.args.max ?? 10;
  }

  get disabled() {
    return this.args.disabled ?? false;
  }

  get classVariant() {
    return this.args.class ?? '';
  }

  get rootClass() {
    const classes = ['stepper-input'];

    if (this.classVariant) {
      classes.push(this.classVariant);
    }

    if (this.disabled) {
      classes.push('is-disabled', 'tooltip', 'multiline');
    }

    return classes.join(' ');
  }

  get isIncrementDisabled() {
    return this.disabled || this.internalValue >= this.max;
  }

  get isDecrementDisabled() {
    return this.disabled || this.internalValue <= this.min;
  }

  syncValueFromArgs = () => {
    this.internalValue = Number(this.args.value ?? 0);
    this.syncInputElementValue(this.internalValue);
  };

  syncInputElementValue(value) {
    const input = document.getElementById(this.inputId);
    if (input) {
      input.value = value;
    }
  }

  increment = () => {
    if (this.internalValue < this.max) {
      const nextValue = this.internalValue + 1;
      this.internalValue = nextValue;
      this.syncInputElementValue(nextValue);
      this.update(nextValue);
    }
  };

  decrement = () => {
    if (this.internalValue > this.min) {
      const nextValue = this.internalValue - 1;
      this.internalValue = nextValue;
      this.syncInputElementValue(nextValue);
      this.update(nextValue);
    }
  };

  setValue = (event) => {
    if (event.target.value !== '') {
      const rawValue = Number(event.target.value);
      const boundedValue = Math.min(this.max, Math.max(this.min, rawValue));
      const newValue = Math.floor(boundedValue);

      if (Number.isFinite(newValue)) {
        this.internalValue = newValue;
        this.syncInputElementValue(newValue);
        this.update(newValue);
      } else {
        event.target.value = this.internalValue;
      }
    } else {
      event.target.value = this.internalValue;
    }
  };

  resetTextInput = (event) => {
    if (event.keyCode === ESC) {
      event.target.value = this.internalValue;
    }
  };

  selectValue = (event) => {
    event.target.select();
  };

  update(value) {
    debounce(this, this.sendUpdateAction, value, this.args.debounce ?? 500);
  }

  sendUpdateAction = (value) => {
    if (this.args.onChange) {
      return this.args.onChange(value);
    }
  };

  <template>
    <div
      class={{this.rootClass}}
      {{didInsert this.syncValueFromArgs}}
      {{didUpdate this.syncValueFromArgs @value}}
    >
      <label
        data-test-stepper-label
        for={{this.inputId}}
        class="stepper-input-label"
      >{{yield}}</label>
      <input
        data-test-stepper-input
        type="number"
        min={{this.min}}
        max={{this.max}}
        value={{this.internalValue}}
        disabled={{this.disabled}}
        id={{this.inputId}}
        class="stepper-input-input"
        {{on "focus" this.selectValue}}
        {{on "keyup" this.resetTextInput}}
        {{on "change" this.setValue}}
      />
      <button
        data-test-stepper-decrement
        aria-label="decrement"
        class="stepper-input-stepper button {{this.classVariant}}"
        disabled={{this.isDecrementDisabled}}
        {{on "click" this.decrement}}
        type="button"
      >
        <HdsIcon @name="minus" />
      </button>
      <button
        data-test-stepper-increment
        aria-label="increment"
        class="stepper-input-stepper button {{this.classVariant}}"
        disabled={{this.isIncrementDisabled}}
        {{on "click" this.increment}}
        type="button"
      >
        <HdsIcon @name="plus" />
      </button>
    </div>
  </template>
}
