/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

/**
  `{{keyboard-commands}}` helper used to initialize and tear down contextual keynav commands
  @public
  @method keyboard-commands
 */
export default class keyboardCommands extends Helper {
  @service keyboard;

  constructor() {
    super(...arguments);
  }

  compute([commands]) {
    if (commands) {
      this.commands = commands;
      this.keyboard.addCommands(commands);
    }
  }
  willDestroy() {
    super.willDestroy();
    this.keyboard.removeCommands(this.commands);
  }
}
