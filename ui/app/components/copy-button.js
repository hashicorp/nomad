/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { task, timeout } from 'ember-concurrency';

export default class CopyButton extends Component {
  @tracked state = null;

  get text() {
    if (typeof this.args.clipboardText === 'function')
      return this.args.clipboardText;
    if (typeof this.args.clipboardText === 'string')
      return this.args.clipboardText;

    return String(this.args.clipboardText);
  }

  @(task(function* () {
    this.state = 'success';

    yield timeout(2000);
    this.state = null;
  }).restartable())
  indicateSuccess;
}
