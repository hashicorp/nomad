/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Modifier from 'ember-modifier';
import { registerDestructor } from '@ember/destroyable';

export default class KeyboardShortcutModifier extends Modifier {
  @service keyboard;
  @service router;

  modify(
    element,
    _positional,
    {
      label,
      pattern = '',
      action = () => {},
      menuLevel = false,
      enumerated = false,
      exclusive = false,
    }
  ) {
    let commands = [
      {
        label,
        action,
        pattern,
        element,
        menuLevel,
        enumerated,
        exclusive,
      },
    ];

    this.keyboard.addCommands(commands);
    registerDestructor(this, () => {
      this.keyboard.removeCommands(commands);
    });
  }
}
