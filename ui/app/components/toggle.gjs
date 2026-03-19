/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { on } from '@ember/modifier';

const noOp = () => {};

export default class Toggle extends Component {
  get isActive() {
    return this.args.isActive ?? false;
  }

  get isDisabled() {
    return this.args.isDisabled ?? false;
  }

  get onToggle() {
    return this.args.onToggle ?? noOp;
  }

  get rootClass() {
    const classes = ['toggle'];

    if (this.isDisabled) {
      classes.push('is-disabled');
    }

    if (this.isActive) {
      classes.push('is-active');
    }

    return classes.join(' ');
  }

  <template>
    {{! template-lint-disable }}
    <label data-test-label class={{this.rootClass}} ...attributes>
      <input
        data-test-input
        type="checkbox"
        checked={{this.isActive}}
        disabled={{this.isDisabled}}
        class="input"
        {{on "change" this.onToggle}}
      />
      <span data-test-toggler class="toggler"></span>
      {{yield}}
    </label>
  </template>
}
