/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import { assert } from '@ember/debug';
import Component from '@glimmer/component';

/**
 * Tooltip component that conditionally displays a truncated text.
 *
 * @class Tooltip
 * @extends Component
 */
export default class Tooltip extends Component {
  /**
   * Determines if the tooltip should be displayed.
   * Defaults to `true` if the `condition` argument is not provided.
   *
   * @property condition
   * @type {boolean}
   * @readonly
   */
  get condition() {
    if (this.args.condition === undefined) return true;

    assert('Must pass a boolean.', typeof this.args.condition === 'boolean');

    return this.args.condition;
  }

  /**
   * Returns the truncated text to be displayed in the tooltip.
   * If the input text length is less than 30 characters, the input text is returned as-is.
   * Otherwise, the text is truncated to include the first 15 characters, followed by an ellipsis,
   * and then the last 10 characters.
   *
   * @property text
   * @type {string}
   * @readonly
   */
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
}
