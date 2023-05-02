/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';

export default class Tooltip extends Component {
  get condition() {
    if (this.args.condition === undefined) return true;
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
}
