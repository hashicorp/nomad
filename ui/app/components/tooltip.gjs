/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { assert } from '@ember/debug';
import Component from '@glimmer/component';

/**
 * Tooltip component that conditionally displays a truncated text.
 *
 * @class Tooltip
 * @extends Component
 */
export default class Tooltip extends Component {
  get condition() {
    if (this.args.condition === undefined) return true;

    assert('Must pass a boolean.', typeof this.args.condition === 'boolean');

    return this.args.condition;
  }

  get text() {
    const inputText = this.args.text?.toString();
    if (!inputText || inputText.length < 30) {
      return inputText;
    }

    const prefix = inputText.substr(0, 15).trim();
    const suffix = inputText
      .substr(inputText.length - 10, inputText.length)
      .trim();
    return `${prefix}...${suffix}`;
  }

  get tooltipClass() {
    return this.args.isFullText ? 'tooltip multiline' : 'tooltip';
  }

  <template>
    {{#if this.condition}}
      <span
        class={{this.tooltipClass}}
        aria-label={{if @isFullText @text this.text}}
      >
        {{yield}}
      </span>
    {{else}}
      {{yield}}
    {{/if}}
  </template>
}
